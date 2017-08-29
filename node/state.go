package node

import (
	"encoding/json"
	"time"
	"strings"
	"bytes"
	"net/http"
	"io"
	"os"
	_"fmt"
)

type state struct {
	ID string
	Neighbours []string
}


/* Periodically sends the nodes current state to the state server*/
func (n *Node) updateState() {

	client := &http.Client{}
	for {
		s := n.newState()
		n.updateReq(s, client)
		time.Sleep(time.Second * 5)
	}

}
/* Creates a new state */
func (n *Node) newState() io.Reader {
	host, _ := os.Hostname()
	hostName := strings.Split(host, ".")[0]

	id := hostName

	s := state {
		ID: id,
		Neighbours: n.getNeighbours(),
	}

	buff := new(bytes.Buffer)

	err := json.NewEncoder(buff).Encode(s)
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
func (n *Node) add() {
	r := n.newState()
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

func (n *Node) remove() {
	r := n.newState()
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

