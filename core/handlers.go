package core

import (
	"crypto/sha256"
	"crypto/x509"
	"errors"

	"github.com/golang/protobuf/proto"
	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/core/discovery"
	pb "github.com/joonnna/ifrit/protobuf"
	"golang.org/x/net/context"
	"google.golang.org/grpc/credentials"
	grpcPeer "google.golang.org/grpc/peer"
)

var (
	errNoPeerInCtx            = errors.New("No peer information found in provided context.")
	errNoTLSInfo              = errors.New("No TLS info provided in peer context.")
	errNeighbourPeerNotFound  = errors.New("Neighbour peer was not found.")
	errNotMyNeighbour         = errors.New("Invalid gossip partner, not my neighbour.")
	errInvalidPeerInformation = errors.New("Could not create local peer representation.")
	errInvalidMaskLength      = errors.New("Mask is of invalid length.")

	errAccAlreadyExists      = errors.New("Accusation already existed, discarding.")
	errDisabledRing          = errors.New("Ring associated with accusation was disabled.")
	errInvalidAccuser        = errors.New("Accuser is not predecessor of accused on given ring, invalid accusation.")
	errInvalidSignature      = errors.New("Signature was invalid.")
	errInvalidSelfAccusation = errors.New("Received accusation about myself, but it was invalid.")
	errInvalidEpoch          = errors.New("Accusation epoch did not match note epoch.")

	errInvalidMask = errors.New("Note contained invalid mask")
	errOldNote     = errors.New("Already had the same or a more recent note")
	errNoPeer      = errors.New("Peer associated with note not found in full view.")

	errNilCert   = errors.New("Certificate was nil.")
	errSelfCert  = errors.New("Certificate was my own.")
	errNoCert    = errors.New("No certificate present in tls context.")
	errInvalidId = errors.New("Id in certificate is of invalid size.")
)

func (n *Node) Spread(ctx context.Context, args *pb.State) (*pb.StateResponse, error) {
	var observed bool
	cert, err := n.validateCtx(ctx)
	if err != nil {
		return nil, err
	}

	reply := &pb.StateResponse{}

	remoteId := string(cert.SubjectKeyId[:])
	peer := n.view.Peer(remoteId)
	if peer != nil {
		observed = true
	}

	if neighbours := n.view.ShouldBeNeighbour(remoteId); neighbours {
		if err := n.evalCertificate(cert); err != nil {
			log.Error(err.Error())
			return nil, err
		}

		err := n.evalNote(args.GetOwnNote())
		if err != nil && err != errOldNote {
			log.Debug(err.Error())
		}

		extGossip := args.GetExternalGossip()
		hosts := args.GetExistingHosts()

		// If hosts is nil gossip message was only a rebuttal,
		// no need to merge views.
		if hosts != nil {
			n.mergeViews(hosts, reply)
		}

		if handler := n.getGossipHandler(); handler != nil && extGossip != nil {
			reply.ExternalGossip, err = handler(extGossip)
			if err != nil {
				log.Error(err.Error())
			}
		}
	} else if observed {
		if !peer.IsAccused() {
			err := n.evalNote(args.GetOwnNote())
			if err != nil {
				log.Debug(err.Error())
			}
			return nil, errNotMyNeighbour
		}

		err := n.evalNote(args.GetOwnNote())
		if err != nil {
			log.Debug(err.Error())
		}

		for _, a := range peer.AllAccusations() {
			reply.Accusations = append(reply.Accusations, a.ToPbMsg())
		}
	} else {
		if err := n.evalCertificate(cert); err != nil {
			log.Error(err.Error())
			return nil, err
		}

		err := n.evalNote(args.GetOwnNote())
		if err != nil {
			log.Debug(err.Error())
		}

		// Help new peer integrate into the network
		reply.Certificates = append(reply.Certificates,
			&pb.Certificate{Raw: n.cm.Certificate().Raw})
		reply.Notes = append(reply.Notes, n.self.Note().ToPbMsg())

		for _, p := range n.view.FindNeighbours(remoteId) {
			reply.Certificates = append(reply.Certificates,
				&pb.Certificate{Raw: p.Certificate()})
			if note := p.Note(); note != nil {
				reply.Notes = append(reply.Notes, note.ToPbMsg())
			}
		}
	}

	return reply, nil
}

