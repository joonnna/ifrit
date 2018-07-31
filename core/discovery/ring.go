package discovery

import (
	"bytes"
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
	errRingIdNotFound     = errors.New("Peer with the provided id not found")
)

type rings struct {
	ringMap  map[uint32]*ring
	numRings uint32

	self *Peer
}

type ring struct {
	ringNum uint32

	peerToRing map[string]*ringId

	succList []*ringId
	length   int

	selfIdx uint32
	selfId  *ringId
}

type ringId struct {
	p    *Peer
	hash []byte
}

func (ri ringId) compare(other *ringId) int {
	return bytes.Compare(ri.hash, other.hash)
}

func (ri ringId) equal(other *ringId) bool {
	return bytes.Equal(ri.hash, other.hash)
}

func createRings(self *Peer, numRings uint32) (*rings, error) {
	var i uint32

	if self == nil || self.Id == "" {
		return nil, errNoSelf
	}

	if numRings == 0 {
		return nil, errNoRings
	}

	rs := &rings{
		ringMap:  make(map[uint32]*ring),
		numRings: numRings,
		self:     self,
	}

	for i = 1; i <= rs.numRings; i++ {
		rs.ringMap[i] = newRing(i, self)
	}

	return rs, nil
}

func (rs *rings) add(p *Peer) []string {
	oldNeighbours := make([]string, 0)

	for _, ring := range rs.ringMap {
		old := ring.add(p)
		if old != nil {
			oldNeighbours = append(oldNeighbours, old...)
		}
	}

	return oldNeighbours
}

func (rs *rings) remove(p *Peer) {
	for _, r := range rs.ringMap {
		r.remove(p)
	}
}

func (rs *rings) isPredecessor(id, toCheck *Peer, ringNum uint32) bool {
	if ring, ok := rs.ringMap[ringNum]; !ok {
		log.Error("Invalid ring number", "ringNum", ringNum)
		return false
	} else {
		return ring.isPrev(id, toCheck)
	}
}

func (rs *rings) findNeighbours(id string) []*Peer {
	ret := make([]*Peer, 0, rs.numRings*2)
	exists := make(map[string]bool)

	for _, r := range rs.ringMap {
		succ, prev := r.neighbours(id)

		if _, ok := exists[succ.p.Id]; !ok {
			exists[succ.p.Id] = true
			ret = append(ret, succ.p)
		}

		if _, ok := exists[prev.p.Id]; !ok {
			exists[prev.p.Id] = true
			ret = append(ret, prev.p)
		}
	}

	return ret
}

func (rs *rings) allMyNeighbours() []*Peer {
	ret := make([]*Peer, 0, rs.numRings*2)
	exists := make(map[string]bool)

	for _, r := range rs.ringMap {
		succ := r.successor()
		prev := r.predecessor()

		if _, ok := exists[succ.p.Id]; !ok && succ.p.Id != rs.self.Id {
			exists[succ.p.Id] = true
			ret = append(ret, succ.p)
		}

		if _, ok := exists[prev.p.Id]; !ok && prev.p.Id != rs.self.Id {
			exists[prev.p.Id] = true
			ret = append(ret, prev.p)
		}
	}

	return ret
}

func (rs *rings) myRingNeighbours(ringNum uint32) []*Peer {
	r, ok := rs.ringMap[ringNum]
	if !ok {
		log.Error(errInvalidRingNum.Error())
		return nil
	}

	ret := make([]*Peer, 0, 2)

	succ := r.successor().p
	prev := r.predecessor().p

	if succ.Id != rs.self.Id {
		ret = append(ret, succ)
	}

	if prev.Id != succ.Id && prev.Id != rs.self.Id {
		ret = append(ret, prev)
	}

	return ret
}

func (rs *rings) myRingSuccessor(ringNum uint32) *Peer {
	r, ok := rs.ringMap[ringNum]
	if !ok {
		log.Error(errInvalidRingNum.Error())
		return nil
	}

	succ := r.successor().p

	if succ.Id == rs.self.Id {
		return nil
	} else {
		return succ
	}
}

func (rs *rings) myRingPredecessor(ringNum uint32) *Peer {
	r, ok := rs.ringMap[ringNum]
	if !ok {
		log.Error(errInvalidRingNum.Error())
		return nil
	}

	prev := r.predecessor().p

	if prev.Id == rs.self.Id {
		return nil
	} else {
		return prev
	}

}

func (rs *rings) shouldBeMyNeighbour(id string) bool {
	for _, r := range rs.ringMap {
		if isNeighbour := r.betweenNeighbours(id); isNeighbour {
			return true
		}
	}

	return false
}

func newRing(ringNum uint32, self *Peer) *ring {
	id := &ringId{
		p:    self,
		hash: hashId(ringNum, []byte(self.Id)),
	}

	r := &ring{
		ringNum:    ringNum,
		succList:   []*ringId{id},
		selfId:     id,
		peerToRing: make(map[string]*ringId),
		length:     1,
	}

	r.peerToRing[self.Id] = id

	return r
}

