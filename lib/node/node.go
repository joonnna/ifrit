package node

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/joonnna/ifrit/lib/protobuf"
	"github.com/joonnna/ifrit/lib/udp"
	"github.com/joonnna/ifrit/logger"
)

var (
	errNoRingNum = errors.New("No ringnumber present in received certificate")
	errNoId      = errors.New("No id present in received certificate")
	errNoData    = errors.New("Gossip data has zero length")
)

const (
	NormalProtocol          = 1
	SpamAccusationsProtocol = 2
	DosProtocol             = 3
)

type Node struct {
	log *logger.Log

	//Local peer representation
	*peer

	*view

	*pinger

	protocol
	protocolMutex sync.RWMutex

	client client
	server server

	wg       *sync.WaitGroup
	exitChan chan bool

	exitFlag  bool
	exitMutex sync.RWMutex

	gossipTimeout      time.Duration
	gossipTimeoutMutex sync.RWMutex

	monitorTimeout  time.Duration
	nodeDeadTimeout float64

	privKey   *ecdsa.PrivateKey
	localCert *x509.Certificate
	caCert    *x509.Certificate

	trustedBootNode bool
	httpAddr        string

	stats *recorder
}

type client interface {
	Init(config *tls.Config)
	Gossip(addr string, args *gossip.GossipMsg) (*gossip.GossipResponse, error)
	Dos(addr string, args *gossip.GossipMsg) (*gossip.GossipResponse, error)
}

type server interface {
	Init(config *tls.Config, n interface{}, maxConcurrent uint32) error
	HostInfo() string
	Start() error
	ShutDown()
}

type protocol interface {
	Monitor(n *Node)
	Gossip(n *Node)
	Rebuttal(n *Node)
	Timeouts(n *Node)
}

type timeout struct {
	observer  *peer
	lastNote  *note
	timeStamp time.Time

	//For debugging
	addr string
}

func (n *Node) gossipLoop() {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Exiting gossiping")
			n.wg.Done()
			return
		case <-time.After(n.getGossipTimeout()):
			n.getProtocol().Gossip(n)
		}
	}
}

func (n *Node) monitor() {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Stopping monitoring")
			n.wg.Done()
			return
		case <-time.After(n.monitorTimeout):
			n.getProtocol().Monitor(n)
		}
	}
}

func (n *Node) checkTimeouts() {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Stopping view update")
			n.wg.Done()
			return
		case <-time.After(n.viewUpdateTimeout):
			n.getProtocol().Timeouts(n)
		}
	}
}

func (n *Node) collectGossipContent() (*gossip.GossipMsg, error) {
	msg := &gossip.GossipMsg{
		ExistingHosts: make(map[string]uint64),
	}

	view := n.getView()

	for _, p := range view {
		peerEpoch := uint64(0)

		peerNote := p.getNote()
		if peerNote != nil {
			peerEpoch = peerNote.epoch
		}

		msg.ExistingHosts[p.key] = peerEpoch
	}

	msg.OwnNote = n.localNoteToPbMsg()
	msg.Rebuttal = false

	return msg, nil
}

func (n *Node) setProtocol(p protocol) {
	n.protocolMutex.Lock()
	defer n.protocolMutex.Unlock()

	n.protocol = p
}

func (n *Node) getProtocol() protocol {
	n.protocolMutex.RLock()
	defer n.protocolMutex.RUnlock()

	return n.protocol
}

func (n *Node) localNoteToPbMsg() *gossip.Note {
	n.noteMutex.RLock()
	defer n.noteMutex.RUnlock()

	return n.recentNote.toPbMsg()
}

func (n *Node) setGossipTimeout(timeout int) {
	n.gossipTimeoutMutex.Lock()
	defer n.gossipTimeoutMutex.Unlock()

	n.gossipTimeout = (time.Duration(timeout) * time.Second)
}

func (n *Node) getGossipTimeout() time.Duration {
	n.gossipTimeoutMutex.RLock()
	defer n.gossipTimeoutMutex.RUnlock()

	return n.gossipTimeout
}

