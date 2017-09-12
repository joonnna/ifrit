package node

import (
	"encoding/json"
	_"time"
	"bytes"
	"net/http"
	"io"
	"fmt"
)

type state struct {
	ID string
	//Neighbours []string
	Next string
	Prev string
}


/* Periodically sends the nodes current state to the state server*/
func (n *Node) updateState(ringId uint8){
	client := &http.Client{}
	s := n.newState(ringId)
	n.updateReq(s, client)

	/*
	for {
		select {
		case <-n.exitChan:
			n.log.Info.Println("Exiting state")
			n.wg.Done()
			return
		case <-time.After(time.Second * 5):
		}
	}
	*/
}

/* Creates a new state */
func (n *Node) newState(ringId uint8) io.Reader {
	id := fmt.Sprintf("%s|%d", n.localAddr, ringId)

	var nextId, prevId string

	succ, err := n.ringMap[ringId].getRingSucc()
	if err != nil {
		nextId = ""
	} else {
		nextId = fmt.Sprintf("%s|%d", succ.nodeId, ringId)
	}

	prev, err := n.ringMap[ringId].getRingPrev()
	if err != nil {
		prevId = ""
	} else {
		prevId = fmt.Sprintf("%s|%d", prev.nodeId, ringId)
	}

	s := state {
		ID: id,
		//Neighbours: n.getNeighbourAddrs(),
		Next: nextId,
		Prev: prevId,
	}

	buff := new(bytes.Buffer)

	err = json.NewEncoder(buff).Encode(s)
	if err != nil {
		n.log.Err.Println(err)
	}

	return bytes.NewReader(buff.Bytes())
}

/* Sends the node state to the state server*/
func (n *Node) updateReq(r io.Reader, c *http.Client) {
	req, err := http.NewRequest("POST", "http://localhost:8080/update", r)
	if err != nil {
		n.log.Err.Println(err)
	}

	resp, err := c.Do(req)
	if err != nil {
		n.log.Err.Println(err)
	} else {
		resp.Body.Close()
	}
}
/* Sends a post request to the state server add endpoint */
func (n *Node) add(ringId uint8) {
	r := n.newState(ringId)
	req, err := http.NewRequest("POST", "http://localhost:8080/add", r)
	if err != nil {
		n.log.Err.Println(err)
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		n.log.Err.Println(err)
	} else {
		resp.Body.Close()
	}
}

func (n *Node) remove(ringId uint8) {
	r := n.newState(ringId)
	req, err := http.NewRequest("POST", "http://129.242.22.74:7560/remove", r)
	if err != nil {
		n.log.Err.Println(err)
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		n.log.Err.Println(err)
	} else {
		resp.Body.Close()
	}
}

