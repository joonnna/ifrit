package node

import (
	"math/rand"
	"errors"
)

var (
	errNotFound = errors.New("No node info found")
)

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
func (r *ring) getRingList () []*ringId{
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
/*
func (r *ring) addToSuccList(id *ringId) {
	newSucc := id
	r.checkHighestLowest(newSucc)
	r.succList = append(r.succList, newSucc)
}
*/