func (n *Node) Messenger(ctx context.Context, args *pb.Msg) (*pb.MsgResponse, error) {
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

	return &pb.MsgResponse{Content: replyContent}, nil
}

func (n *Node) mergeViews(given map[string]uint64, reply *pb.StateResponse) {
	for _, p := range n.view.Full() {
		if _, ok := given[p.Id]; !ok {
			reply.Certificates = append(reply.Certificates,
				&pb.Certificate{Raw: p.Certificate()})

			if note := p.Note(); note != nil {
				reply.Notes = append(reply.Notes, note.ToPbMsg())
			}
		} else if note := p.Note(); note != nil && note.IsMoreRecent(given[p.Id]) {
			reply.Notes = append(reply.Notes, note.ToPbMsg())
		}

		// No solution yet to avoid transferring all accusations.
		// Transferring all notes are avoided by checking epoch numbers.
		accs := p.AllAccusations()
		for _, a := range accs {
			reply.Accusations = append(reply.Accusations, a.ToPbMsg())
		}
	}

	localNote := n.self.Note()

	if epoch, exists := given[n.self.Id]; !exists || localNote.IsMoreRecent(epoch) {
		reply.Notes = append(reply.Notes, localNote.ToPbMsg())
	}
}

func (n *Node) mergeNotes(notes []*pb.Note) {
	if notes == nil {
		return
	}

	for _, newNote := range notes {
		if n.self.Id == string(newNote.GetId()) {
			continue
		}

		err := n.evalNote(newNote)
		if err != nil {
			log.Debug(err.Error())
		}
	}
}

func (n *Node) mergeAccusations(accusations []*pb.Accusation) {
	if accusations == nil {
		return
	}

	for _, acc := range accusations {
		accId := string(acc.GetAccused())
		accuserId := string(acc.GetAccuser())

		accuser := n.view.Peer(accuserId)
		if accuserId == n.self.Id {
			accuser = n.self
		} else if accuser == nil {
			continue
		}

		accused := n.view.Peer(accId)
		if accId == n.self.Id {
			accused = n.self
		} else if accused == nil || accused.Note() == nil {
			continue
		}

		err := n.evalAccusation(acc, accuser, accused)
		if err != nil {
			log.Debug(err.Error(), "ringNum", acc.GetRingNum(), "epoch", acc.GetEpoch(), "accused", accused.Addr, "accuser", accuser.Addr)
		}
	}
}

func (n *Node) mergeCertificates(certs []*pb.Certificate) {
	if certs == nil {
		return
	}
	for _, b := range certs {
		cert, err := x509.ParseCertificate(b.GetRaw())
		if err != nil {
			log.Error(err.Error())
			continue
		}

		if n.self.Id == string(cert.SubjectKeyId) {
			continue
		}

		err = n.evalCertificate(cert)
		if err != nil {
			log.Debug(err.Error())
		}
	}
}

