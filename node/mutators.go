package node

import (
	"errors"
)

var (
	errNotFound = errors.New("No node info found")
)

func (r *ring) getRingList () []*ringId {
	r.ringMutex.Lock()
	defer r.ringMutex.Unlock()

	ret := make([]*ringId, len(r.succList))
	copy(ret, r.succList)
	return ret
}

func (r *ring) getRingSucc() (ringId, error) {
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
	
	for k, v := range n.viewMap {
		ret = append(ret, v.addr)
	}

	return ret
}

func (n *Node) getRandomViewPeer() peer {
	n.viewMutex.RLock()
	defer n.viewMutex.RUnlock()
	
	var ret peer

	for k, v := range n.viewMap {
		ret = *v
		break
	}

	return ret
}


func (n *Node) addViewPeer(addr string) {
	n.viewMutex.Lock()
	defer n.viewMutex.Unlock()

	n.viewMap[addr] = newPeer(addr)
}
/*
func (n *Node) removeViewPeer(addr string) {
	n.viewMutex.Lock()
	defer n.viewMutex.Unlock()

	delete(n.viewMap, addr)
}
*/
func (n *Node) removePeer (key string) {
	n.viewMutex.Lock()
	defer n.viewMutex.Unlock()
	
	p := n.viewMap[key]

	for k, ring := range n.ringMap {
		ring.remove(p.addr)
		n.updateState(k)
	}

	delete(n.viewMap, key)
}




/*

func (n *Node) getLiveNodeAddrs() []string {
	n.liveMutex.RLock()

	var ret []string

	for _, id := range n.live {
		ret = append(ret, id)
	}

	n.liveMutex.RUnlock()
	return ret
}

func (n *Node) getLiveNodes() []string {
	n.liveMutex.RLock()

	ret := make([]string, len(n.live))
	copy(ret, n.live)

	n.liveMutex.RUnlock()
	return ret
}

func (n *Node) getRandomLiveNode() string {
	n.liveMutex.RLock()

	nodeIdx := rand.Int() % len(n.live)
	ret := n.live[nodeIdx]

	n.liveMutex.RUnlock()
	return ret
}


func (n *Node) addLiveNode(addr string) {
	n.liveMutex.Lock()

	n.live = append(n.live, addr)

	n.liveMutex.Unlock()
}

func (n *Node) removeLiveNode(addr string) {
	n.liveMutex.Lock()

	for i, id := range n.live {
		if id == addr {
			n.live = append(n.live[:i], n.live[i+1:]...)
			break
		}
	}

	n.liveMutex.Unlock()
}
*/
/*
func (r *ring) addToSuccList(id *ringId) {
	newSucc := id
	r.checkHighestLowest(newSucc)
	r.succList = append(r.succList, newSucc)
}
*/

/*
func (r *ring) setRingSucc(id *ringId) {
	r.succ = id
	r.checkHighestLowest(id)
	r.addToSuccList(id)
}

func (r *ring) setRingPrev(id *ringId) {
	r.prev = id
	r.checkHighestLowest(id)
	r.addToSuccList(id)
}
*/
