package node

import (
	_"strings"
	_"os"
	_"fmt"
	"sync"
	"time"
	"bytes"
	"encoding/json"
	_"math/rand"
	"github.com/joonnna/capstone/util"
	"github.com/joonnna/capstone/logger"
)

type Node struct {
	log *logger.Log

	neighbours []string
	neighbourMutex sync.RWMutex

	Communication
}

type Communication interface {
	Ping(addr string, addrSlice []string) error
	ReceivePings(cb func(data []byte))
}

func (n *Node) gossip() {
	for {		
		time.Sleep(5)

		neighbours := n.getNeighbours()
		if len(neighbours) == 0 {
			continue
		}

		neighbour := n.getRandomNeighbour()
		err := n.Communication.Ping(neighbour, neighbours)
		if err != nil {
			n.log.Err.Println(err)
		}
	}
}

func (n *Node) receiveGossip(data []byte) {
	reader := bytes.NewReader(data)

	msg := &util.Message{}

	err := json.NewDecoder(reader).Decode(msg)
	if err != nil {
		n.log.Err.Println(err)
	}

	diff := util.SliceDiff(n.getNeighbours(), msg.AddrSlice)
	
	for _, addr := range diff {
		n.addNeighbour(addr)
	}
}

func NewNode(entryAddr string, comm Communication, log *logger.Log) *Node {
	n := &Node{
		Communication: comm,
		log: log,
	}
	
	if entryAddr != ""{
		n.addNeighbour(entryAddr)
	}

	return n
}


func (n *Node) Start() {
	n.log.Info.Println("Started Node")
	n.add()
	go n.gossip()
	go n.updateState()

	n.Communication.ReceivePings(n.receiveGossip)
}
