package node


import (
	"sync"
)


type peer struct {
	noteLock sync.RWMutex

	notes map[uint64]*note
	lastNote uint64
	
	addr string

	accuseLock sync.RWMutex
	*accusation
}

type note struct {
	epoch uint64
	mask []byte
	signature []byte
}

type accusation struct {
	recentNote *note
	accuser string //TODO certificates...
}

func newPeer (addr string) *peer {
	return &peer {
		addr: addr, 
		notes: make(map[uint64]*note),
	}
}

func (p *Peer) addAccusation (lastNote *note, accuser string) {
	p.accuseLock.Lock()	
	defer p.accuseLock.Unlock()	

	a := &accusation {
		recentNote: lastNote,
		accuser: accuser,
	}

	p.accusation = a
}


func (p *Peer) getAccusation () *accusation {
	p.accuseLock.RLock()	
	defer p.accuseLock.RUnlock()	

	return r.accusation
}


func (p *Peer) addNote (newNote *note) {
	p.noteLock.Lock()	
	defer p.noteLock.Unlock()	

	p.notes[n.epoch] = note

	if n.epoch > n.lastNote {
		n.lastNote = n.epoch
	}
}


