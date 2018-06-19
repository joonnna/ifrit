package discovery

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/protobuf"
)

var (
	errPeerId                  = errors.New("certificate contains no subjectKeyIdentifier")
	errPeerAddr                = errors.New("certificate contains no address")
	errNoteId                  = errors.New("note id is invalid")
	errNoteSign                = errors.New("note signature is invalid")
	errAccusedId               = errors.New("accused id is invalid")
	errAccuserId               = errors.New("accuser id is invalid")
	errAccuserSign             = errors.New("accusation signature is invalid")
	errPubKeyErr               = errors.New("Public key type is invalid")
	errOldEpoch                = errors.New("accusation contains old epoch")
	errNoNote                  = errors.New("no note found for the accused peer")
	errInvalidRing             = errors.New("Tried to set an accusation on a non-existing ring")
	errTooManyDeactivatedRings = errors.New("Mask contains too many deactivated rings")
	errNonExistingRing         = errors.New("Accusation specifies non exisiting ring")
	errDeactivatedRing         = errors.New("Accusation on deactivated ring")
)

type Peer struct {
	addr     string
	pingAddr string

	// Debuging and experiments only.
	httpAddr string

	noteMutex sync.RWMutex
	note      *Note

	accuseMutex sync.RWMutex
	accusations map[uint32]*Accusation

	Id        string
	cert      *x509.Certificate
	publicKey *ecdsa.PublicKey

	nPing      uint32
	nPingMutex sync.RWMutex
}

// Ecdsa signature values
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
	accuser string
	mask    uint32
	accused string
	*signature
}

type timeout struct {
	observer  *Peer
	timeStamp time.Time
	accused   *Peer
	lastNote  *Note
}

func NewPeer(cert *x509.Certificate, numRings uint32) (*Peer, error) {
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

	accMap := make(map[uint32]*Accusation)

	for i = 1; i <= numRings; i++ {
		accMap[i] = nil
	}

	return &Peer{
		addr:        cert.Subject.Locality[0],
		pingAddr:    cert.Subject.Locality[1],
		httpAddr:    http,
		cert:        cert,
		Id:          string(cert.SubjectKeyId),
		publicKey:   pb,
		accusations: accMap,
	}, nil

}

func (p Peer) Certificate() []byte {
	return p.cert.Raw
}

func (p Peer) Addr() string {
	return p.addr
}

func (p Peer) ValidateSignature(r, s, data []byte) bool {
	var rInt, sInt big.Int

	b := hashContent(data)

	rInt.SetBytes(r)
	sInt.SetBytes(s)

	return ecdsa.Verify(p.publicKey, b, &rInt, &sInt)
}

func (p *Peer) AddAccusation(accused, accuser string, epoch uint64, mask, ringNum uint32, r, s []byte) error {
	p.accuseMutex.Lock()
	defer p.accuseMutex.Unlock()

	if p.note != nil && p.note.epoch != epoch {
		return errOldEpoch
	}

	if _, ok := p.accusations[ringNum]; !ok {
		return errInvalidRing
	}

	a := &Accusation{
		accused: accused,
		accuser: accuser,
		mask:    mask,
		epoch:   epoch,
		ringNum: ringNum,
		signature: &signature{
			r: r,
			s: s,
		},
	}

	p.accusations[a.ringNum] = a

	log.Debug("Added accusation", "addr", p.addr, "ring", a.ringNum)

	return nil
}

