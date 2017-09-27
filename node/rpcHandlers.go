package node

import (
	"crypto/x509"

	"github.com/joonnna/capstone/protobuf"
	"golang.org/x/net/context"
)

//TODO accusations and notes
func (n *Node) Spread(ctx context.Context, args *gossip.GossipMsg) (*gossip.Empty, error) {
	reply := &gossip.Empty{}

	n.mergeCertificates(args.GetCerts())
	n.mergeNotes(args.GetNotes())
	n.mergeAccusations(args.GetAccusations())

	return reply, nil
}

func (n *Node) Monitor(ctx context.Context, args *gossip.Ping) (*gossip.Pong, error) {
	reply := &gossip.Pong{
		LocalAddr: n.localAddr,
	}

	return reply, nil
}

func (n *Node) mergeNotes(notes map[string]*gossip.Note) {
	for _, newNote := range notes {
		if newNote.GetAddr() == n.localAddr {
			continue
		}
		n.evalNote(newNote)
	}
}

func (n *Node) mergeAccusations(accusations map[string]*gossip.Accusation) {
	for _, acc := range accusations {
		n.evalAccusation(acc)
	}
}

func (n *Node) mergeCertificates(certs map[string]*gossip.Certificate) {
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
	note := a.GetRecentNote()
	accuser := a.GetAccuser()
	accused := note.GetAddr()
	newEpoch := note.GetEpoch()
	mask := note.GetMask()

	//Rebuttal, TODO only gossip own note, not all?
	if accused == n.localAddr {
		n.setEpoch((newEpoch + 1))
		n.getProtocol().Gossip(n)
		return
	}

	p := n.getViewPeer(accused)
	if p != nil {
		peerAccuse := p.getAccusation()
		if peerAccuse == nil || peerAccuse.recentNote.isMoreRecent(newEpoch) {
			newNote := createNote(p.addr, newEpoch, mask)

			n.log.Debug.Println("Added accusation for: ", p.addr)
			p.setAccusation(accuser, newNote)

			if !n.timerExist(p.addr) {
				n.log.Debug.Println("Started timer for: ", p.addr)
				n.startTimer(p.addr, newNote, accuser)
			}
		}
	}
}

func (n *Node) evalNote(gossipNote *gossip.Note) {
	newEpoch := gossipNote.GetEpoch()
	addr := gossipNote.GetAddr()
	mask := gossipNote.GetMask()

	newNote := createNote(addr, newEpoch, mask)

	p := n.getViewPeer(addr)

	//No certificate received for this peer, ignore
	if p == nil {
		return
	} else {
		peerAccuse := p.getAccusation()

		//Not accused, only check if newnote is more recent
		if peerAccuse == nil {
			currNote := p.getNote()

			//Want to store the most recent note
			if currNote == nil || currNote.isMoreRecent(newNote.epoch) {
				p.setNote(newNote)
			}
		} else {
			accuseNote := peerAccuse.getNote()

			//Peer is accused, need to check if this note invalidates accusation
			if accuseNote.isMoreRecent(newNote.epoch) {
				n.log.Info.Println("Rebuttal received for:", p.addr)
				p.removeAccusation()
				n.deleteTimeout(p.addr)
			}
		}
	}
}

func (n *Node) evalCertificate(cert *x509.Certificate) {
	if len(cert.Subject.Locality) < 1 {
		return
	}
	addr := cert.Subject.Locality[0]
	if addr == n.localAddr {
		return
	}

	p := n.getViewPeer(addr)
	if p == nil {
		p := newPeer(nil, cert)
		n.addViewPeer(p)
		n.addLivePeer(p)
	}
}
