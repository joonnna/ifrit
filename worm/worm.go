package worm

import (
	"sync"
	"time"

	"github.com/joonnna/ifrit/log"
	"github.com/joonnna/workerpool"
)

type Worm struct {
	hostMutex    sync.RWMutex
	hostMap      map[string]bool
	hostEndpoint string

	stateEndpoint string
	stateCmp      func([]byte, []byte) error
	stateMap      map[string][]byte
	stateMutex    sync.RWMutex

	exitCh chan bool

	wp *workerpool.Dispatcher

	hostTicker  *time.Ticker
	stateTicker *time.Ticker
}

func NewWorm(cmp func([]byte, []byte) error, hosts, state string, interval uint) *Worm {
	return &Worm{
		stateCmp:      cmp,
		hostEndpoint:  hosts,
		stateEndpoint: state,
		stateTicker:   time.NewTicker(time.Second * time.Duration(interval)),
		hostTicker:    time.NewTicker(time.Second * time.Duration(interval*2)),
		exitCh:        make(chan bool),
		wp:            workerpool.NewDispatcher(50),
		hostMap:       make(map[string]bool),
		stateMap:      make(map[string][]byte),
	}
}

func (w *Worm) Start() {
	go w.hostLoop()
	go w.stateLoop()
}

func (w *Worm) Stop() {
	close(w.exitCh)
}

func (w *Worm) stateLoop() {
	for {
		select {
		case <-w.stateTicker.C:
			w.getHostStates()
			w.cmpStates()
		case <-w.exitCh:
			return
		}
	}
}

func (w *Worm) hostLoop() {
	for {
		select {
		case <-w.hostTicker.C:
			w.getNewHosts()
		case <-w.exitCh:
			return
		}
	}
}

func (w *Worm) hostExist(host string) bool {
	w.hostMutex.RLock()
	defer w.hostMutex.RUnlock()

	_, ok := w.hostMap[host]

	return ok
}

func (w *Worm) AddHost(host string) {
	w.addHost(host)
}

func (w *Worm) addHost(host string) {
	w.hostMutex.Lock()
	defer w.hostMutex.Unlock()

	w.hostMap[host] = true
}

func (w *Worm) getHosts() []string {
	w.hostMutex.RLock()
	defer w.hostMutex.RUnlock()

	idx := 0
	ret := make([]string, len(w.hostMap))

	for h, _ := range w.hostMap {
		ret[idx] = h
		idx++
	}

	return ret
}

func (w *Worm) addState(host string, state []byte) {
	w.stateMutex.Lock()
	defer w.stateMutex.Unlock()

	w.stateMap[host] = state
}

// TODO n^2... fix plz
func (w *Worm) cmpStates() {
	w.stateMutex.RLock()
	defer w.stateMutex.RUnlock()

	for _, s := range w.stateMap {
		for _, s2 := range w.stateMap {
			err := w.stateCmp(s, s2)
			if err != nil {
				log.Error(err.Error())
			}
		}
	}
}
