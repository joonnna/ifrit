package node


import (
	"sync"
)


type peer struct {
	/*
	notes map[uint64]*note
	lastNote uint64
	*/

	noteMutex sync.RWMutex
	recentNote *note

	addr string

	accuseMutex sync.RWMutex
	*accusation
}

type note struct {
	epoch uint64
	mask string
	//TODO signature...
}

type accusation struct {
	recentNote *note
	accuser string //TODO certificates...
}

func newPeer (addr string, recentNote *note) *peer {
	return &peer {
		addr: addr, 
		recentNote: recentNote,
	}
}

func createNote (epoch uint64, mask string) *note {
	return &note{
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

func (n note) isMoreRecent(epoch uint64) bool {
	return n.epoch < epoch
}

/*
func (p *peer) isRecentNote(epoch uint64) bool {
	p.noteMutex.RLock()	
	defer p.noteMutex.RUnlock()	
	
	if p.recentNote == nil {
		return true
	}
	return p.recentNote.epoch < epoch
}

func (p *peer) isRecentAccusation(epoch uint64) bool {
	p.accuseMutex.RLock()	
	defer p.accuseMutex.RUnlock()	

	if p.accusation == nil {
		return true
	}
	return p.accusation.recentNote.epoch < epoch
}
*/

