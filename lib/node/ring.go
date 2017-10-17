package node

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
)

var (
	errSameId             = errors.New("Nodes have identical id!?!?!?")
	errAlreadyExists      = errors.New("Node already exists")
	errRemoveSelf         = errors.New("Tried to remove myself from ring?!")
	errLostSelf           = errors.New("Lost track of myself within ring")
	errRingMemberNotFound = errors.New("Ring member not found")
)

type ring struct {
	ringNum uint32

	existsMap map[string]bool

	succList  []*ringId
	ringMutex sync.RWMutex

	ownIdx      int
	localRingId *ringId
}

type ringId struct {
	peerKey string
	id      []byte
	addr    string
}

func (rI *ringId) equal(target *ringId) bool {
	return rI.peerKey == target.peerKey
}

func (rI *ringId) cmpId(other *ringId) int {
	return bytes.Compare(rI.id, other.id)
}

func newRing(ringNum uint32, id []byte, peerKey string, addr string) *ring {
	h := hashId(ringNum, id)
	localRingId := &ringId{
		id:      h,
		peerKey: peerKey,
		addr:    addr,
	}

	r := &ring{
		ringNum:     ringNum,
		localRingId: localRingId,
		existsMap:   make(map[string]bool),
		succList:    []*ringId{localRingId},
	}
	return r
}

func (r *ring) add(newId []byte, peerKey string, addr string) error {
	r.ringMutex.Lock()
	defer r.ringMutex.Unlock()

	if ok, _ := r.existsMap[peerKey]; ok {
		return errAlreadyExists
	}

	h := hashId(r.ringNum, newId)
	id := &ringId{
		id:      h,
		peerKey: peerKey,
		addr:    addr,
	}

	cmp := id.cmpId(r.localRingId)
	if cmp == 0 {
		return errSameId
	}

	newList, idx := insert(r.succList, id)
	r.succList = newList

	if idx <= r.ownIdx {
		r.ownIdx += 1
	}

	r.existsMap[peerKey] = true

	if !r.succList[r.ownIdx].equal(r.localRingId) {
		return errLostSelf
	}

	return nil
}

func (r *ring) remove(removeId []byte, peerKey string) error {
	r.ringMutex.Lock()
	defer r.ringMutex.Unlock()

	if _, ok := r.existsMap[peerKey]; !ok {
		return errRingMemberNotFound
	}

	h := hashId(r.ringNum, removeId)
	//Dont need addr, just want ringId struct to perform search
	id := &ringId{
		id:      h,
		peerKey: peerKey,
	}

	idx, err := search(r.succList, id)
	if err != nil {
		return err
	}

	if idx < r.ownIdx {
		r.ownIdx -= 1
	} else if idx == r.ownIdx {
		return errRemoveSelf
	}

	delete(r.existsMap, peerKey)

	r.succList = append(r.succList[:idx], r.succList[idx+1:]...)

	if !r.succList[r.ownIdx].equal(r.localRingId) {
		return errLostSelf
	}

	return nil
}

func (r *ring) isPrev(p, other *peer) (bool, error) {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	len := len(r.succList)

	if len <= 1 && other.key == r.localRingId.peerKey {
		return true, nil
	}

	h := hashId(r.ringNum, p.id)
	//Dont need addr, just want ringId struct to perform search
	id := &ringId{
		id:      h,
		peerKey: p.key,
	}

	i, err := search(r.succList, id)
	if err != nil {
		return false, errNotFound
	}

	idx := (i + 1) % len
	if r.succList[idx].peerKey == other.key {
		return true, nil
	}

	return false, nil
}

func (r *ring) betweenNeighbours(other *peerId) bool {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	len := len(r.succList)

	if len <= 1 {
		return true
	}

	h := hashId(r.ringNum, other.id)
	id := &ringId{
		id:      h,
		peerKey: other.key,
	}

	idx := (r.ownIdx + 1) % len
	succ := r.succList[idx]

	prevIdx := (r.ownIdx - 1) % len
	if prevIdx < 0 {
		prevIdx = prevIdx + len
	}
	prev := r.succList[prevIdx]

	if !isBetween(r.localRingId, succ, id) && !isBetween(prev, r.localRingId, id) {
		return false
	} else {
		return true
	}
}

func (r *ring) findNeighbours(p *peerId) (string, string, error) {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	len := len(r.succList)

	if len <= 1 {
		return r.localRingId.peerKey, r.localRingId.peerKey, nil
	}

	h := hashId(r.ringNum, p.id)
	id := &ringId{
		id:      h,
		peerKey: p.key,
	}

	idx, err := search(r.succList, id)
	if err != nil {
		return "", "", errNotFound
	}

	prevIdx := (idx - 1) % len
	if prevIdx < 0 {
		prevIdx = prevIdx + len
	}
	prev := r.succList[prevIdx].peerKey

	succIdx := (idx + 1) % len
	succ := r.succList[succIdx].peerKey

	return succ, prev, nil
}

func (r *ring) findPrev(p *peerId) (string, error) {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	len := len(r.succList)

	if len <= 1 {
		return r.localRingId.peerKey, nil
	}

	h := hashId(r.ringNum, p.id)
	id := &ringId{
		id:      h,
		peerKey: p.key,
	}

	i, err := search(r.succList, id)
	if err != nil {
		return "", errNotFound
	}

	idx := (i - 1) % len
	if idx < 0 {
		idx = idx + len
	}

	return r.succList[idx].peerKey, nil
}

func isBetween(start, end, new *ringId) bool {
	startEndCmp := start.cmpId(end)
	startNewCmp := start.cmpId(new)
	endNewCmp := end.cmpId(new)

	if endNewCmp == 0 || startNewCmp == 0 {
		return true
	}

	//Start has lower id value
	if startEndCmp == -1 {
		if startNewCmp == -1 && endNewCmp == 1 {
			return true
		} else {
			return false
		}
		//Start has higher id value
	} else if startEndCmp == 1 {
		if startNewCmp == 1 && endNewCmp == 1 {
			return true
		} else if startNewCmp == -1 && endNewCmp == -1 {
			return true
		} else {
			return false
		}
	} else {
		return true
	}
}

func (r *ring) getRingList() []*ringId {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	ret := make([]*ringId, len(r.succList))
	copy(ret, r.succList)
	return ret
}

func (r *ring) getRingSucc() (*ringId, error) {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	len := len(r.succList)

	if len == 0 {
		return nil, errNotFound
	} else {
		idx := (r.ownIdx + 1) % len
		return r.succList[idx], nil
	}
}

func (r *ring) getRingPrev() (*ringId, error) {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	len := len(r.succList)

	if len == 0 {
		return nil, errNotFound
	} else {
		idx := (r.ownIdx - 1) % len
		if idx < 0 {
			idx = idx + len
		}
		return r.succList[idx], nil
	}
}

//Used for testing
func (r *ring) getSucc(id []byte) (string, error) {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	h := hashId(r.ringNum, id)
	//Dont need addr, just want ringId struct to perform search
	rId := &ringId{
		id: h,
	}

	i, err := search(r.succList, rId)
	if err != nil {
		return "", errNotFound
	}

	len := len(r.succList)

	idx := (i + 1) % len
	return r.succList[idx].peerKey, nil
}

func hashId(ringNum uint32, id []byte) []byte {
	preHashId := append(id, []byte(fmt.Sprintf("%d", ringNum))...)

	h := sha256.New()
	h.Write(preHashId)

	return h.Sum(nil)
}
