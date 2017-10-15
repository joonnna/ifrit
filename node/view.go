package node

import (
	"sync"
	"time"

	"github.com/joonnna/firechain/logger"
)

type view struct {
	viewMap   map[string]*peer
	viewMutex sync.RWMutex

	liveMap   map[string]*peer
	liveMutex sync.RWMutex

	timeoutMap   map[string]*timeout
	timeoutMutex sync.RWMutex

	//Read only, don't need mutex
	ringMap  map[uint8]*ring
	numRings uint8

	viewUpdateTimeout time.Duration

	log *logger.Log
}

func newView(numRings uint8, log *logger.Log, id *peerId, addr string) *view {
	var i uint8

	v := &view{
		ringMap:           make(map[uint8]*ring),
		viewMap:           make(map[string]*peer),
		liveMap:           make(map[string]*peer),
		timeoutMap:        make(map[string]*timeout),
		viewUpdateTimeout: time.Second * 2,
		log:               log,
		numRings:          numRings,
	}

	for i = 0; i < numRings; i++ {
		v.ringMap[i] = newRing(i, id.id, id.key, addr)
	}

	return v
}

func (v *view) getViewAddrs() []string {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	ret := make([]string, len(v.viewMap))

	for _, v := range v.viewMap {
		ret = append(ret, v.addr)
	}

	return ret
}

func (v *view) getView() []*peer {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	ret := make([]*peer, 0)

	for _, v := range v.viewMap {
		if v != nil {
			ret = append(ret, v)
		}
	}

	return ret
}

func (v *view) getRandomViewPeer() *peer {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	var ret *peer

	for _, v := range v.viewMap {
		ret = v
		break
	}

	return ret
}

func (v *view) addViewPeer(p *peer) {
	v.viewMutex.Lock()
	defer v.viewMutex.Unlock()

	if _, ok := v.viewMap[p.key]; ok {
		v.log.Err.Printf("Tried to add peer twice to viewMap: %s", p.addr)
		return
	}

	v.viewMap[p.key] = p
}

func (v *view) removeViewPeer(key string) {
	v.viewMutex.Lock()
	defer v.viewMutex.Unlock()

	delete(v.viewMap, key)
}

func (v *view) viewPeerExist(key string) bool {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	_, ok := v.viewMap[key]

	return ok
}

func (v *view) getViewPeer(key string) *peer {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	var p *peer
	var ok bool

	if p, ok = v.viewMap[key]; !ok {
		return nil
	}

	return p
}

func (v *view) getNeighbours() []string {
	var neighbours []string
	for _, ring := range v.ringMap {
		succ, err := ring.getRingSucc()
		if err != nil {
			v.log.Info.Println(err)
		} else {
			neighbours = append(neighbours, succ.addr)
		}

		prev, err := ring.getRingPrev()
		if err != nil {
			v.log.Info.Println(err)
		} else {
			neighbours = append(neighbours, prev.addr)
		}
	}
	return neighbours
}

func (v *view) getLivePeers() []*peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	ret := make([]*peer, 0)

	for _, p := range v.liveMap {
		if p != nil {
			ret = append(ret, p)
		}
	}

	return ret
}

func (v *view) getLivePeerAddrs() []string {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	ret := make([]string, 0)

	for _, p := range v.liveMap {
		ret = append(ret, p.addr)
	}

	return ret
}

func (v *view) getLivePeer(key string) *peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	var ok bool
	var p *peer

	if p, ok = v.liveMap[key]; !ok {
		return nil
	}

	return p
}

func (v *view) addLivePeer(p *peer) {
	v.liveMutex.Lock()
	defer v.liveMutex.Unlock()

	if _, ok := v.liveMap[p.key]; ok {
		v.log.Err.Printf("Tried to add peer twice to liveMap: %s", p.addr)
		return
	}

	v.liveMap[p.key] = p

	for _, ring := range v.ringMap {
		ring.add(p.id, p.key, p.addr)
	}
}

func (v *view) removeLivePeer(key string) {
	v.liveMutex.Lock()
	defer v.liveMutex.Unlock()

	var ok bool
	var p *peer

	if p, ok = v.liveMap[key]; !ok {
		return
	}
	peerKey := p.key
	id := p.id

	v.log.Debug.Printf("Removed livePeer: %s", p.addr)
	delete(v.liveMap, key)

	for _, ring := range v.ringMap {
		err := ring.remove(id, peerKey)
		if err != nil {
			v.log.Err.Println(err)
			continue
		}
	}
}

func (v *view) timerExist(key string) bool {
	v.timeoutMutex.RLock()
	defer v.timeoutMutex.RUnlock()

	_, ok := v.timeoutMap[key]

	return ok
}

func (v *view) startTimer(key string, newNote *note, observer *peer, addr string) {
	v.timeoutMutex.Lock()
	defer v.timeoutMutex.Unlock()

	if t, ok := v.timeoutMap[key]; ok && newNote != nil {
		if t.lastNote.epoch >= newNote.epoch {
			return
		}
	}

	newTimeout := &timeout{
		observer:  observer,
		lastNote:  newNote,
		timeStamp: time.Now(),
		addr:      addr,
	}
	v.timeoutMap[key] = newTimeout
}

func (v *view) deleteTimeout(key string) {
	v.timeoutMutex.Lock()
	defer v.timeoutMutex.Unlock()

	delete(v.timeoutMap, key)
}

func (v *view) getAllTimeouts() map[string]*timeout {
	v.timeoutMutex.RLock()
	defer v.timeoutMutex.RUnlock()

	ret := make(map[string]*timeout)

	for k, val := range v.timeoutMap {
		ret[k] = val
	}

	return ret
}
