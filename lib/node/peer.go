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

	errOldEpoch = errors.New("accusation contains old epoch")
	errNoNote   = errors.New("no note found for the accused peer")
)

type peer struct {
	addr string

	noteMutex  sync.RWMutex
	recentNote *note

	accuseMutex sync.RWMutex
	*accusation

	*peerId
	cert      *x509.Certificate
	publicKey *ecdsa.PublicKey
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

func newPeer(recentNote *note, cert *x509.Certificate) (*peer, error) {
	var ok bool
	pb := new(ecdsa.PublicKey)

	if len(cert.Subject.Locality) == 0 {
		return nil, errPeerAddr
	}

	if len(cert.SubjectKeyId) == 0 {
		return nil, errPeerId
	}

	if pb, ok = cert.PublicKey.(*ecdsa.PublicKey); !ok {
		return nil, errPubKeyErr
	}

	return &peer{
		addr:       cert.Subject.Locality[0],
		recentNote: recentNote,
		cert:       cert,
		peerId:     newPeerId(cert.SubjectKeyId),
		publicKey:  pb,
	}, nil
}

/*
func (p peer) isSame(other *peer) bool {
	return bytes.Equal(p.peerId, other.peerId)
}
*/

func (p *peer) setAccusation(a *accusation) error {
	p.accuseMutex.Lock()
	defer p.accuseMutex.Unlock()

	if p.recentNote != nil && p.recentNote.epoch > a.epoch {
		return errOldEpoch
	}

	p.accusation = a

	return nil
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
		n = recentNote.toPbMsg()
	}

	currAcc := p.getAccusation()
	if currAcc != nil {
		a = currAcc.toPbMsg()
	}

	return c, n, a
}

func (n note) isMoreRecent(epoch uint64) bool {
	return n.epoch < epoch
}

func newAccusation(id *peerId, epoch uint64, accuser *peerId) (*accusation, error) {
	if len(id.id) == 0 {
		return nil, errAccusedId
	}

	if len(accuser.id) == 0 {
		return nil, errAccuserId
	}

	return &accusation{
		peerId:  id,
		epoch:   epoch,
		accuser: accuser,
	}, nil
}