func (p *Peer) RemoveAccusation(a *Accusation) {
	p.accuseMutex.RLock()
	defer p.accuseMutex.RUnlock()

	if _, ok := p.accusations[a.ringNum]; !ok {
		log.Debug("Tried to remove accusation from invalid ring number", "ringnum", a.ringNum)
		return
	}

	p.accusations[a.ringNum] = nil
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

	ret := make([]*Accusation, 0, len(p.accusations))

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

func (p *Peer) AddNote(mask uint32, epoch uint64, r, s []byte) {
	p.noteMutex.Lock()
	defer p.noteMutex.Unlock()

	if p.note == nil || p.note.IsMoreRecent(epoch) {
		p.note = &Note{
			id:    p.Id,
			mask:  mask,
			epoch: epoch,
			signature: &signature{
				r: r,
				s: s,
			},
		}
	}
}

func (p *Peer) Note() *Note {
	p.noteMutex.RLock()
	defer p.noteMutex.RUnlock()

	return p.note
}

func (n Note) SameEpoch(epoch uint64) bool {
	return n.epoch == epoch
}

func (p *Peer) Info() (*gossip.Certificate, *gossip.Note, []*gossip.Accusation) {
	var c *gossip.Certificate

	c = &gossip.Certificate{
		Raw: p.cert.Raw,
	}

	n := p.Note().ToPbMsg()

	accs := p.AllAccusations()
	a := make([]*gossip.Accusation, 0, len(accs))

	for _, acc := range accs {
		a = append(a, acc.ToPbMsg())
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

/*
func (a Accusation) Equal(other *Accusation) bool {
	if other == nil {
		return false
	}
	if a.accused == other.accused && a.accuser == other.accuser && a.ringNum == other.ringNum && a.epoch == other.epoch {
		return true
	}

	return false
}
*/

func (a Accusation) Equal(accused, accuser string, ringNum uint32, epoch uint64) bool {
	if a.accused == accused && a.accuser == accuser && a.ringNum == ringNum && a.epoch == epoch {
		return true
	}

	return false
}

func (a Accusation) IsMoreRecent(other uint64) bool {
	return a.epoch < other
}

func (a Accusation) IsAccuser(id string) bool {
	return a.accuser == id
}

func (n Note) IsMoreRecent(other uint64) bool {
	return n.epoch < other
}

func (n Note) ToPbMsg() *gossip.Note {
	return &gossip.Note{
		Epoch: n.epoch,
		Id:    []byte(n.id),
		Mask:  n.mask,
		Signature: &gossip.Signature{
			R: n.r,
			S: n.s,
		},
	}
}

func (a Accusation) ToPbMsg() *gossip.Accusation {
	return &gossip.Accusation{
		Epoch:   a.epoch,
		Accuser: []byte(a.accuser),
		Accused: []byte(a.accused),
		Mask:    a.mask,
		RingNum: a.ringNum,
		Signature: &gossip.Signature{
			R: a.r,
			S: a.s,
		},
	}
}

// Only used for local note.
// Access regulated by mutex in node structure.
func (p *Peer) SignNote(privKey *ecdsa.PrivateKey) error {
	n := p.note

	noteMsg := &gossip.Note{
		Epoch: n.epoch,
		Id:    []byte(n.id),
		Mask:  n.mask,
	}

	b, err := proto.Marshal(noteMsg)
	if err != nil {
		return err
	}

	hash := hashContent(b)

	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash)
	if err != nil {
		return err
	}

	p.note.signature = &signature{
		r: r.Bytes(),
		s: s.Bytes(),
	}

	return nil
}

func hashContent(data []byte) []byte {
	h := sha256.New224()
	h.Write(data)
	return h.Sum(nil)
}

/*
func (n *Note) AddSignature(r, s []byte) {

}

func (n *Note) Marshal() ([]byte, error) {

}

func (n *Note) sign(privKey *ecdsa.PrivateKey) error {
	noteMsg := &gossip.Note{
		Epoch: n.epoch,
		Id:    []byte(n.id),
		Mask:  n.mask,
	}

	b, err := proto.Marshal(noteMsg)
	if err != nil {
		return err
	}

	signature, err := signContent(b, privKey)
	if err != nil {
		return err
	}

	n.signature = signature

	return nil
}

func (n *Note) signAndMarshal(privKey *ecdsa.PrivateKey) (*gossip.Note, error) {
	noteMsg := &gossip.Note{
		Epoch: n.epoch,
		Id:    []byte(n.id),
		Mask:  n.mask,
	}

	b, err := proto.Marshal(noteMsg)
	if err != nil {
		return nil, err
	}

	signature, err := signContent(b, privKey)
	if err != nil {
		return nil, err
	}

	noteMsg.Signature = &gossip.Signature{
		R: signature.r,
		S: signature.s,
	}

	n.signature = signature

	return noteMsg, nil
}


func (a Accusation) signAndMarshal(privKey *ecdsa.PrivateKey) (*gossip.Accusation, error) {
	acc := &gossip.Accusation{
		Epoch:   a.epoch,
		Accuser: []byte(a.accuser),
		Accused: []byte(a.accused),
		Mask:    a.mask,
		RingNum: a.ringNum,
	}

	b, err := proto.Marshal(acc)
	if err != nil {
		return nil, err
	}

	signature, err := signContent(b, privKey)
	if err != nil {
		return nil, err
	}

	acc.Signature = &gossip.Signature{
		R: signature.r,
		S: signature.s,
	}

	return acc, nil
}

func (a *Accusation) sign(privKey *ecdsa.PrivateKey) error {
	acc := &gossip.Accusation{
		Epoch:   a.epoch,
		Accuser: []byte(a.accuser),
		Accused: []byte(a.accused),
		Mask:    a.mask,
		RingNum: a.ringNum,
	}

	b, err := proto.Marshal(acc)
	if err != nil {
		return err
	}

	signature, err := signContent(b, privKey)
	if err != nil {
		return err
	}

	a.signature = signature

	return nil
}
*/