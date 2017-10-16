package node

import (
	"errors"
)

var (
	errNotFound     = errors.New("No node info found")
	errPeerNotFound = errors.New("No peer info found")
)

func (n *Node) setEpoch(newEpoch uint64) {
	n.noteMutex.Lock()
	defer n.noteMutex.Unlock()

	n.recentNote.epoch = newEpoch
}

func (n *Node) getEpoch() uint64 {
	n.noteMutex.RLock()
	defer n.noteMutex.RUnlock()

	return n.recentNote.epoch
}

func (n *Node) deactivateRing(ringNum uint32) {
	n.noteMutex.Lock()
	defer n.noteMutex.Unlock()

	if ringNum >= uint32(len(n.recentNote.mask)) {
		n.log.Err.Println("Tried to deactivate non existing ring!")
		return
	}

	n.recentNote.mask[ringNum] = 0
}

func (n *Node) getMask() []byte {
	n.noteMutex.RLock()
	defer n.noteMutex.RUnlock()

	ret := make([]byte, len(n.recentNote.mask))

	copy(ret, n.recentNote.mask)

	return ret
}
