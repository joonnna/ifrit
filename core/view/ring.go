package view

import (
	"crypto/sha256"
	"errors"
	"fmt"

	log "github.com/inconshreveable/log15"
)

var (
	errSameId             = errors.New("Nodes have identical id!?!?!?")
	errAlreadyExists      = errors.New("Node already exists")
	errRemoveSelf         = errors.New("Tried to remove myself from ring?!")
	errLostSelf           = errors.New("Lost track of myself within ring")
	errRingMemberNotFound = errors.New("Ring member not found")
	errInvalidRingNum     = errors.New("Invalid ring number")
	errNoSelf             = errors.New("No self id provided")
	errNoRings            = errors.New("Numrings cant be 0")
)

type rings struct {
	ringMap  map[uint32]*ring
	numRings uint32

	self *Peer
}

type ring struct {
	ringNum uint32

	peerToRingId map[string]*ringId

	succList []*ringId

	selfIdx  int
	selfHash []byte
}

type ringId struct {
	p  *Peer
	id []byte
}

func createRings(self *Peer, numRings uint32) (*rings, error) {
	var i uint32

	if self == nil {
		return errNoSelf, nil
	}

	if numRings == 0 {
		return errNoRings, nil
	}

	rs := &rings{
		ringMap:  make(map[uint32]*ring),
		numRings: numRings,
		self:     self,
	}

	for i = 1; i <= rs.numRings; i++ {
		rs.ringMap[i] = newRing(i, self)
	}

	return r, nil
}

func (rs *rings) add(p *Peer) {
	for num, ring := range rs.ringMap {
		ring.add(p)
	}
}

func (rs *rings) remove(p *Peer) {
	for _, r := range rs.ringMap {
		r.remove(p)
	}
}

func (rs rings) isPrev(id, toCheck string, ringNum uint32) bool {
	if ring, ok := rs.ringMap[ringNum]; !ok {
		log.Error("Invalid ring number", "ringnum", ringNum)
		return false
	} else {
		return ring.isPrev(id, toCheck)
	}
}

func (rs rings) findNeighbours(id string) []*Peer {
	ret := make([]string, 0, rs.numRings*2)
	exists := make(map[string]bool)

	for num, r := range rs.ringMap {
		succ, prev := r.neighbours(id)

		if _, ok := exists[succ.Id]; !ok {
			ret = append(ret, succ)
		}

		if _, ok := exists[prev.Id]; !ok {
			ret = append(ret, prev)
		}
	}

	return ret
}

func newRing(ringNum uint32, self *Peer) *ring {
	id := &ringId{
		p:    self,
		hash: hashId(ringNum, []byte(self.Id)),
	}

	r := &ring{
		ringNum:    ringNum,
		self:       self,
		succList:   make([]*ringId{id}),
		selfHash:   h,
		peerToRing: make(map[string]*ringId),
	}

	peerToRing[self.Id] = id

	return r
}

func (r *ring) add(p *Peer) {
	var idx int
	hash := hashId(r.ringNum, p.Id)

	id := &ringId{
		hash: hash,
		p:    p,
	}

	r.succList, idx = insert(r.succList, id)
	if idx <= r.ownIdx {
		r.ownIdx += 1
	}

	r.peerToRing[p.Id] = id

	if eq := r.succList[r.ownIdx].bytes.Equal(r.selfHash); eq != 0 {
		log.Error(errLostSelf.Error())
	}

	//TODO should check if i get a new neighbor, if so, add possibility
	//to remove rpc connection of old neighbor.

	len := len(r.succList)

	succIdx := (idx + 1) % len

	prevIdx := (idx - 1) % len
	if prevIdx < 0 {
		prevIdx = prevIdx + len
	}

	succ := r.succList[succIdx]
	prev := r.succList[prevIdx]

	acc := succ.RingAccusation(r.ringNum)
	if acc != nil && acc.IsAccuser(prev) {
		succ.RemoveAccusation(r.ringNum)
	}
}

func (r *ring) remove(p *Peer) {
	ringId := r.peerToRing[p.Id]

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

	r.succList = append(r.succList[:idx], r.succList[idx+1:]...)

	delete(r.peerToRing, p.Id)

	if eq := r.succList[r.ownIdx].bytes.Equal(r.selfHash); eq != 0 {
		log.Error(errLostSelf.Error())
	}
}

func (r *ring) isPrev(id, toCheck string) bool {
	toCheckId, ok := r.peerToRing[toCheck]
	if !ok {
		return false
	}

	currId, ok := r.peerToRing[id]
	if !ok {
		return false
	}

	i, err := search(r.succList, currId)
	if err != nil {
		log.Error(errNotFound.Error())
		return false
	}

	len := len(r.succList)

	idx := (i - 1) % len
	if idx < 0 {
		idx = idx + len
	}

	if eq := r.succList[idx].equal(checkId); eq {
		return true
	}

	return false
}

func (r *ring) betweenNeighbours(other []byte) bool {
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

	if !isBetween(r.selfHash, succ, other) && !isBetween(prev, r.selfHash, other) {
		return false
	} else {
		return true
	}
}

func (r *ring) neighbors(id []byte) (*Peer, *Peer) {
	len := len(r.succList)

	if len <= 1 {
		return r.self.Id, r.self.Id
	}

	idx, err := search(r.succList, id)
	if err != nil {
		log.Error(errNotFound.Error())
		return "", ""
	}

	prevIdx := (idx - 1) % len
	if prevIdx < 0 {
		prevIdx = prevIdx + len
	}
	prev := r.succList[prevIdx].p

	succIdx := (idx + 1) % len
	succ := r.succList[succIdx].p

	return succ, prev
}

func isBetween(start, end, new *ringId) bool {
	startEndCmp := start.bytes.Compare(end)
	startNewCmp := start.bytes.Compare(new)
	endNewCmp := end.bytes.Compare(new)

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
	idx := (r.ownIdx + 1) % len(r.succList)

	return string(r.succList[idx])
}

func (r *ring) predecessor() string {
	len := len(r.succList)

	idx := (r.ownIdx - 1) % len
	if idx < 0 {
		idx = idx + len
	}
	return string(r.succList[idx])
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
