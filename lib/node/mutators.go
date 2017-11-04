package node

import (
	"errors"
)

var (
	errNotFound           = errors.New("No node info found")
	errPeerNotFound       = errors.New("No peer info found")
	errAlreadyDeactivated = errors.New("Ring was already deactivated")
)

func (n *Node) setEpoch(newEpoch uint64) {
	n.noteMutex.Lock()
	defer n.noteMutex.Unlock()

	n.recentNote.epoch = newEpoch

	err := n.recentNote.sign(n.privKey)
	if err != nil {
		n.log.Err.Println(err)
	}
}

func (n *Node) getEpoch() uint64 {
	n.noteMutex.RLock()
	defer n.noteMutex.RUnlock()

	return n.recentNote.epoch
}

func (n *Node) deactivateRing(idx uint32) {
	n.noteMutex.Lock()
	defer n.noteMutex.Unlock()

	if n.maxByz == 0 {
		return
	}

	ringNum := idx - 1

	len := uint32(len(n.recentNote.mask))

	maxIdx := len - 1

	if ringNum > maxIdx || ringNum < 0 {
		n.log.Err.Println(errNonExistingRing)
		return
	}

	if n.recentNote.mask[ringNum] == 0 {
		n.log.Err.Println(errAlreadyDeactivated)
		return
	}

	if n.deactivatedRings == n.maxByz {
		var idx uint32
		for idx = 0; idx < len; idx++ {
			if idx != ringNum && n.recentNote.mask[idx] == 0 {
				break
			}
		}
		n.recentNote.mask[idx] = 1
	} else {
		n.deactivatedRings++
	}

	n.recentNote.mask[ringNum] = 0

	err := n.recentNote.sign(n.privKey)
	if err != nil {
		n.log.Err.Println(err)
	}
}

func (n *Node) getMask() []byte {
	n.noteMutex.RLock()
	defer n.noteMutex.RUnlock()

	ret := make([]byte, len(n.recentNote.mask))

	copy(ret, n.recentNote.mask)

	return ret
}
