package worm

import (
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/workerpool"
)

type Worm struct {
	hostMutex    sync.RWMutex
	hostMap      map[string]bool
	hostEndpoint string

	stateEndpoint    string
	stateCmp         func([]byte, []byte) (uint64, error)
	stateMap         map[string][]byte
	stateMutex       sync.RWMutex
	maxBlock         uint64
	totalStateErrors uint32

	vizAddr string

	exitCh chan bool

	wp *workerpool.Dispatcher

	hostTicker  *time.Ticker
	stateTicker *time.Ticker
}

func NewWorm(cmp func([]byte, []byte) (uint64, error), hosts, state, viz string, interval uint) *Worm {
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
		vizAddr:       viz,
	}
}

func (w *Worm) Start() {
	w.wp.Start()
	go w.hostLoop()
	go w.stateLoop()
}

func (w *Worm) Stop() {
	close(w.exitCh)
}

func (w *Worm) stateLoop() {
	for {
		select {
		case <-w.exitCh:
			return

		case <-w.stateTicker.C:
			w.getHostStates()
			w.cmpStates()
			w.vizUpdate(w.maxBlock, w.totalStateErrors)
		}
	}
}

func (w *Worm) hostLoop() {
	for {
		h := w.getHosts()
		log.Debug("Hosts", "amount", len(h), "addrs", h)
		select {
		case <-w.exitCh:
			return
		case <-w.hostTicker.C:
			w.getNewHosts()
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

	var maxBlock uint64 = 0
	var err error

	errors := 0

	for k, s := range w.stateMap {
		for k2, s2 := range w.stateMap {
			if s == nil || s2 == nil || k == k2 {
				continue
			}
			maxBlock, err = w.stateCmp(s, s2)
			if err != nil {
				log.Error(err.Error())
				errors++
			}
		}
	}

	if len(w.stateMap) == 0 {
		return
	}
	w.totalStateErrors += uint32(errors / len(w.stateMap))
	if maxBlock > w.maxBlock {
		w.maxBlock = maxBlock
	}
	log.Debug("Different states", "amount", errors)
	log.Debug("Total errors", "amount", w.totalStateErrors)
	log.Debug("Max block", "num", w.maxBlock)
}
