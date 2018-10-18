package discovery

import (
	"crypto/ecdsa"
	"crypto/rand"

	"github.com/golang/protobuf/proto"
	pb "github.com/joonnna/ifrit/protobuf"
)

type Note struct {
	epoch uint64
	mask  uint32
	id    string
	*signature
}

func (n Note) IsRingDisabled(ringNum, numRings uint32) bool {
	idx := int(ringNum) - 1

	maxIdx := uint32(numRings - 1)

	if idx > int(maxIdx) || idx < 0 {
		return false
	}

	return !hasBit(n.mask, uint32(idx))
}

func (n *Note) Equal(epoch uint64) bool {
	return n.epoch == epoch
}

func (n *Note) IsMoreRecent(other uint64) bool {
	return n.epoch < other
}

func (n *Note) ToPbMsg() *pb.Note {
	return &pb.Note{
		Epoch: n.epoch,
		Id:    []byte(n.id),
		Mask:  n.mask,
		Signature: &pb.Signature{
			R: n.r,
			S: n.s,
		},
	}
}

/*

########## METHODS ONLY USED FOR TESTING BELOW THIS LINE ##########

*/

// ONLY FOR TESTING
func NewNote(id string, epoch uint64, mask uint32, priv *ecdsa.PrivateKey) *pb.Note {
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

// ONLY FOR TESTING
func NewUnsignedNote(id string, epoch uint64, mask uint32) *pb.Note {
	n := &Note{
		id:        id,
		epoch:     epoch,
		mask:      mask,
		signature: &signature{},
	}

	return n.ToPbMsg()
}

// ONLY FOR TESTING
func signNote(n *Note, privKey *ecdsa.PrivateKey) error {
	if privKey == nil {
		return errNoPrivKey
	}

	noteMsg := &pb.Note{
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
