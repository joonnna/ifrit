package core

import (
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
	"sync"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/protobuf"
)

var (
	errPeerId      = errors.New("certificate contains no subjectKeyIdentifier")
	errPeerAddr    = errors.New("certificate contains no address")
	errNoteId      = errors.New("note id is invalid")
	errNoteSign    = errors.New("note signature is invalid")
	errAccusedId   = errors.New("accused id is invalid")
	errAccuserId   = errors.New("accuser id is invalid")
	errAccuserSign = errors.New("accusation signature is invalid")
	errPubKeyErr   = errors.New("Public key type is invalid")
	errOldEpoch    = errors.New("accusation contains old epoch")
	errNoNote      = errors.New("no note found for the accused peer")
	errInvalidRing = errors.New("Tried to set an accusation on a non-existing ring")
)

type Peer struct {
	addr     string
	pingAddr string

	// Debuging and experiments only.
	httpAddr string

	noteMutex  sync.RWMutex
	recentNote *note

	accuseMutex sync.RWMutex
	accusations map[uint32]*accusation

	*peerId
	cert      *x509.Certificate
	publicKey *ecdsa.PublicKey

	avgLoss      uint64
	avgLossMutex sync.RWMutex

	nPing      uint32
	nPingMutex sync.RWMutex
}

//Ecdsa signature values
type signature struct {
	r []byte
	s []byte
}

type Note struct {
	epoch uint64
	mask  uint32
	id    string
	*signature
}

type Accusation struct {
	ringNum uint32
	epoch   uint64
	accuser *peerId
	mask    uint32
	id      string
	*signature
}

func NewPeer(note *Note, cert *x509.Certificate, numRings uint32) (*Peer, error) {
	var ok bool
	var i uint32
	var http string

	if fields := len(cert.Subject.Locality); fields < 2 {
		return nil, errPeerAddr
	} else if fields == 3 {
		http = cert.Subject.Locality[2]
	}

	if len(cert.SubjectKeyId) == 0 {
		return nil, errPeerId
	}

	pb := new(ecdsa.PublicKey)

	if pb, ok = cert.PublicKey.(*ecdsa.PublicKey); !ok {
		return nil, errPubKeyErr
	}

	accMap := make(map[uint32]*accusation)

	for i = 1; i <= numRings; i++ {
		accMap[i] = nil
	}

	return &peer{
		addr:        cert.Subject.Locality[0],
		pingAddr:    cert.Subject.Locality[1],
		httpAddr:    http,
		recentNote:  recentNote,
		cert:        cert,
		id:          cert.SubjectKeyId,
		publicKey:   pb,
		accusations: accMap,
	}, nil

}

func (p *Peer) AddAccusation(a *Accusation) error {
	p.accuseMutex.Lock()
	defer p.accuseMutex.Unlock()

	if p.recentNote != nil && p.recentNote.epoch != a.epoch {
		return errOldEpoch
	}

	if _, ok := p.accusations[a.ringNum]; !ok {
		return errInvalidRing
	}

	p.accusations[a.ringNum] = a

	return nil
}

func (p *Peer) RemoveRingAccusation(ringNum uint32) {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	if _, ok := p.accusations[ringNum]; !ok {
		log.Debug("Tried to remove accusation from invalid ring number", "ringnum", ringNum)
		return
	}

	p.accusations[ringNum] = nil
}

func (p *Peer) ClearAccusations() {
	p.accuseMutex.Lock()
	defer p.accuseMutex.Unlock()

	for k, _ := range p.accusations {
		p.accusations[k] = nil
	}
}

func (p *Peer) RingAccusation(ringNum uint32) *Accusation {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	if _, ok := p.accusations[ringNum]; !ok {
		log.Debug("Tried to get accusation from invalid ring number", "ringnum", ringNum)
		return nil
	}

	return p.accusations[ringNum]
}

func (p *Peer) AnyAccusation() *Accusation {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	for _, acc := range p.accusations {
		if acc != nil {
			return acc
		}
	}

	return nil
}

func (p *Peer) AllAccusations() []*Accusation {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	ret := make([]*accusation, 0, len(p.accusations))

	for _, v := range p.accusations {
		if v != nil {
			ret = append(ret, v)
		}
	}

	return ret
}

func (p *Peer) IsAccused() bool {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	for _, acc := range p.accusations {
		if acc != nil {
			return true
		}
	}

	return false
}

func (p *Peer) AddNewNote(newNote *Note) {
	p.noteMutex.Lock()
	defer p.noteMutex.Unlock()

	if p.recentNote == nil || p.recentNote.epoch < newNote.epoch {
		p.recentNote = newNote
	}
}

func (p *Peer) Note() *Note {
	p.noteMutex.RLock()
	defer p.noteMutex.RUnlock()

	return p.recentNote
}

func (p *Peer) Info() (*gossip.Certificate, *gossip.Note, []*gossip.Accusation) {
	var c *gossip.Certificate
	var n *gossip.Note

	c = &gossip.Certificate{
		Raw: p.cert.Raw,
	}

	recentNote := p.getNote()
	if recentNote != nil {
		n = recentNote.toPbMsg()
	}

	accs := p.AllAccusations()
	a := make([]*gossip.Accusation, 0, len(accs))

	for _, acc := range accs {
		a = append(acc.toPbMsg())
	}

	return c, n, a
}

func (p *Peer) IncrementPing() {
	p.nPingMutex.Lock()
	defer p.nPingMutex.Unlock()

	p.nPing++
}

func (p *Peer) ResetPing() {
	p.nPingMutex.Lock()
	defer p.nPingMutex.Unlock()

	p.nPing = 0
}

func (p *Peer) NumPing() uint32 {
	p.nPingMutex.RLock()
	defer p.nPingMutex.RUnlock()

	return p.nPing
}

func (a Accusation) Equal(other *Accusation) bool {
	if other == nil {
		return false
	}
	if a.id == other.id && a.accuser == other.accuser && a.ringNum == other.ringNum && a.epoch == other.epoch {
		return true
	}

	return false
}

func (n Note) IsMoreRecent(other *Note) bool {
	return n.epoch < other.epoch
}
