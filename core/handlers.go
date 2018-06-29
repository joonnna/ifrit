package core

import (
	"crypto/x509"
	"errors"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/core/discovery"
	"github.com/joonnna/ifrit/protobuf"
	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"
	grpcPeer "google.golang.org/grpc/peer"
)

var (
	errNoPeerInCtx            = errors.New("No peer information found in provided context")
	errNoTLSInfo              = errors.New("No TLS info provided in peer context")
	errNoCert                 = errors.New("No local certificate present in request")
	errNeighbourPeerNotFound  = errors.New("Neighbour peer was  not found")
	errNotMyNeighbour         = errors.New("Invalid gossip partner, not my neighbour")
	errInvalidPeerInformation = errors.New("Could not create local peer representation")
	errNoMask                 = errors.New("No mask provided")
	errInvalidMaskLength      = errors.New("Mask is of invalid length")
)

func (n *Node) Spread(ctx context.Context, args *gossip.State) (*gossip.StateResponse, error) {
	cert, err := n.validateCtx(ctx)
	if err != nil {
		return nil, err
	}

	reply := &gossip.StateResponse{}

	remoteId := string(cert.SubjectKeyId[:])
	neighbours := n.view.ShouldBeNeighbour(remoteId)

	peer := n.view.Peer(remoteId)

	if !neighbours && peer != nil && peer.IsAccused() {
		n.evalNote(args.GetOwnNote())

		for _, a := range peer.AllAccusations() {
			reply.Accusations = append(reply.Accusations, a.ToPbMsg())
		}
		return reply, nil
	}

	// Peers that have already been seen should be rejected
	if !neighbours && peer != nil {
		n.evalNote(args.GetOwnNote())

		return nil, errNotMyNeighbour
	}

	//Help new peer integrate into the network
	if !neighbours && peer == nil {
		if valid := n.evalCertificate(cert); !valid {
			return nil, errNoCert
		}

		n.evalNote(args.GetOwnNote())
		reply.Certificates = append(reply.Certificates, &gossip.Certificate{Raw: n.localCert.Raw})
		reply.Notes = append(reply.Notes, n.self.Note().ToPbMsg())

		for _, p := range n.view.FindNeighbours(remoteId) {
			reply.Certificates = append(reply.Certificates, &gossip.Certificate{Raw: p.Certificate()})
			if note := p.Note(); note != nil {
				reply.Notes = append(reply.Notes, note.ToPbMsg())
			}
		}

		return reply, nil
	}

	if valid := n.evalCertificate(cert); !valid {
		return nil, errNoCert
	}

	extGossip := args.GetExternalGossip()

	// Gossip message was only a rebuttal, no need to merge views
	if isRebuttal := n.evalNote(args.GetOwnNote()); isRebuttal && extGossip == nil {
		return reply, nil
	}

	n.mergeViews(args.GetExistingHosts(), reply)

	if handler := n.getGossipHandler(); handler != nil && extGossip != nil {
		reply.ExternalGossip, err = handler(extGossip)
		if err != nil {
			return nil, err
		}
	}

	return reply, nil
}

func (n *Node) Messenger(ctx context.Context, args *gossip.Msg) (*gossip.MsgResponse, error) {
	var replyContent []byte

	_, err := n.validateCtx(ctx)
	if err != nil {
		return nil, err
	}

	if handler := n.getMsgHandler(); handler != nil {
		replyContent, err = handler(args.GetContent())
		if err != nil {
			return nil, err
		}
	}

	return &gossip.MsgResponse{Content: replyContent}, nil
}

func (n *Node) mergeViews(given map[string]uint64, reply *gossip.StateResponse) {
	for _, p := range n.view.Full() {
		if _, exists := given[p.Id]; !exists {
			reply.Certificates = append(reply.Certificates, &gossip.Certificate{Raw: p.Certificate()})
			if note := p.Note(); note != nil {
				reply.Notes = append(reply.Notes, note.ToPbMsg())
			}
		} else if note := p.Note(); note != nil && note.IsMoreRecent(given[p.Id]) {
			reply.Notes = append(reply.Notes, note.ToPbMsg())
		}

		// No solution yet to avoid transferring all accusations.
		// Transferring all notes are avoided by checking epoch numbers.
		accs := p.AllAccusations()
		reply.Accusations = make([]*gossip.Accusation, 0, len(accs))
		for _, a := range accs {
			reply.Accusations = append(reply.Accusations, a.ToPbMsg())
		}
	}

	localNote := n.self.Note()

	if epoch, exists := given[n.self.Id]; !exists || localNote.IsMoreRecent(epoch) {
		reply.Notes = append(reply.Notes, localNote.ToPbMsg())
	}
}

func (n *Node) mergeNotes(notes []*gossip.Note) {
	if notes == nil {
		return
	}
	for _, newNote := range notes {
		if n.self.Id == string(newNote.GetId()) {
			continue
		}
		n.evalNote(newNote)
	}
}

func (n *Node) mergeAccusations(accusations []*gossip.Accusation) {
	if accusations == nil {
		return
	}

	for _, acc := range accusations {
		n.evalAccusation(acc)
	}
}

func (n *Node) mergeCertificates(certs []*gossip.Certificate) {
	if certs == nil {
		return
	}
	for _, b := range certs {
		cert, err := x509.ParseCertificate(b.GetRaw())
		if err != nil {
			log.Error(err.Error())
			continue
		}
		n.evalCertificate(cert)
	}
}

