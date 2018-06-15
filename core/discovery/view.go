package discovery

import (
	"crypto/x509"
	"errors"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
)

var (
	errInvalidNeighbours = errors.New("Neighbours are nil ?!")
	errPeerAlreadyExists = errors.New("Peer id already exists in the full view")
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
}

func NewView(numRings uint32, self *Peer, removalTimeout uint32) (*View, error) {
	maxByz := (float64(numRings) / 2.0) - 1
	if maxByz < 0 {
		maxByz = 1
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
		removalTimeout:  float64(removalTimeout),
		maxByz:          uint32(maxByz),
		currGossipRing:  1,
		currMonitorRing: 1,
	}

	return v, nil
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

func (v *View) FullView() []*Peer {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	ret := make([]*Peer, 0, len(v.viewMap))

	for _, p := range v.viewMap {
		ret = append(ret, p)
	}

	return ret
}

func (v *View) LiveView() []*Peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	ret := make([]*Peer, 0, len(v.liveMap))

	for _, p := range v.liveMap {
		ret = append(ret, p)
	}

	return ret

}

func (v *View) AddFull(id string, note *Note, cert *x509.Certificate) error {
	v.viewMutex.Lock()
	defer v.viewMutex.Unlock()

	if _, ok := v.viewMap[id]; ok {
		log.Error("Tried to add peer twice to viewMap")
		return errPeerAlreadyExists
	}

	p, err := NewPeer(note, cert, v.rings.numRings)
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

func (v *View) MonitorTarget() *Peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	// Only one goroutine accesses monitor ring variable,
	// only take read lock for live view.
	defer v.incrementMonitorRing()

	return v.rings.myRingSuccessor(v.currMonitorRing)
}

func (v *View) AddLive(p *Peer) {
	v.liveMutex.Lock()
	defer v.liveMutex.Unlock()

	if _, ok := v.liveMap[p.Id]; ok {
		log.Error("Tried to add peer twice to liveMap", "addr", p.addr)
	}

	v.liveMap[p.Id] = p

	v.rings.add(p)
}

func (v *View) RemoveLive(id string) {
	v.liveMutex.Lock()
	defer v.liveMutex.Unlock()

	if peer, ok := v.liveMap[id]; ok {
		v.rings.remove(peer)

		delete(v.liveMap, peer.Id)

		log.Debug("Removed livePeer", "addr", peer.addr)
	} else {
		log.Debug("Tried to remove non-existing peer from live view", "addr", peer.addr)
	}
}

func (v *View) StartTimer(id string, n *Note, observer *Peer) {
	v.timeoutMutex.Lock()
	defer v.timeoutMutex.Unlock()

	if t, ok := v.timeoutMap[id]; ok && t.lastNote.epoch >= n.epoch {
		return
	}

	newTimeout := &timeout{
		observer:  observer,
		timeStamp: time.Now(),
		lastNote:  n,
	}

	v.timeoutMap[id] = newTimeout
}

func (v *View) deleteTimeout(id string) {
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
	log.Debug("Have timeouts", "amount", len(timeouts))

	for _, t := range timeouts {
		if time.Since(t.timeStamp).Seconds() > v.removalTimeout {
			log.Debug("Timeout expired, removing from live", "id", t.lastNote.id)
			v.deleteTimeout(t.lastNote.id)
			v.RemoveLive(t.lastNote.id)
		}
	}
}

func (v *View) ShouldBeNeighbour(id string) bool {
	return v.rings.shouldBeMyNeighbour(id)
}

func (v *View) FindNeighbours(id string) []*Peer {
	return v.rings.findNeighbours(id)
}