func (n *Node) evalAccusation(a *pb.Accusation, accuserPeer, p *discovery.Peer) error {
	sign := a.GetSignature()
	if sign == nil {
		return errInvalidSignature
	}

	r := sign.GetR()
	s := sign.GetS()

	epoch := a.GetEpoch()
	ringNum := a.GetRingNum()

	a.Signature = nil
	bytes, err := proto.Marshal(a)
	if err != nil {
		return err
	}

	if n.self.Id == p.Id {
		if isPrev := n.view.ValidAccuser(n.self, accuserPeer, ringNum); !isPrev {
			return errInvalidAccuser
		}

		if valid := n.cs.Verify(bytes, r, s, accuserPeer.PublicKey()); !valid {
			return errInvalidSignature
		}

		if rebut := n.view.ShouldRebuttal(epoch, ringNum); rebut {
			n.protocol().Rebuttal(n)
			return nil
		} else {
			return errInvalidSelfAccusation
		}
	}

	acc := p.RingAccusation(ringNum)
	if acc != nil && acc.Equal(p.Id, accuserPeer.Id, ringNum, epoch) {
		live := n.view.IsAlive(p.Id)
		if exists := n.view.HasTimer(p.Id); !exists && live {
			n.view.StartTimer(p, p.Note(), accuserPeer)
			log.Debug("Had accusation with no timer, starting timer.")
		}
		return errAccAlreadyExists
	}

	if note := p.Note(); note != nil && note.Equal(epoch) {
		if disabled := note.IsRingDisabled(ringNum, n.view.NumRings()); disabled {
			return errDisabledRing
		}

		if valid := n.view.ValidAccuser(p, accuserPeer, ringNum); !valid {
			return errInvalidAccuser
		}

		if valid := n.cs.Verify(bytes, r, s, accuserPeer.PublicKey()); !valid {
			return errInvalidSignature
		}

		err := p.AddAccusation(p.Id, accuserPeer.Id, epoch, ringNum, sign.GetR(), sign.GetS())
		if err != nil {
			return err
		}

		live := n.view.IsAlive(p.Id)
		if exists := n.view.HasTimer(p.Id); !exists && live {
			n.view.StartTimer(p, p.Note(), accuserPeer)
		}
	} else {
		return errInvalidEpoch
	}

	return nil
}

func (n *Node) evalNote(newNote *pb.Note) error {
	epoch := newNote.GetEpoch()
	mask := newNote.GetMask()

	sign := newNote.GetSignature()
	if sign == nil {
		return errInvalidSignature
	}

	r := sign.GetR()
	s := sign.GetS()

	p := n.view.Peer(string(newNote.GetId()))
	if p == nil {
		return errNoPeer
	}

	note := p.Note()

	if note != nil && !note.IsMoreRecent(epoch) {
		return errOldNote
	}

	if valid := n.view.ValidMask(mask); !valid {
		return errInvalidMask
	}

	newNote.Signature = nil
	bytes, err := proto.Marshal(newNote)
	if err != nil {
		return err
	}

	accusations := p.AllAccusations()
	// Not accused, only need to check if newnote is more recent
	if numAccs := len(accusations); numAccs == 0 {
		// Want to store the most recent note
		if note == nil || note.IsMoreRecent(epoch) {
			if valid := n.cs.Verify(bytes, r, s, p.PublicKey()); !valid {
				return errInvalidSignature
			}

			p.AddNote(mask, epoch, r, s)

			if alive := n.view.IsAlive(p.Id); !alive {
				n.view.AddLive(p)
			}
		}
	} else {
		if valid := n.cs.Verify(bytes, r, s, p.PublicKey()); !valid {
			return errInvalidSignature
		}

		// Peer is accused, need to check if this note invalidates any accusations.
		for _, a := range accusations {
			if a.IsMoreRecent(epoch) {
				p.RemoveAccusation(a)
			}
		}

		if note == nil || note.IsMoreRecent(epoch) {
			p.AddNote(mask, epoch, r, s)
		}

		// All accusations has to be invalidated before we add peer back to full view.
		if accused := p.IsAccused(); !accused {
			p.ResetPing()

			if exists := n.view.HasTimer(p.Id); exists {
				n.view.DeleteTimeout(p.Id)
			}

			if alive := n.view.IsAlive(p.Id); !alive {
				n.view.AddLive(p)
			}

			log.Debug("Rebuttal received", "epoch", epoch, "addr", p.Addr)
		}
	}

	return nil
}

func (n *Node) evalCertificate(cert *x509.Certificate) error {
	if cert == nil {
		return errNilCert
	}

	id := string(cert.SubjectKeyId)

	if n.self.Id == id {
		return errSelfCert
	}

	if len(id) != sha256.Size {
		return errInvalidId
	}

	if caCert := n.cm.CaCertificate(); caCert != nil {
		err := cert.CheckSignatureFrom(caCert)
		if err != nil {
			return err
		}
	} else {
		err := cert.CheckSignatureFrom(cert)
		if err != nil {
			return err
		}
	}

	if exists := n.view.Exists(id); !exists {
		n.view.AddFull(id, cert)
	}

	return nil
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

func hashContent(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}