func (n *Node) evalAccusation(a *gossip.Accusation) {
	var accuserPeer *discovery.Peer

	sign := a.GetSignature()
	epoch := a.GetEpoch()
	mask := a.GetMask()
	ringNum := a.GetRingNum()

	accusedId := string(a.GetAccused())
	accuserId := string(a.GetAccuser())

	if n.self.Id == accusedId {
		if rebut := n.view.ShouldRebuttal(epoch, ringNum); rebut {
			n.getProtocol().Rebuttal(n)
		}
		return
	}

	p := n.view.LivePeer(accusedId)
	if p == nil || p.Note() == nil {
		return
	}

	if accuserId == n.self.Id {
		accuserPeer = n.self
	} else {
		accuserPeer = n.view.LivePeer(accuserId)
		if accuserPeer == nil || accuserPeer.Note() == nil {
			return
		}
	}

	acc := p.RingAccusation(ringNum)

	if acc != nil && acc.Equal(p.Id, accuserPeer.Id, ringNum, epoch) {
		log.Debug("Already have accusation, discard")
		if exists := n.view.HasTimer(accusedId); !exists {
			n.view.StartTimer(p, p.Note(), accuserPeer)
		}
		return
	}

	if note := p.Note(); note != nil && note.Equal(epoch) {
		if valid := n.view.ValidMask(mask); !valid {
			return
		}

		if disabled := n.view.IsRingDisabled(mask, ringNum); disabled {
			return
		}

		if valid := n.view.ValidAccuser(p.Id, accuserPeer.Id, ringNum); !valid {
			log.Error("Accuser is not predecessor of accused on given ring, invalid accusation", "ringNum", ringNum, "accused", p.Addr, "accuser", accuserPeer.Addr)

			return
		}

		if valid := checkAccusationSignature(a, accuserPeer); !valid {
			return
		}

		err := p.AddAccusation(p.Id, accuserPeer.Id, epoch, mask, ringNum, sign.GetR(), sign.GetS())
		if err != nil {
			log.Error(err.Error())
			return
		}

		if exists := n.view.HasTimer(p.Id); !exists {
			n.view.StartTimer(p, p.Note(), accuserPeer)
		}
	}
}

func (n *Node) evalNote(gossipNote *gossip.Note) bool {
	var haveRebuted bool

	if gossipNote == nil {
		log.Debug("Got nil note")
		return false
	}

	epoch := gossipNote.GetEpoch()
	mask := gossipNote.GetMask()

	p := n.view.Peer(string(gossipNote.GetId()))
	if p == nil {
		return false
	}

	note := p.Note()

	if note != nil && note.Equal(epoch) {
		return false
	}

	if valid := n.view.ValidMask(mask); !valid {
		return false
	}

	accusations := p.AllAccusations()
	// Not accused, only need to check if newnote is more recent
	if numAccs := len(accusations); numAccs == 0 {
		// Want to store the most recent note
		if note == nil || note.IsMoreRecent(epoch) {
			if valid := checkNoteSignature(gossipNote, p); !valid {
				return false
			}

			sign := gossipNote.GetSignature()

			p.AddNote(mask, epoch, sign.GetR(), sign.GetS())

			if alive := n.view.IsAlive(p.Id); !alive {
				n.view.AddLive(p)
			}
		}
	} else {
		// Peer is accused, need to check if this note invalidates any accusations.
		for _, a := range accusations {
			if a.IsMoreRecent(epoch) {
				// We do not repeat all rebuttal operations for each accusation.
				// E.g only check signature once, add one new note, only delete timeout once,
				// etc.
				if !haveRebuted {
					if valid := checkNoteSignature(gossipNote, p); !valid {
						continue
					}

					sign := gossipNote.GetSignature()

					log.Debug("Rebuttal received", "addr", p.Addr)

					n.view.DeleteTimeout(p.Id)
					p.AddNote(mask, epoch, sign.GetR(), sign.GetS())
					p.ResetPing()

					if alive := n.view.IsAlive(p.Id); !alive {
						n.view.AddLive(p)
					}

					haveRebuted = true
				}
				p.RemoveAccusation(a)
			}
		}

		// All accusations has to be invalidated before we add peer back to full view.
		if accused := p.IsAccused(); !accused {
			n.view.AddLive(p)
		}

	}

	return haveRebuted
}

func (n *Node) evalCertificate(cert *x509.Certificate) bool {
	if cert == nil {
		log.Error("Got nil cert")
		return false
	}

	id := string(cert.SubjectKeyId)

	if n.self.Id == id {
		return false
	}

	err := checkCertificateSignature(cert, n.caCert)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	if exists := n.view.Exists(id); !exists {
		n.view.AddFull(id, cert)
	}

	return true
}

func (n *Node) validateCtx(ctx context.Context) (*x509.Certificate, error) {
	var tlsInfo credentials.TLSInfo
	var ok bool

	p, ok := grpcPeer.FromContext(ctx)
	if !ok {
		return nil, errNoPeerInCtx
	}

	if tlsInfo, ok = p.AuthInfo.(credentials.TLSInfo); !ok {
		return nil, errNoTLSInfo
	}

	if len(tlsInfo.State.PeerCertificates) < 1 {
		return nil, errNoCert
	}

	return tlsInfo.State.PeerCertificates[0], nil
}
