package worm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	log "github.com/inconshreveable/log15"
)

const (
	vizEndpoint = "blockupdate"
)

type chainState struct {
	Currblock uint64
	Errors    uint32
}

func (cs *chainState) marshal() io.Reader {
	buff := new(bytes.Buffer)

	err := json.NewEncoder(buff).Encode(cs)
	if err != nil {
		log.Error(err.Error())
	}

	return bytes.NewReader(buff.Bytes())
}

func (w *Worm) getNewHosts() {
	existing := w.getHosts()

	for _, h := range existing {
		tmp := h
		w.wp.Submit(func() {
			w.doHostRequest(tmp)
		})
	}
}

func (w *Worm) doHostRequest(host string) {
	url := fmt.Sprintf("http://%s/%s", host, w.hostEndpoint)

	resp, err := http.Get(url)
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err.Error())
		return
	}

	hosts := strings.Split(string(buf), "\n")

	for _, h := range hosts {
		if h == "" {
			continue
		}
		if exists := w.hostExist(h); !exists {
			w.addHost(h)
		}
	}
}

func (w *Worm) getHostStates() {
	var wg sync.WaitGroup

	existing := w.getHosts()

	for _, h := range existing {
		tmp := h
		wg.Add(1)
		w.wp.Submit(func() {
			w.doStateRequest(tmp, &wg)
		})
	}

	wg.Wait()
}

func (w *Worm) doStateRequest(host string, wg *sync.WaitGroup) {
	defer wg.Done()

	url := fmt.Sprintf("http://%s/%s", host, w.stateEndpoint)

	resp, err := http.Get(url)
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err.Error())
		return
	}

	w.addState(host, buf)
}

func (w *Worm) vizUpdate(blockNum uint64, errors uint32) {
	c := &chainState{
		Currblock: blockNum,
		Errors:    errors,
	}

	url := fmt.Sprintf("http://%s/%s", w.vizAddr, vizEndpoint)

	req, err := http.NewRequest("POST", url, c.marshal())
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
