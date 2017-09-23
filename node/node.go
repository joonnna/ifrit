package node

import (
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

	Communication

	gossipTimeout     time.Duration
	monitorTimeout    time.Duration
	viewUpdateTimeout time.Duration
	nodeDeadTimeout   float64

	epoch      uint64
	epochMutex sync.RWMutex

	httpAddr string
}

type Communication interface {
	HostInfo() string
	ShutDown()
	Register(g gossip.GossipServer)
	Start() error
	Gossip(addr string, args *gossip.GossipMsg) (*gossip.Empty, error)
	Monitor(addr string, args *gossip.Ping) (*gossip.Pong, error)
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

func (n *Node) collectGossipContent() (map[string]*gossip.Note, map[string]*gossip.Accusation) {
	noteMap := make(map[string]*gossip.Note)
	accuseMap := make(map[string]*gossip.Accusation)

	view := n.getView()

	for _, p := range view {
		peerNote := p.getNote()
		peerAccuse := p.getAccusation()

		noteEntry := &gossip.Note{
			Epoch: peerNote.epoch,
			Addr:  p.addr,
			Mask:  peerNote.mask,
		}

		if peerAccuse != nil {
			accuseEntry := &gossip.Accusation{
				Accuser:    peerAccuse.accuser,
				RecentNote: noteEntry,
			}
			accuseMap[p.addr] = accuseEntry
		}

		noteMap[p.addr] = noteEntry
	}

	noteMap[n.localAddr] = &gossip.Note{
		Epoch: n.epoch,
		Addr:  n.localAddr,
	}

	return noteMap, accuseMap
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

func NewNode(comm Communication, log *logger.Log) *Node {
	priv, pub, err := genKeys()
	if err != nil {
		n.log.Err.Panic(err)
	}

	n := &Node{
		Communication:     comm,
		log:               log,
		exitChan:          make(chan bool, 1),
		wg:                &sync.WaitGroup{},
		localAddr:         comm.HostInfo(),
		ringMap:           make(map[uint8]*ring),
		viewMap:           make(map[string]*peer),
		liveMap:           make(map[string]*peer),
		timeoutMap:        make(map[string]*timeout),
		viewUpdateTimeout: time.Second * 2,
		gossipTimeout:     time.Second * 3,
		monitorTimeout:    time.Second * 3,
		nodeDeadTimeout:   5.0,
	}

	n.Communication.Register(n)

	return n
}

func (n *Node) ShutDownNode() {
	for i, _ := range n.ringMap {
		n.remove(i)
	}
	close(n.exitChan)
	n.wg.Wait()
}

func (n *Node) Start(protocol int, caAddr string) {
	n.log.Info.Println("Started Node")
	var i uint8

	done := make(chan bool)

	for i = 0; i < numRings; i++ {
		n.ringMap[i] = newRing(i, n.localAddr)
	}

	if entryAddr != "" {
		newNote := createNote(entryAddr, 0, "")
		p := newPeer(newNote)
		n.addViewPeer(p)
		n.addLivePeer(p)
	}

	n.setProtocol(protocol)

	go n.Communication.Start()

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
	n.Communication.ShutDown()
	n.log.Info.Println("Exiting node")
}
