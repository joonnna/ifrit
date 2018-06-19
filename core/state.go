package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/netutil"
	"github.com/rs/cors"
)

const (
	httpPort = 12300
)

type state struct {
	ID string
	//Neighbours []string
	Next     string
	Prev     string
	HttpAddr string
	Trusted  bool
	AppAddr  string
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

	json.NewEncoder(buff).Encode(s)

	return bytes.NewReader(buff.Bytes())
}

func initHttp() (net.Listener, error) {
	l, err := netutil.ListenOnPort(httpPort)
	if err != nil {
		return nil, err
	}

	return l, nil
}

func (n *Node) httpHandler() {
	if n.httpListener == nil {
		return
	}

	//hostName, _ := os.Hostname()

	r := mux.NewRouter()
	r.HandleFunc("/shutdownNode", n.shutdownHandler)
	r.HandleFunc("/crashNode", n.crashHandler)
	r.HandleFunc("/corruptNode", n.corruptHandler)
	r.HandleFunc("/startrecording", n.recordHandler)
	r.HandleFunc("/numrequests", n.numRequestsHandler)
	r.HandleFunc("/numfailedrequests", n.numFailedRequestsHandler)
	r.HandleFunc("/latencies", n.latenciesHandler)
	r.HandleFunc("/neighbors", n.neighborsHandler)
	/*
		r.HandleFunc("/hosts", n.hostsHandler)
		r.HandleFunc("/state", n.stateHandler)
		r.HandleFunc("/dos", n.dosHandler)
	*/
	/*
		addrs, err := net.LookupHost(hostName)
		if err != nil {
			log.Error(err.Error())
			return
		}
	*/

	ip := netutil.GetLocalIP()
	port := strings.Split(n.httpListener.Addr().String(), ":")[1]
	n.httpAddr = fmt.Sprintf("%s:%d", ip, port)
	n.vizId = fmt.Sprintf("http://%s", n.httpAddr)

	handler := cors.Default().Handler(r)

	err := http.Serve(n.httpListener, handler)
	if err != nil {
		log.Error(err.Error())
		return
	}
}

/*
func (n *Node) hostsHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	addrs := n.getLivePeerAddrs()

	for _, p := range n.view.Live() {
		w.Write([]byte(fmt.Sprintf("%s\n", p.HttpAddr)))
	}
}

func (n *Node) stateHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	addrs := n.getLivePeerAddrs()

	for _, a := range addrs {
		w.Write([]byte(fmt.Sprintf("%s\n", a)))
	}

}
*/

func (n *Node) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	if n.trustedBootNode {
		log.Info("Boot node, ignoring shutdown request")
		return
	}

	if n.setExitFlag() {
		return
	}

	log.Info("Received shutdown request!")

	n.ShutDownNode()
}

func (n *Node) crashHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	if n.trustedBootNode {
		log.Info("Boot node, ignoring crash request")
		return
	}

	log.Info("Received crash request, shutting down local comm")

	n.server.ShutDown()
	n.failureDetector.Shutdown()
}

func (n *Node) corruptHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	log.Info("Received corrupt request, going rogue!")

	//n.setProtocol(spamAccusations{})
}

func (n *Node) dosHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("Received dos request, spamming!")

	var args dosArgs

	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		log.Error(err.Error())
		return
	}
	r.Body.Close()
	/*
		e := experiment{
			addr:    args.Addr, // "129.242.19.146:8100",
			maxConc: args.Conc,
		}*/

	//n.setGossipTimeout(args.Timeout)
	//n.setProtocol(e)
}

func (n *Node) recordHandler(w http.ResponseWriter, r *http.Request) {
	var args expArgs

	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		log.Error(err.Error())
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

	for _, p := range n.view.MyNeighbours() {
		ret += fmt.Sprintf("%s\n", p.Addr)
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

/* Periodically sends the nodes current state to the state server */
func (n *Node) updateState() {
	var i uint32
	client := &http.Client{}

	rings := n.view.NumRings()
	prevStates := make([]*state, 0, rings)

	for i = 0; i < rings; i++ {
		prevStates = append(prevStates, &state{})
	}

	for {
		select {
		case <-n.exitChan:
			n.wg.Done()
			return

		case <-time.After(n.vizUpdateTimeout):
			for i = 1; i <= rings; i++ {
				s := n.newState(i)

				idx := i - 1
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
	var succId, prevId string

	succ, prev := n.view.RingNeighbours(ringId)
	if succ != nil {
		succId = fmt.Sprintf("%s|%d", succ.Addr, ringId)
	}
	if prev != nil {
		prevId = fmt.Sprintf("%s|%d", prev.Addr, ringId)
	}

	return &state{
		ID:       fmt.Sprintf("%s|%d", n.self.Addr, ringId),
		Next:     succId,
		Prev:     prevId,
		HttpAddr: n.vizId,
		Trusted:  n.trustedBootNode,
		AppAddr:  n.vizAppAddr,
	}
}

/* Sends the node state to the state server*/
func (n *Node) updateReq(r io.Reader, c *http.Client) {
	addr := fmt.Sprintf("http://%s/update", n.vizAddr)
	req, err := http.NewRequest("POST", addr, r)
	if err != nil {
		log.Error(err.Error())
		return
	}

	resp, err := c.Do(req)
	if err != nil {
		log.Error(err.Error())
	} else {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
}

/* Sends a post request to the state server add endpoint */
func (n *Node) add(ringId uint32) error {
	s := n.newState(ringId)
	bytes := s.marshal()
	addr := fmt.Sprintf("http://%s/add", n.vizAddr)
	req, err := http.NewRequest("POST", addr, bytes)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.Error(err.Error())
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
	addr := fmt.Sprintf("http://%s/remove", n.vizAddr)
	req, err := http.NewRequest("POST", addr, bytes)
	if err != nil {
		log.Error(err.Error())
		return
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.Error(err.Error())
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
