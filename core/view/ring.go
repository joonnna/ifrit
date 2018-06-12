package core

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"

	log "github.com/inconshreveable/log15"
)

var (
	errSameId             = errors.New("Nodes have identical id!?!?!?")
	errAlreadyExists      = errors.New("Node already exists")
	errRemoveSelf         = errors.New("Tried to remove myself from ring?!")
	errLostSelf           = errors.New("Lost track of myself within ring")
	errRingMemberNotFound = errors.New("Ring member not found")
	errInvalidRingNum     = errors.New("0 is an invalid ring number")
)

type ring struct {
	sync.RWMutex

	ringNum uint32

	peerToRing map[string][]byte
	succList   [][]byte

	selfIdx int
	self    *Peer
}

func newRing(ringNum uint32, self *Peer) (*ring, error) {
	h := hashId(ringNum, peer.Id)

	r := &ring{
		ringNum:   ringNum,
		self:      self,
		existsMap: make(map[string]bool),
		succList:  [][]byte{h},
	}

	r.peerToRing[self.Id] = h

	return r, nil
}

func (r *ring) add(id string, addr string) {
	r.Lock()
	defer r.Unlock()

	if ringId, ok := r.peerToRing[id]; ok {
		log.Error(errAlreadyExists.Error())
		return
	}

	hash := hashId(id, r.ringNum)

	newList, idx := insert(r.succList, hash)
	r.succList = newList

	if idx <= r.ownIdx {
		r.ownIdx += 1
	}

	r.peerToRing[key] = hash

	selfHash := r.peerToRing[self.Id]
	if eq := r.succList[r.ownIdx].bytes.Equal(selfHash); eq != 0 {
		log.Error(errLostSelf.Error())
	}
}

func (r *ring) remove(id string) {
	r.Lock()
	defer r.Unlock()

	if ringId, ok := r.existsMap[id]; !ok {
		log.Error(errNotFound.Error())
		return
	}

	idx, err := search(r.succList, ringId)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if idx < r.ownIdx {
		r.ownIdx -= 1
	} else if idx == r.ownIdx {
		log.Error(errRemoveSelf.Error())
		return
	}

	delete(r.peerToRing, key)

	r.succList = append(r.succList[:idx], r.succList[idx+1:]...)

	selfHash := r.peerToRing[self.Id]
	if eq := r.succList[r.ownIdx].bytes.Equal(selfHash); eq != 0 {
		log.Error(errLostSelf.Error())
	}
}

func (r *ring) isPrev(id, toCheck []byte) bool {
	r.RLock()
	defer r.RUnlock()

	ringId, ok := r.existsMap[id]
	if !ok {
		log.Error(errNotFound.Error())
		return false
	}

	i, err := search(r.succList, ringId)
	if err != nil {
		log.Error(errNotFound.Error())
		return false
	}

	len := len(r.succList)

	idx := (i - 1) % len
	if idx < 0 {
		idx = idx + len
	}

	if eq := r.succList[idx].bytes.Equal(checkId); eq == 0 {
		return true
	}

	return false
}

func (r *ring) betweenNeighbours(other string) bool {
	r.RLock()
	defer r.RUnlock()

	ringId, ok := r.peerToRing[other]
	if !ok {
		log.Error(errNotFound.Error())
		return
	}

	len := len(r.succList)

	if len <= 1 {
		return true
	}

	idx := (r.ownIdx + 1) % len
	succ := r.succList[idx]

	prevIdx := (r.ownIdx - 1) % len
	if prevIdx < 0 {
		prevIdx = prevIdx + len
	}
	prev := r.succList[prevIdx]

	selfHash := r.peerToRing[self.Id]

	if !isBetween(selfHash, succ, ringId) && !isBetween(prev, selfHash, ringId) {
		return false
	} else {
		return true
	}
}

func (r *ring) findNeighbours(id string) (string, string) {
	r.RLock()
	defer r.RUnlock()

	len := len(r.succList)

	if len <= 1 {
		return r.self.Id, r.self.Id
	}

	ringId, ok := r.peerToRing[other]
	if !ok {
		log.Error(errNotFound.Error())
		return
	}

	idx, err := search(r.succList, ringId)
	if err != nil {
		log.Error(errNotFound.Error())
		return "", ""
	}

	prevIdx := (idx - 1) % len
	if prevIdx < 0 {
		prevIdx = prevIdx + len
	}
	prev := r.succList[prevIdx].key

	succIdx := (idx + 1) % len
	succ := r.succList[succIdx].key

	return succ, prev
}

func isBetween(start, end, new []byte) bool {
	startEndCmp := start.bytes.Equal(end)
	startNewCmp := start.bytes.Equal(new)
	endNewCmp := end.bytes.Equal(new)

	if endNewCmp == 0 || startNewCmp == 0 {
		return true
	}

	// Start has lower id value
	if startEndCmp == -1 {
		if startNewCmp == -1 && endNewCmp == 1 {
			return true
		} else {
			return false
		}
		// Start has higher id value
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

func (r *ring) successor() string {
	r.RLock()
	defer r.RUnlock()

	idx := (r.ownIdx + 1) % len(r.succList)

	return r.succList[idx]
}

func (r *ring) getRingPrev() (*ringId, error) {
	r.RLock()
	defer r.RUnlock()

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

func hashId(ringNum uint32, id []byte) []byte {
	preHashId := append(id, []byte(fmt.Sprintf("%d", ringNum))...)

	h := sha256.New()
	h.Write(preHashId)

	return h.Sum(nil)
}

/*

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
	return r.succList[idx].key, nil
}


func (r *ring) getRingList() []*ringId {
	r.ringMutex.RLock()
	defer r.ringMutex.RUnlock()

	ret := make([]*ringId, len(r.succList))
	copy(ret, r.succList)
	return ret
}


//Only used for internal testing
func (r *ring) findPrev(id []byte) (string, error) {
	r.RLock()
	defer r.RUnlock()

	h := hashId(r.ringNum, id)
	rId := &ringId{
		id: h,
	}

	i, err := search(r.succList, rId)
	if err != nil {
		return "", errNotFound
	}

	len := len(r.succList)

	idx := (i - 1) % len
	if idx < 0 {
		idx = idx + len
	}

	return r.succList[idx].key, nil
}


*/
