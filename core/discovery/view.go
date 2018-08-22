package discovery

import (
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"math/bits"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/protobuf"
	pb "github.com/joonnna/ifrit/protobuf"
	"github.com/spf13/viper"
)

var (
	errInvalidNeighbours  = errors.New("Neighbours are nil ?!")
	errPeerAlreadyExists  = errors.New("Peer id already exists in the full view")
	errAlreadyDeactivated = errors.New("Ring was already deactivated")
	errZeroDeactivate     = errors.New("No ring can be deactivated, maxbyz is 0")
	errNoNote             = errors.New("Note was nil.")
	errAccusedIsNil       = errors.New("Accused was nil.")
	errObsIsNil           = errors.New("Observer was nil")
	errWrongNote          = errors.New("Note does not belong to accused.")
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
	updateTimeout  time.Duration

	self *Peer

	cm connectionManager
	s  signer

	exitChan chan bool
}

type connectionManager interface {
	CloseConn(addr string)
}

type signer interface {
	Sign([]byte) ([]byte, []byte, error)
}

func NewView(numRings uint32, cert *x509.Certificate, cm connectionManager, s signer) (*View, error) {
	var i, mask uint32

	maxByz := (float64(numRings) / 2.0) - 1
	if maxByz < 0 {
		maxByz = 0
	}

	self, err := newPeer(cert, numRings)
	if err != nil {
		return nil, err
	}

	rings, err := createRings(self, numRings)
	if err != nil {
		return nil, err
	}

	v := &View{
		rings:           rings,
		viewMap:         make(map[string]*Peer),
		liveMap:         make(map[string]*Peer),
		timeoutMap:      make(map[string]*timeout),
		maxByz:          uint32(maxByz),
		currGossipRing:  1,
		currMonitorRing: 1,
		self:            self,
		cm:              cm,
		exitChan:        make(chan bool, 1),
		s:               s,

		removalTimeout: viper.GetFloat64("dead_timeout"),
		updateTimeout: time.Second * time.Duration(viper.
			GetInt32("view_update_interval")),
	}

	for i = 0; i < numRings; i++ {
		mask = setBit(mask, i)
	}

	localNote := &Note{
		epoch: 1,
		mask:  mask,
		id:    self.Id,
	}

	err = v.signLocalNote(localNote)
	if err != nil {
		return nil, err
	}

	return v, nil
}

func (v *View) Start() {
	for {
		select {
		case <-v.exitChan:
			log.Info("Stopping view update")
			return
		case <-time.After(v.updateTimeout):
			v.checkTimeouts()
		}
	}
}

func (v *View) Stop() {
	close(v.exitChan)
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
		return
	}

	v.liveMap[p.Id] = p

	old := v.rings.add(p)
	for _, addr := range old {
		v.cm.CloseConn(addr)
	}
}

func (v *View) MyRingNeighbours(ringNum uint32) (*Peer, *Peer) {
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

		v.cm.CloseConn(peer.Addr)

		log.Debug("Removed livePeer", "addr", peer.Addr)
	} else {
		log.Debug("Tried to remove non-existing peer from live view.")
	}
}

func (v *View) StartTimer(accused *Peer, n *Note, observer *Peer) error {
	v.timeoutMutex.Lock()
	defer v.timeoutMutex.Unlock()

	var newTimeout *timeout

	if accused == nil {
		return errAccusedIsNil
	}

	if n == nil {
		return errNoNote
	}

	if observer == nil {
		return errObsIsNil
	}

	if n.id != accused.Id {
		return errWrongNote
	}

	if t, ok := v.timeoutMap[accused.Id]; ok {
		if t.lastNote.epoch < n.epoch {
			newTimeout = &timeout{
				observer:  observer,
				timeStamp: t.timeStamp,
				lastNote:  n,
				accused:   accused,
			}
		} else {
			return nil
		}
	} else {
		newTimeout = &timeout{
			observer:  observer,
			timeStamp: time.Now(),
			lastNote:  n,
			accused:   accused,
		}
	}

	v.timeoutMap[accused.Id] = newTimeout

	log.Debug("Started timer", "addr", accused.Addr)

	return nil
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

func (v *View) checkTimeouts() {
	timeouts := v.allTimeouts()
	if numTimeouts := len(timeouts); numTimeouts > 0 {
		log.Debug("Have timeouts", "amount", numTimeouts)
	}

	for _, t := range timeouts {
		if time.Since(t.timeStamp).Seconds() > v.removalTimeout {
			log.Debug("Timeout expired, removing from live", "addr", t.accused.Addr)
			v.RemoveLive(t.accused.Id)
			v.DeleteTimeout(t.accused.Id)
		}
	}
}

func (v *View) ShouldRebuttal(epoch uint64, ringNum uint32) bool {
	v.self.noteMutex.Lock()
	defer v.self.noteMutex.Unlock()

	if eq := v.self.note.Equal(epoch); eq {
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

		err = v.signLocalNote(newNote)
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

func (v *View) ValidAccuser(accused, accuser *Peer, ringNum uint32) bool {
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

func (v *View) ValidMask(mask uint32) bool {
	err := validMask(mask, v.rings.numRings, v.maxByz)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	return true
}

func (v *View) deactivateRing(ringNumber uint32) (uint32, error) {
	var idx uint32

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
		for idx = 0; idx <= maxIdx; idx++ {
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

func (v *View) signLocalNote(n *Note) error {
	pbNote := &pb.Note{
		Epoch: n.epoch,
		Mask:  n.mask,
		Id:    []byte(n.id),
	}

	bytes, err := proto.Marshal(pbNote)
	if err != nil {
		return err
	}

	r, s, err := v.s.Sign(bytes)
	if err != nil {
		return err
	}

	n.signature = &signature{
		r: r,
		s: s,
	}

	v.self.note = n

	return err
}

func validMask(mask, numRings, maxByz uint32) error {
	active := bits.OnesCount32(mask)
	disabled := int(numRings) - active

	if disabled > int(maxByz) {
		return errTooManyDeactivatedRings
	}

	return nil
}

func hashContent(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
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

/*

########## METHODS ONLY USED FOR TESTING BELOW THIS LINE ##########

*/

// ONLY for testing
func (v *View) RemoveTestFull(id string) {
	delete(v.viewMap, id)
}

// ONLY for testing
func (v *View) Compare(other *View) error {
	for id, p := range v.viewMap {
		if p2, ok := other.viewMap[id]; !ok {
			return errors.New("Peer not present in both views.")
		} else {
			for ringNum, a := range p.accusations {
				if a2, ok := p2.accusations[ringNum]; !ok {
					return errors.New("Accusation not present in both views.")
				} else {
					if !a.Equal(a2.accused, a2.accuser, a2.ringNum, a2.epoch) {
						return errors.New("Accusation were not equal in both views.")
					}
				}
			}
			if !p.note.Equal(p2.note.epoch) {
				return errors.New("Did not have same note for peer in both views.")
			}
		}
	}

	for id, _ := range v.liveMap {
		if _, ok := other.liveMap[id]; !ok {
			return errors.New("Did not have peer in both live views.")
		}
	}

	return nil
}
