package node

import (
	"github.com/joonnna/capstone/protobuf"
)

func (n note) toPbMsg() *gossip.Note {
	return &gossip.Note{
		Epoch: n.epoch,
		Id:    n.id,
		Mask:  n.mask,
		Signature: &gossip.Signature{
			R: n.r,
			S: n.s,
		},
	}
}

func (a accusation) toPbMsg() *gossip.Accusation {
	return &gossip.Accusation{
		Epoch:   a.epoch,
		Accuser: a.accuser.id,
		Accused: a.id,
		Signature: &gossip.Signature{
			R: a.r,
			S: a.s,
		},
	}
}

/*
func PbToNote(n *gossip.Note) (*note, error) {
	return createNote(newPeerId(n.GetId()), n.GetEpoch(), n.GetMask())
}

func PbToAccusation(a *gossip.Accusation) (*accusation, error) {
	return newAccusation(a.GetAccused(), a.GetEpoch(), a.GetAccuser())
}
*/
