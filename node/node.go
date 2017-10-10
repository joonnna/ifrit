package node

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/joonnna/capstone/logger"
	"github.com/joonnna/capstone/protobuf"
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

	localAddr string
	*peerId

	protocol
	protocolMutex sync.RWMutex

	viewMap   map[string]*peer
	viewMutex sync.RWMutex

	liveMap   map[string]*peer
	liveMutex sync.RWMutex

	timeoutMap   map[string]*timeout
	timeoutMutex sync.RWMutex

	//Read only, don't need mutex
	ringMap  map[uint8]*ring
	numRings uint8

	wg       *sync.WaitGroup
	exitChan chan bool

	exitFlag  bool
	exitMutex sync.RWMutex

	client Client
	server Server

	gossipTimeout     time.Duration
	monitorTimeout    time.Duration
	viewUpdateTimeout time.Duration
	nodeDeadTimeout   float64

	epoch      uint64
	epochMutex sync.RWMutex

	privKey   *ecdsa.PrivateKey
	localCert *x509.Certificate

	trustedBootNode bool
	httpAddr        string
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
			timeouts := n.getAllTimeouts()
			for key, t := range timeouts {
				n.log.Debug.Println("Have timeout for: ", t.addr)
				since := time.Since(t.timeStamp)
				if since.Seconds() > n.nodeDeadTimeout {
					n.log.Debug.Printf("%s timeout expired, removing from live", t.addr)
					n.deleteTimeout(key)
					n.removeLivePeer(key)
				}
			}
		}
	}
}

func (n *Node) collectGossipContent() (*gossip.GossipMsg, error) {
	msg := &gossip.GossipMsg{}

	view := n.getLivePeers()

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

	noteMsg := &gossip.Note{
		Id:    n.id,
		Epoch: n.getEpoch(),
	}

	b := []byte(fmt.Sprintf("%v", noteMsg))
	signature, err := signContent(b, n.privKey)
	if err != nil {
		n.log.Err.Println(err)
		return nil, err
	}
	noteMsg.Signature = &gossip.Signature{
		R: signature.r,
		S: signature.s,
	}

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

func (n *Node) isPrev(p *peer, other *peer) bool {
	for _, r := range n.ringMap {
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

func (n *Node) isHigherRank(p, other *peerId) bool {
	for _, r := range n.ringMap {
		if r.isHigher(p, other) {
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

	config := genServerConfig(certs, privKey)

	n := &Node{
		exitChan:          make(chan bool, 1),
		wg:                &sync.WaitGroup{},
		ringMap:           make(map[uint8]*ring),
		viewMap:           make(map[string]*peer),
		liveMap:           make(map[string]*peer),
		timeoutMap:        make(map[string]*timeout),
		viewUpdateTimeout: time.Second * 2,
		gossipTimeout:     time.Second * 3,
		monitorTimeout:    time.Second * 3,
		nodeDeadTimeout:   5.0,
		privKey:           privKey,
		peerId:            newPeerId(certs.ownCert.SubjectKeyId),
		client:            c,
		server:            s,
		numRings:          ext[0].Value[0],
		localAddr:         s.HostInfo(),
		log:               logger,
		localCert:         certs.ownCert,
		trustedBootNode:   certs.trusted,
	}

	err = n.server.Init(config, n)
	if err != nil {
		return nil, err
	}

	n.client.Init(genClientConfig(certs, privKey))

	for i = 0; i < n.numRings; i++ {
		n.ringMap[i] = newRing(i, n.id, n.key, n.localAddr)
	}

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
