package discovery

import (
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
	"math/bits"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/protobuf"
	"github.com/spf13/viper"
)

var (
	errInvalidNeighbours  = errors.New("Neighbours are nil ?!")
	errPeerAlreadyExists  = errors.New("Peer id already exists in the full view")
	errAlreadyDeactivated = errors.New("Ring was already deactivated")
	errZeroDeactivate     = errors.New("No ring can be deactivated, maxbyz is 0")
)

type View struct {
	viewMap   map[string]*Peer
	viewMutex sync.RWMutex

	liveMap   map[string]*Peer
	liveMutex sync.RWMutex

	timeoutMap   map[string]*timeout
	timeoutMutex sync.RWMutex

	rings *rings

	currGossipRing  uint32
	currMonitorRing uint32

	maxByz           uint32
	deactivatedRings uint32

	removalTimeout float64

	privKey *ecdsa.PrivateKey
	self    *Peer
}

func NewView(numRings uint32, cert *x509.Certificate, privKey *ecdsa.PrivateKey) (*View, error) {
	var i, mask uint32

	maxByz := (float64(numRings) / 2.0) - 1
	if maxByz < 0 {
		maxByz = 0
	}

	self, err := newPeer(cert, numRings)
	if err != nil {
		return nil, err
	}

	for i = 0; i < numRings; i++ {
		mask = setBit(mask, i)
	}

	localNote := &Note{
		epoch: 1,
		mask:  mask,
		id:    self.Id,
	}

	err = localNote.sign(privKey)
	if err != nil {
		return nil, err
	}

	self.note = localNote

	rings, err := createRings(self, numRings)
	if err != nil {
		return nil, err
	}

	v := &View{
		rings:           rings,
		viewMap:         make(map[string]*Peer),
		liveMap:         make(map[string]*Peer),
		timeoutMap:      make(map[string]*timeout),
		removalTimeout:  viper.GetFloat64("dead_timeout"),
		maxByz:          uint32(maxByz),
		currGossipRing:  1,
		currMonitorRing: 1,
		privKey:         privKey,
		self:            self,
	}

	return v, nil
}

func (v *View) NumRings() uint32 {
	return v.rings.numRings
}

func (v *View) Self() *Peer {
	return v.self
}

func (v *View) Peer(id string) *Peer {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	if p, ok := v.viewMap[id]; !ok {
		return nil
	} else {
		return p
	}
}

func (v *View) Full() []*Peer {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	ret := make([]*Peer, 0, len(v.viewMap))

	for _, p := range v.viewMap {
		ret = append(ret, p)
	}

	return ret
}

func (v *View) Exists(id string) bool {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	_, ok := v.viewMap[id]

	return ok
}

func (v *View) Live() []*Peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	ret := make([]*Peer, 0, len(v.liveMap))

	for _, p := range v.liveMap {
		ret = append(ret, p)
	}

	return ret

}

func (v *View) AddFull(id string, cert *x509.Certificate) error {
	v.viewMutex.Lock()
	defer v.viewMutex.Unlock()

	if _, ok := v.viewMap[id]; ok {
		log.Error("Tried to add peer twice to viewMap")
		return errPeerAlreadyExists
	}

	p, err := newPeer(cert, v.rings.numRings)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	v.viewMap[p.Id] = p

	return nil
}

func (v *View) MyNeighbours() []*Peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	return v.rings.allMyNeighbours()
}

func (v *View) GossipPartners() []*Peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	// Only one goroutine accesses gossip ring variable,
	// only take read lock for live view.
	defer v.incrementGossipRing()

	return v.rings.myRingNeighbours(v.currGossipRing)
}

func (v *View) MonitorTarget() (*Peer, uint32) {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	ringNum := v.currMonitorRing

	// Only one goroutine accesses monitor ring variable,
	// only take read lock for live view.
	defer v.incrementMonitorRing()

	return v.rings.myRingSuccessor(v.currMonitorRing), ringNum
}

func (v *View) AddLive(p *Peer) {
	v.liveMutex.Lock()
	defer v.liveMutex.Unlock()

	if _, ok := v.liveMap[p.Id]; ok {
		log.Error("Tried to add peer twice to liveMap", "addr", p.Addr)
	}

	v.liveMap[p.Id] = p

	v.rings.add(p)
}

func (v *View) RingNeighbours(ringNum uint32) (*Peer, *Peer) {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	return v.rings.myRingSuccessor(ringNum), v.rings.myRingPredecessor(ringNum)
}

func (v *View) LivePeer(id string) *Peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	return v.liveMap[id]
}

func (v *View) RemoveLive(id string) {
	v.liveMutex.Lock()
	defer v.liveMutex.Unlock()

	if peer, ok := v.liveMap[id]; ok {
		v.rings.remove(peer)

		delete(v.liveMap, peer.Id)

		log.Debug("Removed livePeer", "addr", peer.Addr)
	} else {
		log.Debug("Tried to remove non-existing peer from live view", "addr", peer.Addr)
	}
}

func (v *View) StartTimer(accused *Peer, n *Note, observer *Peer) {
	v.timeoutMutex.Lock()
	defer v.timeoutMutex.Unlock()

	if t, ok := v.timeoutMap[accused.Id]; ok && t.lastNote.epoch >= n.epoch {
		return
	}

	newTimeout := &timeout{
		observer:  observer,
		timeStamp: time.Now(),
		lastNote:  n,
		accused:   accused,
	}

	v.timeoutMap[accused.Id] = newTimeout

	log.Debug("Started timer for: %s", accused.Addr)
}

