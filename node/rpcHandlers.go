package node

import (
	"crypto/x509"

	"github.com/joonnna/capstone/protobuf"
	"golang.org/x/net/context"
)

//TODO accusations and notes
func (n *Node) Spread(ctx context.Context, args *gossip.GossipMsg) (*gossip.Empty, error) {
	reply := &gossip.Empty{}

	n.mergeCertificates(args.GetCertificates())
	n.mergeNotes(args.GetNotes())
	n.mergeAccusations(args.GetAccusations())

	return reply, nil
}

func (n *Node) Monitor(ctx context.Context, args *gossip.Ping) (*gossip.Pong, error) {
	reply := &gossip.Pong{}

	return reply, nil
}

func (n *Node) mergeNotes(notes []*gossip.Note) {
	for _, newNote := range notes {
		//TODO kinda ugly, maybe just pass []byte instead?
		if n.peerId.equal(&peerId{id: newNote.GetId()}) {
			continue
		}
		n.evalNote(newNote)
	}
}

func (n *Node) mergeAccusations(accusations []*gossip.Accusation) {
	for _, acc := range accusations {
		n.evalAccusation(acc)
	}
}

func (n *Node) mergeCertificates(certs []*gossip.Certificate) {
	for _, b := range certs {
		cert, err := x509.ParseCertificate(b.GetRaw())
		if err != nil {
			n.log.Err.Println(err)
			continue
		}
		n.evalCertificate(cert)
	}
}

func (n *Node) evalAccusation(a *gossip.Accusation) {
	signature := a.GetSignature()
	epoch := a.GetEpoch()
	accuser := a.GetAccuser()
	accused := a.GetAccused()

	accusedKey := string(accused[:])
	accuserKey := string(accuser[:])

	//Rebuttal, TODO only gossip own note, not all?
	if n.peerId.equal(&peerId{id: accused}) {
		n.setEpoch((epoch + 1))
		n.getProtocol().Gossip(n)
		return
	}

	p := n.getViewPeer(accusedKey)
	if p != nil {
		peerAccusation := p.getAccusation()
		if peerAccusation == nil || peerAccusation.epoch < epoch {
			accuserPeer := n.getViewPeer(accuserKey)
			if accuserPeer == nil {
				return
			}

			newAcc, err := newAccusation(p.peerId, epoch, signature, accuserPeer.peerId)
			if err != nil {
				n.log.Err.Println(err)
				return
			}

			err = p.setAccusation(newAcc)
			if err != nil {
				n.log.Err.Println(err)
				return
			}
			n.log.Debug.Println("Added accusation for: ", p.addr)

			if !n.timerExist(accusedKey) {
				//TODO fix mask shit
				newNote, err := createNote(p.peerId, epoch, signature, "")
				if err != nil {
					n.log.Err.Println(err)
					return
				}
				n.log.Debug.Println("Started timer for: ", p.addr)
				n.startTimer(p.key, newNote, accuserPeer)
			}
		}
	}
}

func (n *Node) evalNote(gossipNote *gossip.Note) {
	epoch := gossipNote.GetEpoch()
	mask := gossipNote.GetMask()
	signature := gossipNote.GetSignature()
	id := gossipNote.GetId()

	peerKey := string(id[:])

	p := n.getViewPeer(peerKey)

	//No certificate received for this peer, ignore
	if p != nil {
		peerAccuse := p.getAccusation()

		//Not accused, only need to check if newnote is more recent
		if peerAccuse == nil {
			currNote := p.getNote()

			//Want to store the most recent note
			if currNote == nil || currNote.epoch < epoch {
				newNote, err := createNote(p.peerId, epoch, signature, mask)
				if err != nil {
					n.log.Err.Println(err)
					return
				}
				p.setNote(newNote)
			}
		} else {
			//Peer is accused, need to check if this note invalidates accusation
			if peerAccuse.epoch < epoch {
				n.log.Info.Println("Rebuttal received for:", p.addr)
				p.removeAccusation()
				n.deleteTimeout(peerKey)
			}
		}
	}
}

func (n *Node) evalCertificate(cert *x509.Certificate) {
	if len(cert.Subject.Locality) == 0 || cert.Subject.Locality[0] == n.localAddr {
		return
	}

	if len(cert.SubjectKeyId) == 0 || n.peerId.equal(&peerId{id: cert.SubjectKeyId}) {
		return
	}

	peerKey := string(cert.SubjectKeyId[:])

	p := n.getViewPeer(peerKey)
	if p == nil {
		p, err := newPeer(nil, cert)
		if err != nil {
			n.log.Err.Println(err)
			return
		}
		n.addViewPeer(p)
		n.addLivePeer(p)
	}
}
