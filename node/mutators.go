package node

import (
	"errors"
	"time"
)

var (
	errNotFound     = errors.New("No node info found")
	errPeerNotFound = errors.New("No peer info found")
)

func (r *ring) getRingList() []*ringId {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	ret := make([]*ringId, len(r.succList))
	copy(ret, r.succList)
	return ret
}

func (r *ring) getRingSucc() (ringId, error) {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	len := len(r.succList)

	if len == 0 {
		return ringId{}, errNotFound
	} else {
		idx := (r.ownIdx + 1) % len
		return *r.succList[idx], nil
	}
}

func (r *ring) getRingSuccAddr() (string, error) {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	len := len(r.succList)

	if len == 0 {
		return "", errNotFound
	} else {
		idx := (r.ownIdx + 1) % len
		return r.succList[idx].nodeId, nil
	}
}

func (r *ring) getRingPrev() (ringId, error) {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	len := len(r.succList)

	if len == 0 {
		return ringId{}, errNotFound
	} else {
		idx := (r.ownIdx - 1) % len
		if idx < 0 {
			idx = idx + len
		}
		return *r.succList[idx], nil
	}
}

func (n *Node) getViewAddrs() []string {
	n.viewMutex.RLock()
	defer n.viewMutex.RUnlock()

	ret := make([]string, len(n.viewMap))

	for _, v := range n.viewMap {
		ret = append(ret, v.addr)
	}

	return ret
}

func (n *Node) getView() []*peer {
	n.viewMutex.RLock()
	defer n.viewMutex.RUnlock()

	ret := make([]*peer, 0)

	for _, v := range n.viewMap {
		ret = append(ret, v)
	}

	return ret
}

func (n *Node) getRandomViewPeer() *peer {
	n.viewMutex.RLock()
	defer n.viewMutex.RUnlock()

	var ret *peer

	for _, v := range n.viewMap {
		ret = v
		break
	}

	return ret
}

func (n *Node) addViewPeer(p *peer) {
	n.viewMutex.Lock()
	defer n.viewMutex.Unlock()

	if _, ok := n.viewMap[p.addr]; ok {
		n.log.Err.Printf("Tried to add peer twice to viewMap: %s", p.addr)
		return
	}

	n.viewMap[p.addr] = p
}

func (n *Node) removeViewPeer(key string) {
	n.viewMutex.Lock()
	defer n.viewMutex.Unlock()

	delete(n.viewMap, key)
}

func (n *Node) viewPeerExist(key string) bool {
	n.viewMutex.RLock()
	defer n.viewMutex.RUnlock()

	_, ok := n.viewMap[key]

	return ok
}

func (n *Node) getViewPeer(key string) *peer {
	n.viewMutex.RLock()
	defer n.viewMutex.RUnlock()

	var p *peer
	var ok bool

	if p, ok = n.viewMap[key]; !ok {
		return nil
	}

	return p
}

func (n *Node) getNeighbours() []string {
	var neighbours []string
	for _, ring := range n.ringMap {
		succ, err := ring.getRingSucc()
		if err != nil {
			n.log.Info.Println(err)
		}

		prev, err := ring.getRingPrev()
		if err != nil {
			n.log.Info.Println(err)
		}
		neighbours = append(neighbours, succ.nodeId, prev.nodeId)
	}
	return neighbours
}

func (n *Node) getLivePeers() []*peer {
	n.liveMutex.RLock()
	defer n.liveMutex.RUnlock()

	ret := make([]*peer, 0)

	for _, p := range n.liveMap {
		if p != nil {
			ret = append(ret, p)
		}
	}

	return ret
}

func (n *Node) getLivePeerAddrs() []string {
	n.liveMutex.RLock()
	defer n.liveMutex.RUnlock()

	ret := make([]string, 0)

	for _, p := range n.liveMap {
		ret = append(ret, p.addr)
	}

	return ret
}

func (n *Node) getLivePeer(key string) *peer {
	n.liveMutex.RLock()
	defer n.liveMutex.RUnlock()

	var ok bool
	var p *peer

	if p, ok = n.liveMap[key]; !ok {
		return nil
	}

	return p
}

func (n *Node) addLivePeer(p *peer) {
	n.liveMutex.Lock()
	defer n.liveMutex.Unlock()

	if _, ok := n.liveMap[p.addr]; ok {
		n.log.Err.Printf("Tried to add peer twice to liveMap: %s", p.addr)
		return
	}

	n.liveMap[p.addr] = p

	for k, ring := range n.ringMap {
		ring.add(p.id, p.addr)
		n.updateState(k)
	}
}

func (n *Node) removeLivePeer(key string) {
	n.liveMutex.Lock()
	defer n.liveMutex.Unlock()

	var ok bool
	var p *peer

	if p, ok = n.liveMap[key]; !ok {
		return
	}
	addr := p.addr
	id := p.id

	n.log.Debug.Printf("Removed livePeer: %s", key)
	delete(n.liveMap, key)

	for k, ring := range n.ringMap {
		err := ring.remove(id, addr)
		if err != nil {
			n.log.Err.Println(err)
			continue
		}
		n.updateState(k)
	}
}

func (n *Node) timerExist(addr string) bool {
	n.timeoutMutex.RLock()
	defer n.timeoutMutex.RUnlock()

	_, ok := n.timeoutMap[addr]

	return ok
}

func (n *Node) startTimer(addr string, newNote *note, observer string) {
	n.timeoutMutex.Lock()
	defer n.timeoutMutex.Unlock()

	if t, ok := n.timeoutMap[addr]; ok {
		if !t.lastNote.isMoreRecent(newNote.epoch) {
			return
		}
	}

	newTimeout := &timeout{
		observer:  observer,
		lastNote:  newNote,
		timeStamp: time.Now(),
	}
	n.timeoutMap[addr] = newTimeout
}

func (n *Node) deleteTimeout(key string) {
	n.timeoutMutex.Lock()
	defer n.timeoutMutex.Unlock()

	delete(n.timeoutMap, key)
}

func (n *Node) getAllTimeouts() map[string]*timeout {
	n.timeoutMutex.RLock()
	defer n.timeoutMutex.RUnlock()

	ret := make(map[string]*timeout)

	for k, val := range n.timeoutMap {
		ret[k] = val
	}

	return ret
}

func (n *Node) setEpoch(newEpoch uint64) {
	n.epochMutex.Lock()
	defer n.epochMutex.Unlock()

	n.epoch = newEpoch
}

func (n *Node) getEpoch() uint64 {
	n.epochMutex.RLock()
	defer n.epochMutex.RUnlock()

	return n.epoch
}