func (n *Node) LiveMembers() []string {
	return n.getLivePeerAddrs()
}

func NewNode(caAddr string, c client, s server) (*Node, error) {
	var i uint32
	var extValue []byte
	logger := logger.CreateLogger(s.HostInfo(), "nodelog")

	udpServer, err := udp.NewServer(logger)
	if err != nil {
		logger.Err.Println(err)
		return nil, err
	}

	privKey, err := genKeys()
	if err != nil {
		logger.Err.Println(err)
		return nil, err
	}

	addr := fmt.Sprintf("http://%s/certificateRequest", caAddr)

	certs, err := sendCertRequest(addr, privKey, s.HostInfo(), udpServer.Addr())
	if err != nil {
		logger.Err.Println(err)
		return nil, err
	}

	extensions := certs.ownCert.Extensions

	if len(certs.ownCert.SubjectKeyId) < 1 {
		logger.Err.Println(errNoId)
		return nil, errNoId
	}

	for _, e := range extensions {
		if e.Id.Equal(asn1.ObjectIdentifier{2, 5, 13, 37}) {
			extValue = e.Value
		}
	}
	if extValue == nil {
		logger.Err.Println(errNoRingNum)
		return nil, errNoRingNum
	}

	numRings := binary.LittleEndian.Uint32(extValue[0:])

	config := genServerConfig(certs, privKey)

	p, err := newPeer(nil, certs.ownCert, numRings)
	if err != nil {
		logger.Err.Println(err)
		return nil, err
	}

	v, err := newView(numRings, logger, p.peerId, p.addr)
	if err != nil {
		logger.Err.Println(err)
		return nil, err
	}

	n := &Node{
		exitChan:        make(chan bool, 1),
		wg:              &sync.WaitGroup{},
		gossipTimeout:   time.Second * 5,
		monitorTimeout:  time.Second * 5,
		nodeDeadTimeout: 20,
		view:            v,
		pinger:          newPinger(udpServer, privKey, logger),
		privKey:         privKey,
		client:          c,
		server:          s,
		peer:            p,
		stats:           &recorder{recordDuration: 60, log: logger},
		log:             logger,
		localCert:       certs.ownCert,
		caCert:          certs.caCert,
		trustedBootNode: certs.trusted,
	}

	err = n.server.Init(config, n, ((n.numRings * 2) + 20))
	if err != nil {
		n.log.Err.Println(err)
		return nil, err
	}

	n.client.Init(genClientConfig(certs, privKey))

	localNote := &note{
		epoch:  1,
		mask:   make([]byte, numRings),
		peerId: n.peerId,
	}
	for i = 0; i < n.numRings; i++ {
		localNote.mask[i] = 1
	}

	err = localNote.sign(n.privKey)
	if err != nil {
		n.log.Err.Println(err)
		return nil, err
	}

	n.recentNote = localNote

	for _, c := range certs.knownCerts {
		if n.peerId.equal(&peerId{id: c.SubjectKeyId}) {
			continue
		}
		n.evalCertificate(c)
	}

	view := n.getView()

	for _, p := range view {
		n.addLivePeer(p)
	}

	return n, nil
}

func (n *Node) ShutDownNode() {
	for _, r := range n.ringMap {
		n.remove(r.ringNum)
	}
	close(n.exitChan)
	n.wg.Wait()
}

func (n *Node) Start() {
	n.log.Info.Println("Started Node")

	done := make(chan bool)

	n.setProtocol(correct{})

	go n.server.Start()
	go n.pinger.serve()

	n.wg.Add(4)
	go n.gossipLoop()
	go n.monitor()
	go n.checkTimeouts()
	go n.updateState()

	go n.httpHandler(done)

	<-done

	for _, r := range n.ringMap {
		for {
			err := n.add(r.ringNum)
			if err == nil {
				break
			}
		}
	}

	<-n.exitChan
	n.server.ShutDown()
	n.pinger.shutdown()
	n.log.Info.Println("Exiting node")
}
