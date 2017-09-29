package node

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"sync"
	"time"

	"github.com/joonnna/capstone/logger"
	"github.com/joonnna/capstone/protobuf"
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

	NodeComm

	gossipTimeout     time.Duration
	monitorTimeout    time.Duration
	viewUpdateTimeout time.Duration
	nodeDeadTimeout   float64

	epoch      uint64
	epochMutex sync.RWMutex

	httpAddr string
}

//TODO Getting too large and specific,
//either cut down or simply include the rpc package and drop the interface
//Or make rpc apart of the node package(probably not)
type NodeComm interface {
	HostInfo() string
	ShutDown()
	Register(g gossip.GossipServer)
	Start() error
	Gossip(addr string, args *gossip.GossipMsg) (*gossip.Empty, error)
	Monitor(addr string, args *gossip.Ping) (*gossip.Pong, error)

	//Might change this...
	EntryCertificates() []*x509.Certificate
	OwnCertificate() *x509.Certificate
	ID() []byte
	NumRings() uint8
}

type protocol interface {
	Monitor(n *Node)
	Gossip(n *Node)
}

type timeout struct {
	observer  *peer
	lastNote  *note
	timeStamp time.Time
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
				n.log.Debug.Println("Have timeout for: ", key)
				since := time.Since(t.timeStamp)
				if since.Seconds() > n.nodeDeadTimeout {
					n.log.Debug.Printf("%s timeout expired, removing from live", key)
					n.deleteTimeout(key)
					n.removeLivePeer(key)
				}
			}
		}
	}
}

func (n *Node) collectGossipContent() *gossip.GossipMsg {
	msg := &gossip.GossipMsg{}

	view := n.getView()

	/*
		viewLen := len(view)

		msg.Certificates = make([]*gossip.Certificate, viewLen)
		msg.Notes = make([]*gossip.Note, viewLen)
		msg.Accusations = make([]*gossip.Accusation, 0)
	*/
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

	ownCert := n.NodeComm.OwnCertificate()
	certMsg := &gossip.Certificate{
		Raw: ownCert.Raw,
	}

	noteMsg := &gossip.Note{
		Id:        n.NodeComm.ID(),
		Epoch:     n.getEpoch(),
		Signature: ownCert.Signature,
	}

	msg.Certificates = append(msg.Certificates, certMsg)
	msg.Notes = append(msg.Notes, noteMsg)

	return msg
}

func (n *Node) setProtocol(protocol int) {
	n.protocolMutex.Lock()
	defer n.protocolMutex.Unlock()

	switch protocol {
	case NormalProtocol:
		n.protocol = correct{}
	/*
		case SpamAccusationsProtocol:
			n.protocol = spamAccusations{}
	*/
	default:
		n.protocol = correct{}
	}
}

func (n *Node) getProtocol() protocol {
	n.protocolMutex.RLock()
	defer n.protocolMutex.RUnlock()

	return n.protocol
}

func NewNode(comm NodeComm, log *logger.Log) (*Node, error) {
	var i uint8

	n := &Node{
		NodeComm:          comm,
		log:               log,
		exitChan:          make(chan bool, 1),
		wg:                &sync.WaitGroup{},
		localAddr:         comm.HostInfo(),
		peerId:            newPeerId(comm.ID()),
		numRings:          comm.NumRings(),
		ringMap:           make(map[uint8]*ring),
		viewMap:           make(map[string]*peer),
		liveMap:           make(map[string]*peer),
		timeoutMap:        make(map[string]*timeout),
		viewUpdateTimeout: time.Second * 2,
		gossipTimeout:     time.Second * 3,
		monitorTimeout:    time.Second * 3,
		nodeDeadTimeout:   5.0,
	}

	n.NodeComm.Register(n)

	for i = 0; i < n.numRings; i++ {
		n.ringMap[i] = newRing(i, n.id, n.key, n.localAddr)
	}

	certs := n.NodeComm.EntryCertificates()
	for _, c := range certs {
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

	go n.NodeComm.Start()

	n.wg.Add(3)
	go n.gossipLoop()
	go n.monitor()
	go n.checkTimeouts()

	go n.httpHandler(done)

	<-done

	for i = 0; i < n.numRings; i++ {
		n.add(i)
	}

	<-n.exitChan
	n.NodeComm.ShutDown()
	n.log.Info.Println("Exiting node")
}

func genKeys() (*rsa.PrivateKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	return privKey, nil
}
