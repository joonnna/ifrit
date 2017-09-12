package node

import (
	"math/big"
	"sync"
	"errors"
	"fmt"
	"crypto/sha256"
)

var (
	errSameId = errors.New("Nodes have identical id!?!?!?")
	errAlreadyExists = errors.New("Node already exists")
)

type ring struct {
	ringNum uint8
	succList []*ringId
	ringMutex sync.RWMutex
	
	existsMap map[string]bool

	ownIdx int

	localRingId *ringId
}


type ringId struct {
	nodeId string
	id *big.Int
}

func newRing(ringNum uint8, localId string) *ring {
	id := genRingId(ringNum, localId)
	localRingId := &ringId{
		id: id, 
		nodeId: localId,
	}

	r := &ring {
		ringNum: ringNum,
		localRingId: localRingId,
		existsMap: make(map[string]bool),
		succList: make([]*ringId, 0),
	}
	r.succList = append(r.succList, localRingId)
	return r
}

func (r *ring) add (newId string) error {
	r.ringMutex.Lock()
	defer r.ringMutex.Unlock()

	if ok,_ := r.existsMap[newId]; ok {
		return errAlreadyExists
	}

	id := genRingId(r.ringNum, newId)

	cmp := id.Cmp(r.localRingId.id)

	if cmp == 0 {
		return errSameId
	}

	ringIdentity := newRingId(id, newId)
	newList, idx := insert(r.succList, ringIdentity)

	r.succList = newList

	if idx <= r.ownIdx {
		r.ownIdx += 1
	}

	r.existsMap[newId] = true

	return nil
}

func (r *ring) remove (newId string) {
	r.ringMutex.Lock()
	defer r.ringMutex.Unlock()
}


func genRingId (ringNum uint8, id string) *big.Int {
	var hashVal big.Int

	preHashId := fmt.Sprintf("%s%d", id, ringNum)

	h := sha256.New()
	h.Write([]byte(preHashId))

	hashVal.SetBytes(h.Sum(nil))

	return &hashVal
}

func newRingId (id *big.Int, nodeId string) *ringId {
	return &ringId {
		nodeId: nodeId,
		id: id,
	}
}
/*
func isBetween(currNext *big.Int, newId *big.Int, localId *big.Int) bool {
	if currNext.Cmp(localId) == 1 {
		if newId.Cmp(currNext) == -1 && newId.Cmp(localId) == 1 {
			return true
		} else {
			return false
		}
	} else if currNext.Cmp(localId) == -1 {
		if newId.Cmp(currNext) == 1 && newId.Cmp(localId) == -1 {
			return true
		} else {
			return false
		}
	}
	return false
}

func (r *ring) checkHighestLowest(newId *ringId) {
	if r.highestId.id.Cmp(newId.id) == -1 {
		r.highestId = newId
	} else if r.lowestId.id.Cmp(newId.id) == 1 {
		r.lowestId = newId
	}
}

func (r *ring) isLowest(id *big.Int) bool {
	if r.lowestId.id.Cmp(id) == 1 {
		return true
	} else {
		return false
	}
}


func (r *ring) isHighest(id *big.Int) bool {
	if r.highestId.id.Cmp(id) == -1 {
		return true
	} else {
		return false
	}
}
*/
