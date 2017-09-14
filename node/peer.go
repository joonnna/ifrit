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
	//recentNote *note
	accuser string //TODO certificates...
}

func newPeer (addr string) *peer {
	return &peer {
		addr: addr, 
		notes: make(map[uint64]*note),
	}
}

func (p *peer) addAccusation (accuser string) {
	p.accuseLock.Lock()	
	defer p.accuseLock.Unlock()	

	a := &accusation {
		//recentNote: lastNote,
		accuser: accuser,
	}

	p.accusation = a
}


func (p *peer) getAccusation () *accusation {
	p.accuseLock.RLock()	
	defer p.accuseLock.RUnlock()	

	return p.accusation
}


func (p *peer) addNote (newNote *note) {
	p.noteLock.Lock()	
	defer p.noteLock.Unlock()	

	p.notes[newNote.epoch] = newNote

	if newNote.epoch > p.lastNote {
		p.lastNote = newNote.epoch
	}
}


