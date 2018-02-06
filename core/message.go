package core

import (
	"crypto/ecdsa"

	"github.com/golang/protobuf/proto"
	"github.com/joonnna/ifrit/protobuf"
)

func (n *note) sign(privKey *ecdsa.PrivateKey) error {
	noteMsg := &gossip.Note{
		Epoch: n.epoch,
		Id:    n.id,
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

func (n *note) signAndMarshal(privKey *ecdsa.PrivateKey) (*gossip.Note, error) {
	noteMsg := &gossip.Note{
		Epoch: n.epoch,
		Id:    n.id,
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
		Mask:    a.mask,
		RingNum: a.ringNum,
		Signature: &gossip.Signature{
			R: a.r,
			S: a.s,
		},
	}
}

func (a accusation) signAndMarshal(privKey *ecdsa.PrivateKey) (*gossip.Accusation, error) {
	acc := &gossip.Accusation{
		Epoch:   a.epoch,
		Accuser: a.accuser.id,
		Accused: a.id,
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

func (a *accusation) sign(privKey *ecdsa.PrivateKey) error {
	acc := &gossip.Accusation{
		Epoch:   a.epoch,
		Accuser: a.accuser.id,
		Accused: a.id,
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
