package worm

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	log "github.com/inconshreveable/log15"
)

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
