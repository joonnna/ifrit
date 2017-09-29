package node

import (
	"bytes"
	"crypto/x509"
	"errors"
	"sync"

	"github.com/joonnna/capstone/protobuf"
)

var (
	errPeerId            = errors.New("certificate contains no subjectKeyIdentifier")
	errPeerAddr          = errors.New("certificate contains no address")
	errNoteId            = errors.New("note id is invalid")
	errNoteSign          = errors.New("note signature is invalid")
	errAccusedId         = errors.New("accused id is invalid")
	errAccuserId         = errors.New("accuser id is invalid")
	errAccuserSign       = errors.New("accusation signature is invalid")
	errAccusationInvalid = errors.New("accusation is invlaid")
)

type peer struct {
	addr string

	noteMutex  sync.RWMutex
	recentNote *note

	accuseMutex sync.RWMutex
	*accusation

	*peerId
	cert *x509.Certificate
}

type peerId struct {
	//byte slice of subjectkeyIdentifier from certificate
	id []byte

	//string representation of id slice, used for internal maps
	key string
}

type note struct {
	epoch     uint64
	mask      string
	signature []byte
	*peerId
}

type accusation struct {
	epoch     uint64
	signature []byte
	accuser   *peerId
	*peerId
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

func newPeer(recentNote *note, cert *x509.Certificate) (*peer, error) {
	if len(cert.Subject.Locality) == 0 {
		return nil, errPeerAddr
	}

	if len(cert.SubjectKeyId) == 0 {
		return nil, errPeerId
	}

	return &peer{
		addr:       cert.Subject.Locality[0],
		recentNote: recentNote,
		cert:       cert,
		peerId:     newPeerId(cert.SubjectKeyId),
	}, nil
}

func (p *peer) setAccusation(a *accusation) error {
	p.accuseMutex.Lock()
	defer p.accuseMutex.Unlock()

	if p.recentNote == nil || p.recentNote.epoch < a.epoch {
		p.accusation = a
		return nil
	} else {
		return errAccusationInvalid
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

func (p *peer) createPbInfo() (*gossip.Certificate, *gossip.Note, *gossip.Accusation) {
	var c *gossip.Certificate
	var n *gossip.Note
	var a *gossip.Accusation

	c = &gossip.Certificate{
		Raw: p.cert.Raw,
	}

	recentNote := p.getNote()
	if recentNote != nil {
		n = &gossip.Note{
			Epoch:     recentNote.epoch,
			Id:        recentNote.id,
			Mask:      recentNote.mask,
			Signature: recentNote.signature,
		}
	}

	currAcc := p.getAccusation()
	if currAcc != nil {
		a = &gossip.Accusation{
			Epoch:     currAcc.epoch,
			Accuser:   currAcc.accuser.id,
			Accused:   currAcc.id,
			Signature: currAcc.signature,
		}
	}

	return c, n, a
}

func createNote(id *peerId, epoch uint64, signature []byte, mask string) (*note, error) {
	if len(id.id) == 0 {
		return nil, errNoteId
	}

	if len(signature) == 0 {
		return nil, errNoteSign
	}

	return &note{
		peerId:    id,
		epoch:     epoch,
		mask:      mask,
		signature: signature,
	}, nil
}

func (n note) isMoreRecent(epoch uint64) bool {
	return n.epoch < epoch
}

func newAccusation(id *peerId, epoch uint64, signature []byte, accuser *peerId) (*accusation, error) {
	if len(id.id) == 0 {
		return nil, errAccusedId
	}

	if len(signature) == 0 {
		return nil, errAccuserSign
	}

	if len(accuser.id) == 0 {
		return nil, errAccuserId
	}

	return &accusation{
		peerId:    id,
		epoch:     epoch,
		signature: signature,
		accuser:   accuser,
	}, nil
}

func cmpSignature(s1, s2 []byte) bool {
	return bytes.Equal(s1, s2)
}
