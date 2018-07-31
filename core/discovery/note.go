package discovery

import (
	"crypto/ecdsa"
	"crypto/rand"

	"github.com/golang/protobuf/proto"
	"github.com/joonnna/ifrit/protobuf"
)

type Note struct {
	epoch uint64
	mask  uint32
	id    string
	*signature
}

func (n Note) Mask() uint32 {
	return n.mask
}

func (n Note) Equal(epoch uint64) bool {
	return n.epoch == epoch
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

func signNote(n *Note, privKey *ecdsa.PrivateKey) error {
	if privKey == nil {
		return errNoPrivKey
	}

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

	n.signature = &signature{
		r: r.Bytes(),
		s: s.Bytes(),
	}

	return nil
}

// ONLY for testing
func NewNote(id string, epoch uint64, mask uint32, priv *ecdsa.PrivateKey) *gossip.Note {
	n := &Note{
		id:    id,
		epoch: epoch,
		mask:  mask,
	}

	err := signNote(n, priv)
	if err != nil {
		panic(err)
	}

	return n.ToPbMsg()
}

// ONLY for testing
func NewUnsignedNote(id string, epoch uint64, mask uint32) *gossip.Note {
	n := &Note{
		id:        id,
		epoch:     epoch,
		mask:      mask,
		signature: &signature{},
	}

	return n.ToPbMsg()
}
