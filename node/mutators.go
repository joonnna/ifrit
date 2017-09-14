package node

import (
	"errors"
)

var (
	errNotFound = errors.New("No node info found")
	errPeerNotFound = errors.New("No peer info found")
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
	
	for _, v := range n.viewMap {
		ret = append(ret, v.addr)
	}

	return ret
}

func (n *Node) getView() []peer {
	n.viewMutex.RLock()
	defer n.viewMutex.RUnlock()

	ret := make([]peer, len(n.viewMap))
	
	for _, v := range n.viewMap {
		ret = append(ret, *v)
	}

	return ret
}



func (n *Node) getRandomViewPeer() peer {
	n.viewMutex.RLock()
	defer n.viewMutex.RUnlock()
	
	var ret peer

	for _, v := range n.viewMap {
		ret = *v
		break
	}

	return ret
}


func (n *Node) addViewPeer(addr string) {
	n.viewMutex.Lock()
	defer n.viewMutex.Unlock()

	peer := newPeer(addr)

	n.viewMap[addr] = peer

	for k, ring := range n.ringMap {
		ring.add(peer.addr)
		n.updateState(k)
	}

}

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

func (n *Node) viewPeerExist (key string) bool{
	n.viewMutex.Lock()
	defer n.viewMutex.Unlock()

	_, ok := n.viewMap[key]

	return ok
}

func (n *Node) getViewPeer (key string) (peer, error) {
	n.viewMutex.Lock()
	defer n.viewMutex.Unlock()
	
	var p *peer
	var ok bool

	if p, ok = n.viewMap[key]; !ok {
		return peer{}, errPeerNotFound
	}

	return *p, nil
}


