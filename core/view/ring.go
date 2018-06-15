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

	selfIdx int
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

	if self == nil {
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

func (rs *rings) add(p *Peer) {
	for _, ring := range rs.ringMap {
		ring.add(p)
	}
}

func (rs *rings) remove(p *Peer) {
	for _, r := range rs.ringMap {
		r.remove(p)
	}
}

func (rs rings) isPredecessor(id, toCheck string, ringNum uint32) bool {
	if ring, ok := rs.ringMap[ringNum]; !ok {
		log.Error("Invalid ring number", "ringnum", ringNum)
		return false
	} else {
		return ring.isPrev(id, toCheck)
	}
}

func (rs rings) findNeighbours(id string) []*Peer {
	ret := make([]*Peer, 0, rs.numRings*2)
	exists := make(map[string]bool)

	for _, r := range rs.ringMap {
		succ, prev := r.neighbours(id)

		if succ != nil {
			if _, ok := exists[succ.Id]; !ok {
				ret = append(ret, succ)
			}
		}

		if prev != nil {
			if _, ok := exists[prev.Id]; !ok {
				ret = append(ret, prev)
			}
		}
	}

	return ret
}

func (rs rings) allMyNeighbours() []*Peer {
	ret := make([]*Peer, 0, rs.numRings*2)
	exists := make(map[string]bool)

	for _, r := range rs.ringMap {
		succ := r.successor()
		prev := r.predecessor()

		if _, ok := exists[succ.Id]; !ok {
			ret = append(ret, succ)
		}

		if _, ok := exists[prev.Id]; !ok {
			ret = append(ret, prev)
		}
	}

	return ret
}

func (rs rings) myRingNeighbours(ringNum uint32) []*Peer {
	r, ok := rs.ringMap[ringNum]
	if !ok {
		log.Error(errInvalidRingNum.Error())
		return nil
	}

	ret := make([]*Peer, 0, 2)

	succ := r.successor()
	prev := r.predecessor()

	ret = append(ret, succ)

	if prev.Id != succ.Id {
		ret = append(ret, prev)
	}

	return ret
}

func (rs rings) myRingSuccessor(ringNum uint32) *Peer {
	r, ok := rs.ringMap[ringNum]
	if !ok {
		log.Error(errInvalidRingNum.Error())
		return nil
	}

	return r.successor()
}

func (rs rings) shouldBeMyNeighbour(id string) bool {
	rId := []byte(id)

	for _, r := range rs.ringMap {
		if isNeighbour := r.betweenNeighbours(rId); isNeighbour {
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
	}

	return r
}

func (r *ring) add(p *Peer) {
	var idx int
	hash := hashId(r.ringNum, []byte(p.Id))

	id := &ringId{
		hash: hash,
		p:    p,
	}

	r.succList, idx = insert(r.succList, id)
	if idx <= r.selfIdx {
		r.selfIdx += 1
	}

	if eq := r.succList[r.selfIdx].equal(r.selfId); !eq {
		log.Error(errLostSelf.Error())
	}

	r.peerToRing[p.Id] = id

	//TODO should check if i get a new neighbor, if so, add possibility
	//to remove rpc connection of old neighbor.

	len := len(r.succList)

	succIdx := (idx + 1) % len

	prevIdx := (idx - 1) % len
	if prevIdx < 0 {
		prevIdx = prevIdx + len
	}

	succ := r.succList[succIdx].p
	prev := r.succList[prevIdx].p

	acc := succ.RingAccusation(r.ringNum)
	if acc != nil && acc.IsAccuser(prev.Id) {
		succ.RemoveRingAccusation(r.ringNum)
	}
}

func (r *ring) remove(p *Peer) {
	ringId := r.peerToRing[p.Id]

	idx, err := search(r.succList, ringId)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if idx < r.selfIdx {
		r.selfIdx -= 1
	} else if idx == r.selfIdx {
		log.Error(errRemoveSelf.Error())
		return
	}

	r.succList = append(r.succList[:idx], r.succList[idx+1:]...)

	delete(r.peerToRing, p.Id)

	if eq := r.succList[r.selfIdx].equal(r.selfId); !eq {
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
		log.Error(errIdNotFound.Error())
		return false
	}

	len := len(r.succList)

	idx := (i - 1) % len
	if idx < 0 {
		idx = idx + len
	}

	if eq := r.succList[idx].equal(toCheckId); eq {
		return true
	}

	return false
}

func (r *ring) betweenNeighbours(other []byte) bool {
	var id *ringId

	exists, ok := r.peerToRing[string(other)]
	if !ok {
		// Do not have peer representation
		// only need hash to perform search.
		id = &ringId{
			hash: hashId(r.ringNum, other),
		}
	} else {
		id = exists
	}

	len := len(r.succList)

	if len <= 1 {
		return true
	}

	idx := (r.selfIdx + 1) % len
	succ := r.succList[idx]

	prevIdx := (r.selfIdx - 1) % len
	if prevIdx < 0 {
		prevIdx = prevIdx + len
	}
	prev := r.succList[prevIdx]

	if !isBetween(r.selfId, succ, id) && !isBetween(prev, r.selfId, id) {
		return false
	} else {
		return true
	}
}

func (r *ring) neighbours(id string) (*Peer, *Peer) {
	len := len(r.succList)

	if len <= 1 {
		return r.selfId.p, r.selfId.p
	}

	ringId := r.peerToRing[id]

	idx, err := search(r.succList, ringId)
	if err != nil {
		log.Error(errRingIdNotFound.Error())
		return nil, nil
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

func (r *ring) successor() *Peer {
	idx := (r.selfIdx + 1) % len(r.succList)

	return r.succList[idx].p
}

func (r *ring) predecessor() *Peer {
	len := len(r.succList)

	idx := (r.selfIdx - 1) % len
	if idx < 0 {
		idx = idx + len
	}
	return r.succList[idx].p
}

func hashId(ringNum uint32, id []byte) []byte {
	preHashId := append(id, []byte(fmt.Sprintf("%d", ringNum))...)

	h := sha256.New()
	h.Write(preHashId)

	return h.Sum(nil)
}
