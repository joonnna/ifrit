package node

import (
	"errors"
	"sync"
	"time"

	"github.com/joonnna/firechain/logger"
)

var (
	errInvalidNeighbours = errors.New("Neighbours are nil ?!")
)

type view struct {
	viewMap   map[string]*peer
	viewMutex sync.RWMutex

	liveMap   map[string]*peer
	liveMutex sync.RWMutex

	timeoutMap   map[string]*timeout
	timeoutMutex sync.RWMutex

	//Read only, don't need mutex
	ringMap  map[uint32]*ring
	numRings uint32

	viewUpdateTimeout time.Duration

	log *logger.Log

	//Local id only used for special case in addLivePeer
	local *peerId
}

func newView(numRings uint32, log *logger.Log, id *peerId, addr string) (*view, error) {
	var i uint32
	var err error

	v := &view{
		ringMap:           make(map[uint32]*ring),
		viewMap:           make(map[string]*peer),
		liveMap:           make(map[string]*peer),
		timeoutMap:        make(map[string]*timeout),
		viewUpdateTimeout: time.Second * 2,
		log:               log,
		numRings:          numRings,
		local:             id,
	}

	for i = 0; i < numRings; i++ {
		v.ringMap[i], err = newRing((i + 1), id.id, addr)
		if err != nil {
			return nil, err
		}
	}

	return v, nil
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

/*
func (v *view) removeViewPeer(key string) {
	v.viewMutex.Lock()
	defer v.viewMutex.Unlock()

	delete(v.viewMap, key)
}
*/

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

	if _, ok := v.liveMap[p.key]; ok {
		v.log.Err.Printf("Tried to add peer twice to liveMap: %s", p.addr)
		v.liveMutex.Unlock()
		return
	}

	v.liveMap[p.key] = p
	v.liveMutex.Unlock()

	var prevId *peerId

	for num, ring := range v.ringMap {
		//TODO handle this differently? continue after failed ring add is dodgy
		//Although no errors "should" occur
		err := ring.add(p.id, p.addr)
		if err != nil {
			v.log.Err.Println(err)
			continue
		}

		succKey, prevKey, err := ring.findNeighbours(p.id)
		if err != nil {
			v.log.Err.Println(err)
			continue
		}

		succ := v.getViewPeer(succKey)
		prev := v.getViewPeer(prevKey)

		//Special case when prev is the local peer
		//do not care if local peer is succ, will not have accusations about myself
		if prev != nil {
			prevId = prev.peerId
		} else if prevKey == v.local.key {
			prevId = v.local
		}

		//Occurs when a fresh node starts up and has no nodes in its view
		//Or if the local node is either the new succ or prev
		//TODO handle this differently?
		if succ == nil || prevId == nil {
			continue
		}

		acc := succ.getRingAccusation(num)
		if acc != nil && acc.accuser.equal(prevId) {
			succ.removeAccusation(num)
		}
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
	id := p.id

	v.log.Debug.Printf("Removed livePeer: %s", p.addr)
	delete(v.liveMap, key)

	for _, ring := range v.ringMap {
		err := ring.remove(id)
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

func (v *view) getTimeout(key string) *timeout {
	v.timeoutMutex.RLock()
	defer v.timeoutMutex.RUnlock()

	t, ok := v.timeoutMap[key]

	if !ok {
		return nil
	}

	return t
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

func (v *view) isPrev(curr, toCheck *peer, ringNum uint32) bool {
	r, ok := v.ringMap[ringNum]
	if !ok {
		v.log.Err.Println("accusation on non-existing ring")
		return false
	}

	prev, err := r.isPrev(curr.id, toCheck.id)
	if err != nil {
		v.log.Err.Println(err)
		return false
	}

	return prev
}

func (v *view) shouldBeNeighbours(id *peerId) bool {
	for _, r := range v.ringMap {
		if r.betweenNeighbours(id.id) {
			return true
		}
	}
	return false
}

func (v *view) findNeighbours(id *peerId) []string {
	keys := make([]string, 0)
	exist := make(map[string]bool)

	for _, r := range v.ringMap {
		succ, prev, err := r.findNeighbours(id.id)
		if err != nil {
			continue
		}
		if _, ok := exist[succ]; !ok {
			keys = append(keys, succ)
			exist[succ] = true
		}

		if _, ok := exist[prev]; !ok {
			keys = append(keys, prev)
			exist[prev] = true
		}
	}
	return keys
}
