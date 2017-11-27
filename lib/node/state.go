package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joonnna/go-fireflies/lib/netutils"
	"github.com/rs/cors"
)

const (
	stateAddr = "http://129.242.19.146:8095"
	httpPort  = 12300
	//stateAddr = "http://localhost:8080"
)

type state struct {
	ID string
	//Neighbours []string
	Next     string
	Prev     string
	HttpAddr string
	Trusted  bool
}

type expArgs struct {
	MaxConc  int
	Byz      float32
	NumRings int
	Duration int
}

type dosArgs struct {
	Addr    string
	Conc    int
	Timeout int
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
	hostName, _ := os.Hostname()
	//hostName := netutils.GetLocalIP()
	l, err := netutils.ListenOnPort(hostName, httpPort)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	r := mux.NewRouter()
	r.HandleFunc("/shutdownNode", n.shutdownHandler)
	r.HandleFunc("/crashNode", n.crashHandler)
	r.HandleFunc("/corruptNode", n.corruptHandler)
	r.HandleFunc("/startrecording", n.recordHandler)
	r.HandleFunc("/numrequests", n.numRequestsHandler)
	r.HandleFunc("/numfailedrequests", n.numFailedRequestsHandler)
	r.HandleFunc("/latencies", n.latenciesHandler)
	r.HandleFunc("/dos", n.dosHandler)
	r.HandleFunc("/neighbors", n.neighborsHandler)

	port := strings.Split(l.Addr().String(), ":")[1]

	addrs, err := net.LookupHost(hostName)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	n.httpAddr = fmt.Sprintf("http://%s:%s", addrs[0], port)
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
	n.pinger.Shutdown()
}

func (n *Node) corruptHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	n.log.Info.Println("Received corrupt request, going rogue!")

	n.setProtocol(spamAccusations{})
}

func (n *Node) dosHandler(w http.ResponseWriter, r *http.Request) {
	n.log.Info.Println("Received dos request, spamming!")

	var args dosArgs

	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		n.log.Err.Println(err)
		return
	}
	r.Body.Close()

	e := experiment{
		addr:    args.Addr, // "129.242.19.146:8100",
		maxConc: args.Conc,
	}

	n.setGossipTimeout(args.Timeout)
	n.setProtocol(e)
}

func (n *Node) recordHandler(w http.ResponseWriter, r *http.Request) {
	var args expArgs

	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		n.log.Err.Println(err)
		return
	}
	r.Body.Close()

	go n.stats.doExp(&args)

	n.stats.setRecordFlag(true)
}

func (n *Node) numRequestsHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	w.Write([]byte(strconv.Itoa(n.stats.getCompletedRequests())))
}

func (n *Node) numFailedRequestsHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	w.Write([]byte(strconv.Itoa(n.stats.getFailedRequests())))
}

func (n *Node) neighborsHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	var ret string

	for _, r := range n.ringMap {
		succ, err := r.getRingSucc()
		if err != nil {
			continue
		}

		prev, err := r.getRingPrev()
		if err != nil {
			continue
		}

		ret += fmt.Sprintf("%s\n%s\n", succ.addr, prev.addr)
	}

	w.Write([]byte(ret))
}

func (n *Node) latenciesHandler(w http.ResponseWriter, r *http.Request) {
	var resp bytes.Buffer

	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	latencies := n.stats.getLatencies()

	for _, lat := range latencies {
		resp.WriteString(fmt.Sprintf("%f\n", lat))
	}

	w.Write(resp.Bytes())
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
			for _, r := range n.ringMap {
				s := n.newState(r.ringNum)

				idx := r.ringNum - 1
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
		n.log.Err.Println(err)
	} else {
		nextId = fmt.Sprintf("%s|%d", succ.addr, ringId)
	}

	prev, err := n.ringMap[ringId].getRingPrev()
	if err != nil {
		prevId = ""
		n.log.Err.Println(err)
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
	addr := fmt.Sprintf("%s/update", stateAddr)
	req, err := http.NewRequest("POST", addr, r)
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
func (n *Node) add(ringId uint32) error {
	s := n.newState(ringId)
	bytes := s.marshal()
	addr := fmt.Sprintf("%s/add", stateAddr)
	req, err := http.NewRequest("POST", addr, bytes)
	if err != nil {
		n.log.Err.Println(err)
		return err
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		n.log.Err.Println(err)
		return err
	} else {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
	return nil
}

func (n *Node) remove(ringId uint32) {
	s := n.newState(ringId)
	bytes := s.marshal()
	addr := fmt.Sprintf("%s/remove", stateAddr)
	req, err := http.NewRequest("POST", addr, bytes)
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
