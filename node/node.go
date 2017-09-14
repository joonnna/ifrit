package node

import (
	"sync"
	"time"
	"github.com/joonnna/capstone/logger"
	_"math/big"
	"bytes"
	"encoding/json"
	"io"
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
/*
type nodeId struct {
	//Hash []byte
	Hash big.Int
	Addr string
}
*/

const (
	pingCode uint8 = 1
	gossipCode uint8 = 2
	accusationCode uint8 = 3
	rebuttalCode uint8 = 4
)

type message struct {
	Code uint8
	Payload []byte
	/*
	Operation uint8
	LocalAddr string
	AddrSlice []string
	*/
}




type Communication interface {
	SendMsg(addr string, data []byte) error
	ReceiveMsg(cb func(reader io.Reader))
	BroadcastMsg(data []byte)
	ShutDown()
}

func (n *Node) gossip () {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Exiting gossiping")
			n.wg.Done()
			return
		case <-time.After(n.gossipTimeout):
			view := n.getViewAddrs()
			if len(view) == 0 {
				continue
			}
			fullView := append(view, n.localAddr)

			payload := []byte(&gossip{AddrSlice: view, LocalAddr: n.localAddr})

			msg, err := newMsg(pingCode, payload)
			if err != nil {
				n.log.Err.Println(err)
				continue
			}

			peer := n.getRandomViewPeer()
			err = n.Communication.SendMsg(peer.addr, msg)
			if err != nil {
				n.log.Err.Println(err)
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
		/*
			msg, err := newMsg(n.localAddr, allLive, ping)
			if err != nil {
				n.log.Err.Println(err)
				continue
			}
		*/
			for _, ring := range n.ringMap {
				err = n.Communication.SendMsg(ring.getRingSuccAddr(), []byte("YOU DEAD BRO?"))
				if err != nil {
					n.log.Err.Println(err)
					//n.Communcation.BroadcastMsg(msg)
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
		case <-time.After(n.updateViewTimeout):
			for _, p := range r.viewMap {
				if p.getAccusation() != nil {
					n.removePeer(p.addr)
				}
			}
		}
	}
}

func (n *Node) receiveTraffic (reader io.Reader) {
	msg := &message{}

	err := json.NewDecoder(reader).Decode(msg)
	if err != nil {
		n.log.Err.Println(err)
		return
	}
	
	payloadReader := bytes.NewReader(msg.Payload)

	switch msg.Code {
	case ping:
		break
	case gossip:
		n.handleGossip(payloadReader)
	case accusation:
		break
	case rebuttal:
		break
	default:
		n.log.Err.Println("Unknown message received")
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
		updateTimeout: time.Second * 2,
		gossipTimeout: time.Second * 3,
		monitorTimeout: Time.Second * 3,
	}

	for i = 0; i < numRings; i++ {
		n.ringMap[i] = newRing(i, n.localAddr)
		n.ringMap[i].add(entryAddr)
	}

	if entryAddr != ""{
		n.addLiveNode(entryAddr)
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

	n.wg.Add(3)
	go n.gossip()
	go n.monitor()
	go n.updateView()

	go func() {
		<-n.exitChan
		n.Communication.ShutDown()
		n.log.Info.Println("Exiting node")
		n.wg.Done()
	}()

	n.Communication.ReceiveMsg(n.receiveTraffic)
}