func (v *View) HasTimer(id string) bool {
	v.timeoutMutex.RLock()
	defer v.timeoutMutex.RUnlock()

	_, ok := v.timeoutMap[id]

	return ok
}

func (v *View) DeleteTimeout(id string) {
	v.timeoutMutex.Lock()
	defer v.timeoutMutex.Unlock()

	delete(v.timeoutMap, id)
}

func (v *View) allTimeouts() []*timeout {
	v.timeoutMutex.RLock()
	defer v.timeoutMutex.RUnlock()

	ret := make([]*timeout, 0, len(v.timeoutMap))

	for _, t := range v.timeoutMap {
		ret = append(ret, t)
	}

	return ret
}

func (v *View) CheckTimeouts() {
	timeouts := v.allTimeouts()
	if numTimeouts := len(timeouts); numTimeouts > 0 {
		log.Debug("Have timeouts", "amount", numTimeouts)
	}

	for _, t := range timeouts {
		if time.Since(t.timeStamp).Seconds() > v.removalTimeout {
			log.Debug("Timeout expired, removing from live", "addr", t.accused.Addr)
			v.DeleteTimeout(t.accused.Id)
			v.RemoveLive(t.accused.Id)
		}
	}
}

func (v *View) ShouldRebuttal(epoch uint64, ringNum uint32) bool {
	v.self.noteMutex.Lock()
	defer v.self.noteMutex.Unlock()

	if eq := v.self.note.SameEpoch(epoch); eq {
		newMask := v.self.note.mask

		mask, err := v.deactivateRing(ringNum)
		if err != nil {
			log.Error(err.Error())
		} else {
			newMask = mask
		}

		newNote := &Note{
			id:    v.self.Id,
			epoch: v.self.note.epoch + 1,
			mask:  newMask,
		}

		err = newNote.sign(v.privKey)
		if err != nil {
			log.Error(err.Error())
		}

		v.self.note = newNote

		return true
	} else {
		return false
	}
}

func (v *View) ShouldBeNeighbour(id string) bool {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	return v.rings.shouldBeMyNeighbour(id)
}

func (v *View) FindNeighbours(id string) []*Peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	return v.rings.findNeighbours(id)
}

func (v *View) ValidAccuser(accused, accuser string, ringNum uint32) bool {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	return v.rings.isPredecessor(accused, accuser, ringNum)
}

func (v *View) IsAlive(id string) bool {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	_, ok := v.liveMap[id]

	return ok
}

func (v *View) incrementGossipRing() {
	v.currGossipRing = ((v.currGossipRing + 1) % (v.rings.numRings + 1))
	if v.currGossipRing == 0 {
		v.currGossipRing = 1
	}
}

func (v *View) incrementMonitorRing() {
	v.currMonitorRing = ((v.currMonitorRing + 1) % (v.rings.numRings + 1))
	if v.currMonitorRing == 0 {
		v.currMonitorRing = 1
	}
}

func (v *View) selfNote() *gossip.Note {
	v.self.noteMutex.RLock()
	defer v.self.noteMutex.RUnlock()

	return v.self.note.ToPbMsg()
}

func (v *View) State() *gossip.State {
	ownNote := v.selfNote()

	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	ret := &gossip.State{
		ExistingHosts: make(map[string]uint64),
		OwnNote:       ownNote,
	}

	for _, p := range v.viewMap {
		if note := p.Note(); note != nil {
			ret.ExistingHosts[p.Id] = note.epoch
		} else {
			ret.ExistingHosts[p.Id] = 0
		}
	}

	return ret
}

func (v View) ValidMask(mask uint32) bool {
	err := validMask(mask, v.rings.numRings, v.maxByz)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	return true
}

func (v View) IsRingDisabled(mask, ringNum uint32) bool {
	err := checkDisabledRings(mask, v.rings.numRings, ringNum)
	if err != nil {
		log.Error(err.Error())
		return true
	}

	return false
}

func (v *View) deactivateRing(ringNumber uint32) (uint32, error) {
	if v.maxByz == 0 {
		return 0, errZeroDeactivate
	}

	ringIdx := ringNumber - 1

	maxIdx := v.rings.numRings - 1

	if ringIdx > maxIdx || ringNumber < 0 {
		return 0, errNonExistingRing
	}

	currMask := v.self.note.mask
	if active := hasBit(currMask, ringIdx); !active {
		return 0, errAlreadyDeactivated
	}

	if v.deactivatedRings == v.maxByz {
		var idx uint32
		for idx = 0; idx < maxIdx; idx++ {
			if idx != ringIdx && !hasBit(currMask, idx) {
				break
			}
		}
		currMask = setBit(currMask, idx)
	} else {
		v.deactivatedRings++
	}

	return clearBit(currMask, ringIdx), nil
}

func validMask(mask, numRings, maxByz uint32) error {
	active := bits.OnesCount32(mask)
	disabled := numRings - uint32(active)

	if disabled > maxByz {
		return errTooManyDeactivatedRings
	}

	return nil
}

func checkDisabledRings(mask, numRings, ringNum uint32) error {
	idx := ringNum - 1

	maxIdx := uint32(numRings - 1)

	if idx > maxIdx || idx < 0 {
		return errNonExistingRing
	}

	if active := hasBit(mask, idx); !active {
		return errDeactivatedRing
	}

	return nil
}

func setBit(n uint32, pos uint32) uint32 {
	n |= (1 << pos)
	return n
}

func clearBit(n uint32, pos uint32) uint32 {
	mask := uint32(^(1 << pos))
	n &= mask
	return n
}

func hasBit(n uint32, pos uint32) bool {
	val := n & (1 << pos)
	return (val > 0)
}
