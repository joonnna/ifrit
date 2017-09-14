package node

import (
	"sync"
	"time"
	"github.com/joonnna/capstone/logger"
	"github.com/joonnna/capstone/util"
	"github.com/joonnna/capstone/protobuf"
)

type Node struct {
	log *logger.Log

	localAddr string

	viewMap map[string]*peer
	viewMutex sync.RWMutex

	ringMap map[uint8]*ring

	wg *sync.WaitGroup
	exitChan chan bool

	Communication

	numRings uint8

	gossipTimeout time.Duration
	monitorTimeout time.Duration
	viewUpdateTimeout time.Duration

	/*
	notes []Notes
	accusations
	timeouts
	*/
}

type Communication interface {
	ShutDown()
	Register(g gossip.GossipServer)
	Start() error
	Gossip(addr string, args *gossip.NodeInfo) (*gossip.Nodes, error)
	Monitor(addr string, args *gossip.Ping) (*gossip.Pong, error)
	Accuse(addrList []string, args *gossip.Accusation) (*gossip.Empty, error)
}

func (n *Node) gossipLoop () {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Exiting gossiping")
			n.wg.Done()
			return
		case <-time.After(n.gossipTimeout):
			peer := n.getRandomViewPeer()
			args := &gossip.NodeInfo{LocalAddr: n.localAddr}

			r, err := n.Communication.Gossip(peer.addr, args)
			if err != nil {
				n.log.Err.Println(err)
				continue
			}

			diff := util.SliceDiff(n.getViewAddrs(), r.GetAddrList())
			for _, addr := range diff {
				if addr == n.localAddr {
					continue
				}
				n.addViewPeer(addr)
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

				//TODO validate reply
				_, err = n.Communication.Monitor(succ.nodeId, msg)
				if err != nil {
					n.log.Info.Println("%s is dead, accusing", succ.nodeId)
					accuseMsg := &gossip.Accusation {
						Accuser: n.localAddr,
						Accused: succ.nodeId,
					}

					p, err := n.getViewPeer(succ.nodeId)
					if err == nil {
						p.addAccusation(n.localAddr)
					}
					
					//TODO validate reply
					_, err = n.Communication.Accuse(n.getViewAddrs(), accuseMsg)
					if err != nil {
						n.log.Err.Println(err)
						continue
					}
				}
			}	
		}
	}
}

func (n *Node) updateView () {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Stopping view update")
			n.wg.Done()
			return
		case <-time.After(n.viewUpdateTimeout):
			view := n.getView()
			for _, p := range view {
				if p.getAccusation() != nil {
					n.removePeer(p.addr)
				}
			}
		}
	}
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
		viewUpdateTimeout: time.Second * 2,
		gossipTimeout: time.Second * 3,
		monitorTimeout: time.Second * 3,
	}

	n.Communication.Register(n)

	for i = 0; i < numRings; i++ {
		n.ringMap[i] = newRing(i, n.localAddr)
	}

	if entryAddr != "" {
		n.addViewPeer(entryAddr)
	}

	return n
}

func (n *Node) ShutDownNode () {
	close(n.exitChan)
	n.wg.Wait()
}


func (n *Node) Start () {
	n.log.Info.Println("Started Node")
	var i uint8

	for i = 0; i < n.numRings; i++ {
		n.add(i)
	}

	go n.Communication.Start()
	n.wg.Add(3)
	go n.gossipLoop()
	go n.monitor()
	go n.updateView()
	go n.httpHandler()

	<-n.exitChan
	n.Communication.ShutDown()
	n.log.Info.Println("Exiting node")
}
