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
	localId   []byte

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

type NodeComm interface {
	HostInfo() string
	ShutDown()
	Register(g gossip.GossipServer)
	Start() error
	Gossip(addr string, args *gossip.GossipMsg) (*gossip.Empty, error)
	Monitor(addr string, args *gossip.Ping) (*gossip.Pong, error)

	//Might change this...
	EntryCertificates() []*x509.Certificate
	ID() []byte
	NumRings() uint8
}

type protocol interface {
	Monitor(n *Node)
	Gossip(n *Node)
}

type timeout struct {
	observer  string
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
			for addr, t := range timeouts {
				n.log.Debug.Println("Have timeout for:", addr)
				since := time.Since(t.timeStamp)
				if since.Seconds() > n.nodeDeadTimeout {
					n.log.Debug.Printf("%s timeout expired, removing from live", addr)
					n.deleteTimeout(addr)
					n.removeLivePeer(addr)
				}
			}
		}
	}
}

func (n *Node) collectGossipContent() (map[string]*gossip.Note, map[string]*gossip.Accusation, map[string]*gossip.Certificate) {
	var noteEntry *gossip.Note

	noteMap := make(map[string]*gossip.Note)
	accuseMap := make(map[string]*gossip.Accusation)
	certMap := make(map[string]*gossip.Certificate)

	view := n.getView()

	for _, p := range view {
		peerNote := p.getNote()
		peerAccuse := p.getAccusation()

		if peerNote != nil {
			noteEntry = &gossip.Note{
				Epoch: peerNote.epoch,
				Addr:  p.addr,
				Mask:  peerNote.mask,
			}
			noteMap[p.addr] = noteEntry
		}

		if peerAccuse != nil {
			accuseEntry := &gossip.Accusation{
				Accuser:    peerAccuse.accuser,
				RecentNote: noteEntry,
			}
			accuseMap[p.addr] = accuseEntry
		}

		certMap[p.addr] = &gossip.Certificate{Raw: p.cert.Raw}
	}

	noteMap[n.localAddr] = &gossip.Note{
		Epoch: n.epoch,
		Addr:  n.localAddr,
	}

	return noteMap, accuseMap, certMap
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

func NewNode(comm NodeComm, log *logger.Log) (*Node, error) {
	var i uint8

	n := &Node{
		NodeComm:          comm,
		log:               log,
		exitChan:          make(chan bool, 1),
		wg:                &sync.WaitGroup{},
		localAddr:         comm.HostInfo(),
		localId:           comm.ID(),
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
		n.ringMap[i] = newRing(i, n.localId, n.localAddr)
	}

	certs := n.NodeComm.EntryCertificates()
	for _, c := range certs {
		p := newPeer(nil, c)
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
