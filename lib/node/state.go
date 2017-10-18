package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

type state struct {
	ID string
	//Neighbours []string
	Next     string
	Prev     string
	HttpAddr string
	Trusted  bool
}

func (s state) equal(other *state) bool {
	return (s.Next == other.Next && s.Prev == other.Prev)
}

func (s *state) marshal() io.Reader {
	buff := new(bytes.Buffer)

	_ = json.NewEncoder(buff).Encode(s)

	return bytes.NewReader(buff.Bytes())
}

func (n *Node) httpHandler(c chan bool) {
	var l net.Listener
	var err error
	var addr string

	host, _ := os.Hostname()
	hostName := strings.Split(host, ".")[0]

	port := 5000

	for {
		addr = fmt.Sprintf("%s:%d", hostName, (1000 + (mrand.Int() % port)))
		l, err = net.Listen("tcp", addr)

		if err != nil {
			n.log.Err.Println(err)
		} else {
			break
		}
		port += 1
	}

	r := mux.NewRouter()
	r.HandleFunc("/shutdownNode", n.shutdownHandler)
	r.HandleFunc("/crashNode", n.crashHandler)
	r.HandleFunc("/corruptNode", n.corruptHandler)

	n.httpAddr = fmt.Sprintf("http://%s", addr)

	go func() {
		<-n.exitChan
		l.Close()
	}()

	close(c)

	handler := cors.Default().Handler(r)

	err = http.Serve(l, handler)
	if err != nil {
		n.log.Err.Println(err)
		return
	}
}

func (n *Node) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	if n.trustedBootNode {
		n.log.Info.Println("Boot node, ignoring shutdown request")
		return
	}

	if n.setExitFlag() {
		return
	}

	n.log.Info.Println("Received shutdown request!")

	n.ShutDownNode()
}

func (n *Node) crashHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	if n.trustedBootNode {
		n.log.Info.Println("Boot node, ignoring crash request")
		return
	}

	n.log.Info.Println("Received crash request, shutting down local comm")

	n.server.ShutDown()
}

func (n *Node) corruptHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	n.log.Info.Println("Received corrupt request, going rogue!")

	n.setProtocol(SpamAccusationsProtocol)
}

/* Periodically sends the nodes current state to the state server*/
func (n *Node) updateState() {
	client := &http.Client{}

	prevStates := make([]*state, n.numRings)

	for i := 0; i < len(prevStates); i++ {
		prevStates[i] = &state{}
	}

	for {
		select {
		case <-n.exitChan:
			n.wg.Done()
			return

		case <-time.After(time.Second * 5):
			for num, r := range n.ringMap {
				s := n.newState(r.ringNum)

				idx := num - 1

				if prevStates[idx].equal(s) {
					continue
				}

				prevStates[idx] = s

				n.updateReq(s.marshal(), client)
			}

		}
	}
}

/* Creates a new state */
func (n *Node) newState(ringId uint32) *state {
	id := fmt.Sprintf("%s|%d", n.addr, ringId)

	var nextId, prevId string

	succ, err := n.ringMap[ringId].getRingSucc()
	if err != nil {
		nextId = ""
	} else {
		nextId = fmt.Sprintf("%s|%d", succ.addr, ringId)
	}

	prev, err := n.ringMap[ringId].getRingPrev()
	if err != nil {
		prevId = ""
	} else {
		prevId = fmt.Sprintf("%s|%d", prev.addr, ringId)
	}

	return &state{
		ID: id,
		//Neighbours: n.getNeighbourAddrs(),
		Next:     nextId,
		Prev:     prevId,
		HttpAddr: n.httpAddr,
		Trusted:  n.trustedBootNode,
	}
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
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
}

/* Sends a post request to the state server add endpoint */
func (n *Node) add(ringId uint32) {
	s := n.newState(ringId)
	bytes := s.marshal()
	req, err := http.NewRequest("POST", "http://localhost:8080/add", bytes)
	if err != nil {
		n.log.Err.Println(err)
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		n.log.Err.Println(err)
	} else {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
}

func (n *Node) remove(ringId uint32) {
	s := n.newState(ringId)
	bytes := s.marshal()
	req, err := http.NewRequest("POST", "http://localhost:8080/remove", bytes)
	if err != nil {
		n.log.Err.Println(err)
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		n.log.Err.Println(err)
	} else {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
}

func (n *Node) setExitFlag() bool {
	n.exitMutex.Lock()
	defer n.exitMutex.Unlock()

	if n.exitFlag {
		return true
	}

	n.exitFlag = true
	return false
}
