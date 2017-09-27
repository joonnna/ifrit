package node

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"sync"
)

var (
	errSameId        = errors.New("Nodes have identical id!?!?!?")
	errAlreadyExists = errors.New("Node already exists")
	errRemoveSelf    = errors.New("Tried to remove myself from ring?!")
	errLostSelf      = errors.New("Lost track of myself within ring")
)

type ring struct {
	ringNum uint8

	existsMap map[string]bool

	succList  []*ringId
	ringMutex sync.RWMutex
	ownIdx    int

	localRingId *ringId
}

type ringId struct {
	nodeId string
	id     []byte
}

func (rI *ringId) isSameAddr(target *ringId) bool {
	return rI.nodeId == target.nodeId
}

func (rI *ringId) cmpId(other *ringId) int {
	return bytes.Compare(rI.id, other.id)
}

func newRing(ringNum uint8, id []byte, addr string) *ring {
	rId := genRingId(ringNum, id)
	localRingId := &ringId{
		id:     rId,
		nodeId: addr,
	}

	r := &ring{
		ringNum:     ringNum,
		localRingId: localRingId,
		existsMap:   make(map[string]bool),
		succList:    []*ringId{localRingId},
	}
	return r
}

func (r *ring) add(newId []byte, addr string) error {
	r.ringMutex.Lock()
	defer r.ringMutex.Unlock()

	if ok, _ := r.existsMap[addr]; ok {
		return errAlreadyExists
	}

	id := genRingId(r.ringNum, newId)
	ringIdentity := newRingId(id, addr)

	cmp := ringIdentity.cmpId(r.localRingId)

	if cmp == 0 {
		return errSameId
	}

	newList, idx := insert(r.succList, ringIdentity)

	r.succList = newList

	if idx <= r.ownIdx {
		r.ownIdx += 1
	}

	r.existsMap[addr] = true

	if !r.succList[r.ownIdx].isSameAddr(r.localRingId) {
		return errLostSelf
	}

	return nil
}

func (r *ring) remove(removeId []byte, addr string) error {
	r.ringMutex.Lock()
	defer r.ringMutex.Unlock()

	id := newRingId(genRingId(r.ringNum, removeId), addr)

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

func genRingId(ringNum uint8, id []byte) []byte {
	preHashId := append(id, ringNum)

	h := sha256.New()
	h.Write([]byte(preHashId))

	return h.Sum(nil)
}

func newRingId(id []byte, nodeId string) *ringId {
	return &ringId{
		nodeId: nodeId,
		id:     id,
	}
}