func (v *View) ValidAccuser(accused, accuser string, ringNum uint32) bool {
	return v.rings.isPredecessor(accused, accuser, ringNum)
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

/*
func (v *View) addToLiveMap(p *Peer) error {
	v.liveMutex.Lock()
	defer v.liveMutex.Unlock()

	if _, ok := v.liveMap[p.Id]; ok {
		log.Error("Tried to add peer twice to liveMap", "addr", p.addr)
		return errAlreadyInLive
	}

	v.liveMap[p.Id] = p
}
*/

// ########################### OLD BENEATH THIS LINE ###########################
/*
func (v *view) getViewAddrs() []string {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	ret := make([]string, 0, len(v.viewMap))

	for _, v := range v.viewMap {
		ret = append(ret, v.addr)
	}

	return ret
}

func (v *view) getView() []*peer {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	ret := make([]*peer, len(v.viewMap))
	idx := 0

	for _, v := range v.viewMap {
		ret[idx] = v
		idx++
	}

	return ret
}

func (v *view) getRandomViewPeer() *peer {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	var ret *peer

	for _, v := range v.viewMap {
		ret = v
		break
	}

	return ret
}

func (v *view) addViewPeer(key string, n *note, cert *x509.Certificate, rings uint32) {
	v.viewMutex.Lock()
	defer v.viewMutex.Unlock()

	if _, ok := v.viewMap[key]; ok {
		log.Error("Tried to add peer twice to viewMap")
		return
	}

	p, err := newPeer(n, cert, rings)
	if err != nil {
		log.Error(err.Error())
		return
	}

	v.viewMap[p.key] = p
}

func (v *view) viewPeerExist(key string) bool {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	_, ok := v.viewMap[key]

	return ok
}

func (v *view) getViewPeer(key string) *peer {
	v.viewMutex.RLock()
	defer v.viewMutex.RUnlock()

	var p *peer
	var ok bool

	if p, ok = v.viewMap[key]; !ok {
		return nil
	}

	return p
}

func (v *view) getNeighbours() []string {
	var neighbours []string
	for _, ring := range v.ringMap {
		succ, err := ring.getRingSucc()
		if err != nil {
			log.Error(err.Error())
		} else {
			neighbours = append(neighbours, succ.addr)
		}

		prev, err := ring.getRingPrev()
		if err != nil {
			log.Error(err.Error())
		} else {
			neighbours = append(neighbours, prev.addr)
		}
	}
	return neighbours
}

func (v *view) livePeerExist(key string) bool {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	_, ok := v.liveMap[key]

	return ok
}

func (v *view) getLivePeers() []*peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	idx := 0
	ret := make([]*peer, len(v.liveMap))

	for _, p := range v.liveMap {
		ret[idx] = p
		idx++
	}

	return ret
}

func (v *view) getLivePeerAddrs() []string {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	idx := 0
	ret := make([]string, len(v.liveMap))

	for _, p := range v.liveMap {
		ret[idx] = p.addr
		idx++
	}

	return ret
}

func (v *view) getLivePeerHttpAddrs() []string {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	idx := 0
	ret := make([]string, len(v.liveMap))

	for _, p := range v.liveMap {
		ret[idx] = p.httpAddr
		idx++
	}

	return ret
}

func (v *view) getLivePeer(key string) *peer {
	v.liveMutex.RLock()
	defer v.liveMutex.RUnlock()

	var ok bool
	var p *peer

	if p, ok = v.liveMap[key]; !ok {
		return nil
	}

	return p
}

func (v *view) addLivePeer(p *peer) {
	v.liveMutex.Lock()

	if _, ok := v.liveMap[p.key]; ok {
		log.Error("Tried to add peer twice to liveMap", "addr", p.addr)
		v.liveMutex.Unlock()
		return
	}

	v.liveMap[p.key] = p
	v.liveMutex.Unlock()

	//TODO should check if i get a new neighbor, if so, add possibility
	//to remove rpc connection of old neighbor.

	var prevId *peerId

	for _, ring := range v.ringMap {
		//TODO handle this differently? continue after failed ring add is dodgy
		//Although no errors "should" occur
		err := ring.add(p.id, p.addr)
		if err != nil {
			log.Error(err.Error())
			continue
		}

		succKey, prevKey, err := ring.findNeighbours(p.id)
		if err != nil {
			log.Error(err.Error())
			continue
		}

		succ := v.getLivePeer(succKey)
		prev := v.getLivePeer(prevKey)

		//Special case when prev is the local peer
		//do not care if local peer is succ, will not have accusations about myself
		if prev != nil {
			prevId = prev.peerId
		} else if prevKey == v.local.key {
			prevId = v.local
		}

		//Occurs when a fresh node starts up and has no nodes in its view
		//Or if the local node is either the new succ or prev
		//TODO handle this differently?
		if succ == nil || prevId == nil {
			continue
		}

		acc := succ.getRingAccusation(ring.ringNum)
		if acc != nil && acc.accuser.equal(prevId) {
			succ.removeAccusation(ring.ringNum)
		}
	}
}

func (v *view) removeLivePeer(key string) {
	v.liveMutex.Lock()
	defer v.liveMutex.Unlock()

	var ok bool
	var p *peer

	if p, ok = v.liveMap[key]; !ok {
		return
	}
	id := p.id

	log.Debug("Removed livePeer", "addr", p.addr)
	delete(v.liveMap, key)

	for _, ring := range v.ringMap {
		err := ring.remove(id)
		if err != nil {
			log.Error(err.Error())
			continue
		}
	}
}

func (v *view) timerExist(key string) bool {
	v.timeoutMutex.RLock()
	defer v.timeoutMutex.RUnlock()

	_, ok := v.timeoutMap[key]

	return ok
}

func (v *view) startTimer(key string, newNote *note, observer *peer, addr string) {
	v.timeoutMutex.Lock()
	defer v.timeoutMutex.Unlock()

	if t, ok := v.timeoutMap[key]; ok && newNote != nil {
		if t.lastNote.epoch >= newNote.epoch {
			return
		}
	}

	newTimeout := &timeout{
		observer:  observer,
		lastNote:  newNote,
		timeStamp: time.Now(),
		addr:      addr,
	}
	v.timeoutMap[key] = newTimeout
}

func (v *view) deleteTimeout(key string) {
	v.timeoutMutex.Lock()
	defer v.timeoutMutex.Unlock()

	delete(v.timeoutMap, key)
}

func (v *view) getTimeout(key string) *timeout {
	v.timeoutMutex.RLock()
	defer v.timeoutMutex.RUnlock()

	if t, ok := v.timeoutMap[key]; !ok {
		return nil
	} else {
		return t
	}
}

func (v *view) getAllTimeouts() map[string]*timeout {
	v.timeoutMutex.RLock()
	defer v.timeoutMutex.RUnlock()

	ret := make(map[string]*timeout)

	for k, val := range v.timeoutMap {
		ret[k] = val
	}

	return ret
}

func (v *view) isPrev(curr, toCheck *peer, ringNum uint32) bool {
	r, ok := v.ringMap[ringNum]
	if !ok {
		log.Error("checking prev on non-existing ring")
		return false
	}

	prev, err := r.isPrev(curr.id, toCheck.id)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	return prev
}

func (v *view) shouldBeNeighbours(id *peerId) bool {
	for _, r := range v.ringMap {
		if r.betweenNeighbours(id.id) {
			return true
		}
	}
	return false
}

func (v *view) findNeighbours(id *peerId) []string {
	var keys []string
	exist := make(map[string]bool)

	for _, r := range v.ringMap {
		succ, prev, err := r.findNeighbours(id.id)
		if err != nil {
			continue
		}
		if _, ok := exist[succ]; !ok {
			keys = append(keys, succ)
			exist[succ] = true
		}

		if _, ok := exist[prev]; !ok {
			keys = append(keys, prev)
			exist[prev] = true
		}
	}
	return keys
}

func (v *view) getGossipPartners() ([]string, error) {
	var addrs []string
	defer v.incrementGossipRing()

	r := v.ringMap[v.currGossipRing]

	succ, err := r.getRingSucc()
	if err != nil {
		return nil, err
	}
	addrs = append(addrs, succ.addr)

	prev, err := r.getRingPrev()
	if err != nil {
		return nil, err
	}

	addrs = append(addrs, prev.addr)

	return addrs, nil
}

func (v *view) getMonitorTarget() (string, uint32, error) {
	defer v.incrementMonitorRing()

	r := v.ringMap[v.currMonitorRing]

	succ, err := r.getRingSucc()
	if err != nil {
		return "", 0, err
	}

	return succ.key, r.ringNum, nil
}
*/
