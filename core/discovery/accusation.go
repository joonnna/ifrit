package discovery

import (
	"crypto/ecdsa"
	"crypto/rand"

	"github.com/golang/protobuf/proto"
	"github.com/joonnna/ifrit/protobuf"
)

type Accusation struct {
	ringNum uint32
	epoch   uint64
	accuser string
	mask    uint32
	accused string
	*signature
}

func (a Accusation) Equal(accused, accuser string, ringNum uint32, epoch uint64) bool {
	return a.accused == accused && a.accuser == accuser && a.ringNum == ringNum && a.epoch == epoch
}

func (a Accusation) IsMoreRecent(other uint64) bool {
	return a.epoch < other
}

func (a Accusation) IsAccuser(id string) bool {
	return a.accuser == id
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

func (a *Accusation) sign(privKey *ecdsa.PrivateKey) error {
	accMsg := &gossip.Accusation{
		Epoch:   a.epoch,
		Accuser: []byte(a.accuser),
		Accused: []byte(a.accused),
		RingNum: a.ringNum,
		Mask:    a.mask,
	}

	b, err := proto.Marshal(accMsg)
	if err != nil {
		return err
	}

	hash := hashContent(b)

	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash)
	if err != nil {
		return err
	}

	a.signature = &signature{
		r: r.Bytes(),
		s: s.Bytes(),
	}

	return nil
}
