package node

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"sync"
	"time"

	"github.com/joonnna/firechain/lib/protobuf"
	"github.com/joonnna/firechain/logger"
)

var (
	errNoRingNum = errors.New("No ringnumber present in received certificate")
	errNoId      = errors.New("No id present in received certificate")
)

const (
	NormalProtocol          = 1
	SpamAccusationsProtocol = 2
)

type Node struct {
	log *logger.Log

	//Local peer representation
	*peer

	*view

	protocol
	protocolMutex sync.RWMutex

	wg       *sync.WaitGroup
	exitChan chan bool

	exitFlag  bool
	exitMutex sync.RWMutex

	client Client
	server Server

	gossipTimeout   time.Duration
	monitorTimeout  time.Duration
	nodeDeadTimeout float64

	privKey   *ecdsa.PrivateKey
	localCert *x509.Certificate

	trustedBootNode bool
	httpAddr        string
}

type Pinger interface {
	Ping(addr string, args *gossip.Ping) (*gossip.Pong, error)
}

type Client interface {
	Init(config *tls.Config)
	Gossip(addr string, args *gossip.GossipMsg) (*gossip.Partners, error)
	Monitor(addr string, args *gossip.Ping) (*gossip.Pong, error)
}

type Server interface {
	Init(config *tls.Config, n interface{}) error
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
		case <-time.After(n.gossipTimeout):
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
	msg := &gossip.GossipMsg{}

	view := n.getView()

	for _, p := range view {
		c, n, a := p.createPbInfo()

		msg.Certificates = append(msg.Certificates, c)
		if n != nil {
			msg.Notes = append(msg.Notes, n)
		}

		if a != nil {
			msg.Accusations = append(msg.Accusations, a)
		}
	}

	noteMsg := n.localNoteToPbMsg()

	msg.Notes = append(msg.Notes, noteMsg)

	return msg, nil
}

func (n *Node) setProtocol(protocol int) {
	n.protocolMutex.Lock()
	defer n.protocolMutex.Unlock()

	switch protocol {
	case NormalProtocol:
		n.protocol = correct{}
	case SpamAccusationsProtocol:
		n.protocol = spamAccusations{}
	default:
		n.protocol = correct{}
	}
}

func (n *Node) getProtocol() protocol {
	n.protocolMutex.RLock()
	defer n.protocolMutex.RUnlock()

	return n.protocol
}

func (n *Node) isPrev(p, other *peer, mask []byte) bool {
	if len(mask) != len(n.ringMap) {
		n.log.Err.Printf("Mask is of invalid length, %d != %d ", len(mask), len(n.ringMap))
		return false
	}

	for idx, r := range n.ringMap {
		if mask[idx] == 0 {
			n.log.Err.Println("Mask have deactivated ring", idx)
			continue
		}
		prev, err := r.isPrev(p, other)
		if err != nil {
			n.log.Err.Println(err)
			continue
		}

		if prev {
			return true
		}
	}
	return false
}

func (n *Node) shouldBeNeighbours(id []byte) bool {
	pId := newPeerId(id)

	for _, r := range n.ringMap {
		if r.betweenNeighbours(pId) {
			return true
		}
	}
	return false
}

func (n *Node) localNoteToPbMsg() *gossip.Note {
	n.noteMutex.RLock()
	defer n.noteMutex.RUnlock()

	return n.recentNote.toPbMsg()
}

func (n *Node) findNeighbours(id []byte) []string {
	keys := make([]string, 0)

	pId := newPeerId(id)

	for _, r := range n.ringMap {
		key, err := r.findNeighbour(pId)
		if err != nil {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func NewNode(caAddr string, c Client, s Server) (*Node, error) {
	var i uint8

	logger := logger.CreateLogger(s.HostInfo(), "nodelog")

	privKey, err := genKeys()
	if err != nil {
		return nil, err
	}

	certs, err := sendCertRequest(caAddr, privKey, s.HostInfo())
	if err != nil {
		return nil, err
	}

	ext := certs.ownCert.Extensions

	if len(ext) < 1 || len(ext[0].Value) < 1 {
		return nil, errNoRingNum
	}

	if len(certs.ownCert.SubjectKeyId) < 1 {
		return nil, errNoId
	}

	numRings := ext[0].Value[0]

	config := genServerConfig(certs, privKey)

	p, err := newPeer(nil, certs.ownCert)
	if err != nil {
		return nil, err
	}

	n := &Node{
		exitChan:        make(chan bool, 1),
		wg:              &sync.WaitGroup{},
		gossipTimeout:   time.Second * 3,
		monitorTimeout:  time.Second * 3,
		nodeDeadTimeout: 5.0,
		view:            newView(numRings, logger, p.peerId, p.addr),
		privKey:         privKey,
		client:          c,
		server:          s,
		peer:            p,
		log:             logger,
		localCert:       certs.ownCert,
		trustedBootNode: certs.trusted,
	}

	err = n.server.Init(config, n)
	if err != nil {
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
		return nil, err
	}

	n.recentNote = localNote

	for _, c := range certs.knownCerts {
		p, err := newPeer(nil, c)
		if err != nil {
			n.log.Err.Println(err)
			continue
		}
		n.addViewPeer(p)
		n.addLivePeer(p)
	}

	return n, nil
}

func (n *Node) ShutDownNode() {
	for i, _ := range n.ringMap {
		n.remove(i)
	}
	close(n.exitChan)
	n.wg.Wait()
}

func (n *Node) Start(protocol int) {
	var i uint8
	n.log.Info.Println("Started Node")

	done := make(chan bool)

	n.setProtocol(protocol)

	go n.server.Start()

	n.wg.Add(4)
	go n.gossipLoop()
	go n.monitor()
	go n.checkTimeouts()
	go n.updateState()

	go n.httpHandler(done)

	<-done

	for i = 0; i < n.numRings; i++ {
		n.add(i)
	}

	<-n.exitChan
	n.server.ShutDown()
	n.log.Info.Println("Exiting node")
}
