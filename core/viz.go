package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/netutil"
	"github.com/rs/cors"
)

const (
	httpPort = 12300
)

type viz struct {
	n *Node

	l          net.Listener
	httpServer *http.Server
	httpAddr   string
	trusted    bool

	id            string
	addr          string
	updateTimeout time.Duration

	exitChan chan bool
}

type state struct {
	ID       string
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

	json.NewEncoder(buff).Encode(s)

	return bytes.NewReader(buff.Bytes())
}

func newViz(n *Node, vizAddr string, updateInterval time.Duration, trusted bool) (*viz, error) {
	l, err := netutil.ListenOnPort(httpPort)
	if err != nil {
		return nil, err
	}

	v := &viz{
		updateTimeout: updateInterval,
		n:             n,
		addr:          vizAddr,
		exitChan:      make(chan bool, 1),
		httpAddr:      l.Addr().String(),
		trusted:       trusted,
	}

	log.Debug("http addr", "addr", v.httpAddr)

	r := mux.NewRouter()
	r.HandleFunc("/shutdownNode", v.shutdownHandler)
	r.HandleFunc("/byzantine", v.byzantineHandler)

	handler := cors.Default().Handler(r)

	v.httpServer = &http.Server{
		Handler: handler,
	}
	v.l = l

	return v, nil
}

func (v *viz) start() error {
	v.addToViz()
	go v.updateState()

	return v.httpServer.Serve(v.l)
}

func (v *viz) stop() {
	v.remove()
	v.httpServer.Close()
	close(v.exitChan)
}

func (v *viz) byzantineHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	log.Info("Got byzantine request, periodically not responding to pings!")

	go v.periodicallyPause(time.Second * 20)
}

func (v *viz) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()

	log.Info("Received shutdown request!")

	go v.n.Stop()
}

func (v *viz) periodicallyPause(d time.Duration) {
	for {
		select {
		case <-v.exitChan:
			return
		default:
			v.n.fd.stopServing(d)
		}

		time.Sleep(d * 3)
	}
}

/* Periodically sends the nodes current state to the state server */
func (v *viz) updateState() {
	var i uint32
	client := &http.Client{}

	rings := v.n.view.NumRings()
	prevStates := make([]*state, 0, rings)

	for i = 0; i < rings; i++ {
		prevStates = append(prevStates, &state{})
	}

	for {
		select {
		case <-v.exitChan:
			return

		case <-time.After(v.updateTimeout):
			for i = 1; i <= rings; i++ {
				s := v.newState(i)

				idx := i - 1
				if prevStates[idx].equal(s) {
					continue
				}

				prevStates[idx] = s

				v.updateReq(s.marshal(), client)
			}
		}
	}
}

/* Creates a new state */
func (v *viz) newState(ringId uint32) *state {
	var succId, prevId string

	succ, prev := v.n.view.MyRingNeighbours(ringId)
	if succ != nil {
		succId = fmt.Sprintf("%s|%d", succ.Addr, ringId)
	}
	if prev != nil {
		prevId = fmt.Sprintf("%s|%d", prev.Addr, ringId)
	}

	return &state{
		ID:       fmt.Sprintf("%s|%d", v.n.self.Addr, ringId),
		Next:     succId,
		Prev:     prevId,
		HttpAddr: v.httpAddr,
		Trusted:  v.trusted,
	}
}

/* Sends the node state to the state server*/
func (v *viz) updateReq(r io.Reader, c *http.Client) {
	addr := fmt.Sprintf("http://%s/update", v.addr)
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
func (v *viz) addToViz() {
	var i uint32

	addr := fmt.Sprintf("http://%s/add", v.addr)

	for i = 1; i <= v.n.view.NumRings(); i++ {
		s := v.newState(i)
		req, err := http.NewRequest("POST", addr, s.marshal())
		if err != nil {
			log.Error(err.Error())
			continue
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
}

func (v *viz) remove() {
	removeMsg := struct {
		HttpAddr string
	}{
		v.httpAddr,
	}

	buff := new(bytes.Buffer)

	json.NewEncoder(buff).Encode(removeMsg)

	addr := fmt.Sprintf("http://%s/remove", v.addr)
	req, err := http.NewRequest("POST", addr, bytes.NewReader(buff.Bytes()))
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
