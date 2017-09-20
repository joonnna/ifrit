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
	errRemoveSelf = errors.New("Tried to remove myself from ring?!")
	errLostSelf = errors.New("Lost track of myself within ring")
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

func (i *ringId) isSameAddr(target *ringId) bool {
	return i.nodeId == target.nodeId
}


func (i *ringId) cmpId(target *ringId) int {
	return i.id.Cmp(target.id)
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

	if !r.succList[r.ownIdx].isSameAddr(r.localRingId) {
		return errLostSelf
	}

	return nil
}

func (r *ring) remove (addr string) error {
	r.ringMutex.Lock()
	defer r.ringMutex.Unlock()

	id := newRingId(genRingId(r.ringNum, addr), addr)

	idx, err := search(r.succList, id)
	if err != nil {
		return err
	}
	
	r.existsMap[addr] = false
	
	if idx < r.ownIdx {
		r.ownIdx -= 1
	} else if idx == r.ownIdx {
		return errRemoveSelf
	}
	
	r.succList = append(r.succList[:idx], r.succList[idx+1:]...)
	
	if !r.succList[r.ownIdx].isSameAddr(r.localRingId) {
		return errLostSelf
	}

	return nil
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
