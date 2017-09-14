package node

import (
	"encoding/json"
	_"time"
	"bytes"
	"net/http"
	"io"
	"io/ioutil"
	"fmt"
	"net"
	"os"
	"strings"
)

type state struct {
	ID string
	//Neighbours []string
	Next string
	Prev string
}

func (n *Node) httpHandler() {
	var l net.Listener
	var err error

	host, _ := os.Hostname()
	hostName := strings.Split(host, ".")[0]

	port := 2345

	for {
		l, err = net.Listen("tcp", fmt.Sprintf("%s:%d", hostName, port))
		if err != nil {
			n.log.Err.Println(err)
		} else {
			break
		}
		port += 1
	}

	http.HandleFunc(fmt.Sprintf("%s:%d/shutdownNode", hostName, port), n.shutdownHandler)

	go func() {
		<-n.exitChan
		l.Close()
	}()

	err = http.Serve(l, nil)
	if err != nil {
		n.log.Err.Println(err)
		return
	}
}

func (n *Node) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()
	n.ShutDownNode()
}


/* Periodically sends the nodes current state to the state server*/
func (n *Node) updateState(ringId uint8){
	client := &http.Client{}
	s := n.newState(ringId)
	n.updateReq(s, client)
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