func (r *ring) add(p *Peer) []string {
	var idx int
	var oldNeighbours []string

	if _, ok := r.peerToRing[p.Id]; ok {
		log.Error("Peer already exists in ring", "ringNum", r.ringNum, "addr", p.Addr)
		return nil
	}

	hash := hashId(r.ringNum, []byte(p.Id))

	id := &ringId{
		hash: hash,
		p:    p,
	}

	oldSucc := r.successor()
	oldPrev := r.predecessor()

	r.succList, idx = insert(r.succList, id, r.length)
	if uint32(idx) <= r.selfIdx {
		r.selfIdx += 1
	}
	r.length++

	if new := r.successor(); !new.equal(oldSucc) {
		oldNeighbours = append(oldNeighbours, oldSucc.p.Addr)
	}

	if new := r.predecessor(); !new.equal(oldPrev) {
		oldNeighbours = append(oldNeighbours, oldPrev.p.Addr)
	}

	if eq := r.succList[r.selfIdx].equal(r.selfId); !eq {
		log.Error(errLostSelf.Error())
	}

	r.peerToRing[p.Id] = id

	succ := r.succAtIdx(idx).p
	prev := r.prevAtIdx(idx).p

	acc := succ.RingAccusation(r.ringNum)
	if acc != nil && acc.IsAccuser(prev.Id) {
		succ.RemoveRingAccusation(r.ringNum)
	}

	return oldNeighbours
}

func (r *ring) remove(p *Peer) {
	var rId *ringId
	var ok bool

	if rId, ok = r.peerToRing[p.Id]; !ok {
		log.Error("Peer does not exists in ring", "ringNum", r.ringNum, "addr", p.Addr)
		return
	}

	if r.length == 0 {
		log.Error("Successor list is already empty.")
		return
	}

	idx, err := search(r.succList, rId, r.length)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if uint32(idx) < r.selfIdx {
		r.selfIdx -= 1
	} else if uint32(idx) == r.selfIdx {
		log.Error(errRemoveSelf.Error())
		return
	}

	r.succList = append(r.succList[:idx], r.succList[idx+1:]...)
	r.length--

	delete(r.peerToRing, p.Id)

	if eq := r.succList[r.selfIdx].equal(r.selfId); !eq {
		log.Error(errLostSelf.Error())
	}
}

func (r *ring) isPrev(p, toCheck *Peer) bool {
	var deadAcc, deadAccuser bool

	toCheckId, ok := r.peerToRing[toCheck.Id]
	if !ok {
		deadAccuser = true
	}

	rId, ok := r.peerToRing[p.Id]
	if !ok {
		deadAcc = true
	}

	if !deadAcc && !deadAccuser {
		if !rId.equal(r.selfId) {
			i, err := search(r.succList, rId, r.length)
			if err != nil {
				log.Error(err.Error())
				return false
			}

			return r.prevAtIdx(i).equal(toCheckId)
		} else {
			return r.predecessor().equal(toCheckId)
		}
	} else if deadAcc && !deadAccuser {
		accId := &ringId{
			hash: hashId(r.ringNum, []byte(p.Id)),
		}

		// No need for self check, i will never consider myself dead.

		_, prev := findSuccAndPrev(r.succList, accId, r.length)

		return isBetween(prev, accId, toCheckId)

	} else if !deadAcc && deadAccuser {
		accuserId := &ringId{
			hash: hashId(r.ringNum, []byte(toCheck.Id)),
		}

		if !rId.equal(r.selfId) {
			_, prev := findSuccAndPrev(r.succList, rId, r.length)

			return isBetween(prev, rId, accuserId)
		} else {
			return isBetween(r.predecessor(), rId, accuserId)
		}
	} else {
		accId := &ringId{
			hash: hashId(r.ringNum, []byte(p.Id)),
		}

		accuserId := &ringId{
			hash: hashId(r.ringNum, []byte(toCheck.Id)),
		}

		_, prev := findSuccAndPrev(r.succList, accId, r.length)

		return isBetween(prev, accId, accuserId)
	}
}

func (r *ring) betweenNeighbours(other string) bool {
	var id *ringId
	var ok bool

	if r.length <= 1 {
		return true
	}

	if id, ok = r.peerToRing[other]; !ok {
		// Do not have peer representation
		// only need hash to perform search.
		id = &ringId{
			hash: hashId(r.ringNum, []byte(other)),
		}
	}

	if !isBetween(r.selfId, r.successor(), id) && !isBetween(r.predecessor(), r.selfId, id) {
		return false
	} else {
		return true
	}
}

func (r *ring) neighbours(id string) (*ringId, *ringId) {
	var ok bool
	var rId *ringId

	if r.length <= 1 {
		return r.selfId, r.selfId
	}

	if rId, ok = r.peerToRing[id]; !ok {
		// Do not have peer representation
		// only need hash to perform search.
		rId = &ringId{
			hash: hashId(r.ringNum, []byte(id)),
		}
	}

	return findSuccAndPrev(r.succList, rId, r.length)
}

func isBetween(start, end, new *ringId) bool {
	startEndCmp := start.compare(end)
	startNewCmp := start.compare(new)
	endNewCmp := end.compare(new)

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

func (r *ring) successor() *ringId {
	return succ(r.succList, int(r.selfIdx), r.length)
}

func (r *ring) predecessor() *ringId {
	return prev(r.succList, int(r.selfIdx), r.length)
}

func (r *ring) succAtIdx(idx int) *ringId {
	return succ(r.succList, idx, r.length)
}

func (r *ring) prevAtIdx(idx int) *ringId {
	return prev(r.succList, idx, r.length)
}

func hashId(ringNum uint32, id []byte) []byte {
	preHashId := append(id, []byte(fmt.Sprintf("%d", ringNum))...)

	h := sha256.New()
	h.Write(preHashId)

	return h.Sum(nil)
}
