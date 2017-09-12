package node

import (
	"sync"
	"time"
	"github.com/joonnna/capstone/logger"
	_"math/big"
	"bytes"
	"encoding/json"
	"github.com/joonnna/capstone/util"
)

type Node struct {
	log *logger.Log

	localAddr string

	live []string
	liveMutex sync.RWMutex

	ringMap map[uint8]*ring

	wg *sync.WaitGroup
	exitChan chan bool

	Communication

	numRings uint8

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

type message struct {
	LocalAddr string
	AddrSlice []string
}


type Communication interface {
	Ping(addr string, data []byte) error
	ReceivePings(cb func(data []byte))
	ShutDown()
}


func (n *Node) gossip() {
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Exiting gossiping")
			n.wg.Done()
			return
		case <-time.After(time.Second * 5):
			live := n.getLiveNodes()
			if len(live) == 0 {
				continue
			}
			allLive := append(live, n.localAddr)

			msg, err := newMsg(n.localAddr, allLive)
			if err != nil {
				n.log.Err.Println(err)
				continue
			}

			liveNode := n.getRandomLiveNode()
			err = n.Communication.Ping(liveNode, msg)
			if err != nil {
				n.log.Err.Println(err)
				n.removeLiveNode(liveNode)
				n.updateState(0)
			}
		}
	}
}

func (n *Node) receiveGossip(data []byte) {
	reader := bytes.NewReader(data)

	msg := &message{}

	err := json.NewDecoder(reader).Decode(msg)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	diff := util.SliceDiff(n.getLiveNodes(), msg.AddrSlice)
	for _, addr := range diff {
		if addr == n.localAddr {
			continue
		}
		
		for id, ring := range n.ringMap {
			err = ring.add(addr)
			if err != nil {
				n.log.Err.Println(err)
			}
			n.updateState(id)
		}
		n.addLiveNode(addr)
	}
}

func NewNode(entryAddr string, comm Communication, log *logger.Log, addr string, numRings uint8) *Node {
	var i uint8

	n := &Node{
		Communication: comm,
		log: log,
		exitChan: make(chan bool, 1),
		wg: &sync.WaitGroup{},
		localAddr: addr,
		ringMap: make(map[uint8]*ring),
		numRings: numRings,
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

func (n *Node) ShutDownNode() {
	close(n.exitChan)
	n.wg.Wait()
}


func (n *Node) Start() {
	n.log.Info.Println("Started Node")
	var i uint8

	for i = 0; i < n.numRings; i++ {
		n.add(i)
	}

	n.wg.Add(2)
	go n.gossip()

	go func() {
		<-n.exitChan
		n.Communication.ShutDown()
		n.log.Info.Println("Exiting node")
		n.wg.Done()
	}()

	n.Communication.ReceivePings(n.receiveGossip)
}
