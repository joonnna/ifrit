package node

import (
	"sync"
	"time"

	"github.com/joonnna/capstone/logger"
	"github.com/joonnna/capstone/protobuf"
)

type Node struct {
	log *logger.Log

	localAddr string

	viewMap map[string]*peer
	viewMutex sync.RWMutex

	liveMap map[string]*peer
	liveMutex sync.RWMutex

	timeoutMap map[string]*timeout
	timeoutMutex sync.RWMutex

	//Read only, don't need mutex
	ringMap map[uint8]*ring
	numRings uint8

	wg *sync.WaitGroup
	exitChan chan bool

	Communication

	gossipTimeout time.Duration
	monitorTimeout time.Duration
	viewUpdateTimeout time.Duration
	nodeDeadTimeout float64

	epoch uint64

	httpAddr string
}

type Communication interface {
	ShutDown()
	Register(g gossip.GossipServer)
	Start() error
	Gossip(addr string, args *gossip.GossipMsg) (*gossip.Empty, error)
	Monitor(addr string, args *gossip.Ping) (*gossip.Pong, error)
}

type timeout struct {
	observer string
	lastNote *note
	timeStamp time.Time
}

func (n *Node) gossipLoop () {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Exiting gossiping")
			n.wg.Done()
			return
		case <-time.After(n.gossipTimeout):
			notes, accusations := n.collectGossipContent()
			args := &gossip.GossipMsg{Notes: notes, Accusations: accusations}

			neighbours := n.getNeighbours()

			for _, addr := range neighbours {
				_, err := n.Communication.Gossip(addr, args)
				if err != nil {
					n.log.Err.Println(err)
					continue
				}
			}
		}
	}
}

func (n *Node) monitor () {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Stopping monitoring")
			n.wg.Done()
			return
		case <-time.After(n.monitorTimeout):
			msg := &gossip.Ping{}
			
			for _, ring := range n.ringMap {
				succ, err :=  ring.getRingSucc()
				if err != nil {
					n.log.Err.Println(err)
					continue
				}

				//TODO maybe udp? only add accusation
				_, err = n.Communication.Monitor(succ.nodeId, msg)
				if err != nil {
					n.log.Info.Printf("%s is dead, accusing", succ.nodeId)
					p := n.getViewPeer(succ.nodeId)
					if p != nil {
						peerNote := p.getNote()
						p.setAccusation(n.localAddr, peerNote)
						if !n.timerExist(p.addr) {
							n.startTimer(p.addr, peerNote, n.localAddr)
						}
					}
				}
			}	
		}
	}
}

func (n *Node) checkTimeouts () {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Stopping view update")
			n.wg.Done()
			return
		//TODO only check timeout set?
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

func (n *Node) collectGossipContent() (map[string] *gossip.Note, map[string] *gossip.Accusation) {
	noteMap := make(map[string]*gossip.Note)
	accuseMap := make(map[string]*gossip.Accusation)

	view := n.getView() 

	for _, p := range view {
		peerNote := p.getNote()
		peerAccuse := p.getAccusation()

		noteEntry := &gossip.Note {
			Epoch: peerNote.epoch,
			Addr: p.addr,
			Mask: peerNote.mask,
		}

		if peerAccuse != nil {
			//n.log.Debug.Println("Have accusation for: ", p.addr) 
			accuseEntry := &gossip.Accusation {
				Accuser: peerAccuse.accuser,
				RecentNote: noteEntry,
			}
			accuseMap[p.addr] = accuseEntry
		}

		noteMap[p.addr] = noteEntry
	}
	/*
	for addr, _ := range accuseMap {
		//n.log.Debug.Println("Gossiping accusation for:", addr)
	}
	*/

	noteMap[n.localAddr] = &gossip.Note {
		Epoch: n.epoch,
		Addr: n.localAddr,
	}

	return noteMap, accuseMap
}



func (n *Node) checkAccusation(addr string) *accusation {
	p := n.getViewPeer(addr)
	if p == nil {
		return nil
	}

	return p.getAccusation()
}


func NewNode (entryAddr string, comm Communication, log *logger.Log, addr string, numRings uint8) *Node {
	var i uint8

	n := &Node{
		Communication: comm,
		log: log,
		exitChan: make(chan bool, 1),
		wg: &sync.WaitGroup{},
		localAddr: addr,
		ringMap: make(map[uint8]*ring),
		numRings: numRings,
		viewMap: make(map[string]*peer),
		liveMap: make(map[string]*peer),
		timeoutMap: make(map[string]*timeout),
		viewUpdateTimeout: time.Second * 2,
		gossipTimeout: time.Second * 3,
		monitorTimeout: time.Second * 3,
		nodeDeadTimeout: 5.0,
	}

	n.Communication.Register(n)

	for i = 0; i < numRings; i++ {
		n.ringMap[i] = newRing(i, n.localAddr)
	}

	if entryAddr != "" {
		newNote := createNote(0, "")
		p := newPeer(entryAddr, newNote)
		n.addViewPeer(p)
		n.addLivePeer(p)
	}

	return n
}

func (n *Node) ShutDownNode () {
	for i,_ := range n.ringMap {
		n.remove(i)
	}
	close(n.exitChan)
	n.wg.Wait()

}


func (n *Node) Start () {
	n.log.Info.Println("Started Node")
	var i uint8

	done := make(chan bool)
	
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
