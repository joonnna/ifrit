package node


import (
	"sync"
)


type peer struct {
	noteMutex sync.RWMutex
	recentNote *note

	addr string

	accuseMutex sync.RWMutex
	*accusation
}

type note struct {
	epoch uint64
	mask string
	addr string
	//TODO signature...
}

type accusation struct {
	recentNote *note
	accuser string //TODO certificates...
	noteMutex sync.RWMutex
}

func newPeer (recentNote *note) *peer {
	return &peer {
		addr: recentNote.addr,
		recentNote: recentNote,
	}
}

func createNote (addr string, epoch uint64, mask string) *note {
	return &note{
		addr: addr,
		epoch: epoch,
		mask: mask,
	}
}

func (p *peer) setAccusation (accuser string, recentNote *note) {
	p.accuseMutex.Lock()
	defer p.accuseMutex.Unlock()

	if p.accusation == nil || p.accusation.recentNote.epoch < recentNote.epoch {
		a := &accusation {
			recentNote: recentNote,
			accuser: accuser,
		}
		p.accusation = a
	}
}

func (p *peer) removeAccusation() {
	p.accuseMutex.Lock()
	defer p.accuseMutex.Unlock()

	p.accusation = nil
}


func (p *peer) getAccusation() *accusation {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	return p.accusation
}

func (p *peer) setNote(newNote *note) {
	p.noteMutex.Lock()
	defer p.noteMutex.Unlock()

	if p.recentNote == nil || p.recentNote.epoch < newNote.epoch {
		p.recentNote = newNote
	}
}

func (p *peer) getNote() *note {
	p.noteMutex.RLock()
	defer p.noteMutex.RUnlock()

	return p.recentNote
}


func (a *accusation) setNote(newNote *note) {
	a.noteMutex.Lock()
	defer a.noteMutex.Unlock()

	if a.recentNote == nil || a.recentNote.epoch < newNote.epoch {
		a.recentNote = newNote
	}
}

func (a *accusation) getNote() *note {
	a.noteMutex.RLock()
	defer a.noteMutex.RUnlock()

	return a.recentNote
}




func (n note) isMoreRecent(epoch uint64) bool {
	return n.epoch < epoch
}
