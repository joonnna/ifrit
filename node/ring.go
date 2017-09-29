package node

import (
	"bytes"
	"crypto/sha256"
	"errors"
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
	ringNum uint8

	existsMap map[string]bool

	succList  []*ringId
	ringMutex sync.RWMutex
	ownIdx    int

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

func newRing(ringNum uint8, id []byte, peerKey string, addr string) *ring {
	h := hashId(ringNum, id)
	localRingId := newRingId(h, peerKey, addr)

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
	ringIdentity := newRingId(h, peerKey, addr)

	cmp := ringIdentity.cmpId(r.localRingId)
	if cmp == 0 {
		return errSameId
	}

	newList, idx := insert(r.succList, ringIdentity)
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
	id := newRingId(h, peerKey, "")

	idx, err := search(r.succList, id)
	if err != nil {
		return err
	}

	if idx < r.ownIdx {
		r.ownIdx -= 1
	} else if idx == r.ownIdx {
		return errRemoveSelf
	}

	r.existsMap[peerKey] = false
	r.succList = append(r.succList[:idx], r.succList[idx+1:]...)

	if !r.succList[r.ownIdx].equal(r.localRingId) {
		return errLostSelf
	}

	return nil
}

func hashId(ringNum uint8, id []byte) []byte {
	preHashId := append(id, ringNum)

	h := sha256.New()
	h.Write([]byte(preHashId))

	return h.Sum(nil)
}

func newRingId(id []byte, peerKey string, addr string) *ringId {
	return &ringId{
		peerKey: peerKey,
		id:      id,
		addr:    addr,
	}
}
