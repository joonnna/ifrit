package node

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
	"sync"

	"github.com/joonnna/firechain/lib/protobuf"
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

type peer struct {
	addr     string
	pingAddr string

	noteMutex  sync.RWMutex
	recentNote *note

	accuseMutex sync.RWMutex
	accusations map[uint32]*accusation

	*peerId
	cert      *x509.Certificate
	publicKey *ecdsa.PublicKey

	avgLoss      uint64
	avgLossMutex sync.RWMutex

	nPing      uint64
	nPingMutex sync.RWMutex
}

type peerId struct {
	//byte slice of subjectkeyIdentifier from certificate
	id []byte

	//string representation of id slice, used for internal maps
	key string
}

//Ecdsa signature values
type signature struct {
	r []byte
	s []byte
}

type note struct {
	epoch uint64
	mask  []byte
	*peerId
	*signature
}

type accusation struct {
	ringNum uint32
	epoch   uint64
	accuser *peerId
	mask    []byte
	*peerId
	*signature
}

func newPeerId(id []byte) *peerId {
	return &peerId{
		id:  id,
		key: string(id[:]),
	}
}

func (p peerId) cmp(other *peerId) int {
	return bytes.Compare(p.id, other.id)
}

func (p peerId) equal(other *peerId) bool {
	return bytes.Equal(p.id, other.id)
}

func newPeer(recentNote *note, cert *x509.Certificate, numRings uint32) (*peer, error) {
	var ok bool
	var i uint32

	if len(cert.Subject.Locality) < 2 {
		return nil, errPeerAddr
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
		recentNote:  recentNote,
		cert:        cert,
		peerId:      newPeerId(cert.SubjectKeyId),
		publicKey:   pb,
		accusations: accMap,
	}, nil

}

func (a accusation) equal(other *accusation) bool {
	if other == nil {
		return false
	}
	if a.peerId.equal(other.peerId) && a.accuser.equal(other.accuser) && a.ringNum == other.ringNum && a.epoch == other.epoch {
		return true
	}

	return false
}

func (p *peer) setAccusation(a *accusation) error {
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

func (p *peer) removeAccusation(ringNum uint32) {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	if _, ok := p.accusations[ringNum]; !ok {
		return
	}

	p.accusations[ringNum] = nil
}

func (p *peer) removeAccusations() {
	p.accuseMutex.Lock()
	defer p.accuseMutex.Unlock()

	for k, _ := range p.accusations {
		p.accusations[k] = nil
	}
}

func (p *peer) getRingAccusation(ringNum uint32) *accusation {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	if _, ok := p.accusations[ringNum]; !ok {
		return nil
	}

	return p.accusations[ringNum]
}

func (p *peer) getAnyAccusation() *accusation {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	for _, acc := range p.accusations {
		if acc != nil {
			return acc
		}
	}

	return nil
}

func (p *peer) getAllAccusations() []*accusation {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	ret := make([]*accusation, len(p.accusations))

	for _, v := range p.accusations {
		ret = append(ret, v)
	}

	return ret
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

func (p *peer) createPbInfo() (*gossip.Certificate, *gossip.Note, []*gossip.Accusation) {
	var c *gossip.Certificate
	var n *gossip.Note
	var a []*gossip.Accusation

	c = &gossip.Certificate{
		Raw: p.cert.Raw,
	}

	recentNote := p.getNote()
	if recentNote != nil {
		n = recentNote.toPbMsg()
	}

	accs := p.getAllAccusations()
	for _, acc := range accs {
		if acc != nil {
			a = append(a, acc.toPbMsg())
		}
	}

	return c, n, a
}

func (n note) isMoreRecent(epoch uint64) bool {
	return n.epoch < epoch
}

func (p *peer) incrementPing() {
	p.nPingMutex.Lock()
	defer p.nPingMutex.Unlock()

	p.nPing++
}

func (p *peer) resetPing() {
	p.nPingMutex.Lock()
	defer p.nPingMutex.Unlock()

	p.nPing = 0
}

func (p *peer) setAvgLoss() {
	p.avgLossMutex.Lock()
	defer p.avgLossMutex.Unlock()
}

func (p *peer) getAvgLoss() uint64 {
	p.avgLossMutex.RLock()
	defer p.avgLossMutex.RUnlock()

	return p.avgLoss
}

func (p *peer) getNPing() uint64 {
	p.nPingMutex.RLock()
	defer p.nPingMutex.RUnlock()

	return p.nPing
}
