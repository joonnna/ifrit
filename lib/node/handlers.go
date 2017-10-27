package node

import (
	"crypto/x509"
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/joonnna/firechain/lib/protobuf"
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
	errNonExistingRing        = errors.New("Accusation specifies non exisiting ring")
	errDeactivatedRing        = errors.New("Accusation on deactivated ring")
	errNoMask                 = errors.New("No mask provided")
)

func (n *Node) Spread(ctx context.Context, args *gossip.GossipMsg) (*gossip.Partners, error) {
	var tlsInfo credentials.TLSInfo
	var ok bool

	reply := &gossip.Partners{}

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

	cert := tlsInfo.State.PeerCertificates[0]

	key := string(cert.SubjectKeyId[:])
	pId := newPeerId(cert.SubjectKeyId)

	if !n.shouldBeNeighbours(pId) {
		n.log.Err.Println(errNotMyNeighbour)
		n.log.Debug.Println(p.Addr)

		if existingPeer := n.getViewPeer(key); existingPeer == nil {
			p, err := newPeer(nil, cert, n.numRings)
			if err != nil {
				n.log.Err.Println(err)
				return nil, errInvalidPeerInformation
			}
			n.addViewPeer(p)
			n.addLivePeer(p)
		} else if !n.livePeerExist(key) {
			n.addLivePeer(existingPeer)
		}

		peerKeys := n.findNeighbours(pId)
		for _, k := range peerKeys {
			p := n.getLivePeer(k)
			if p != nil {
				c := &gossip.Certificate{Raw: p.cert.Raw}
				reply.Certificates = append(reply.Certificates, c)
			}
		}
		n.log.Debug.Println(len(reply.Certificates))
		return reply, nil
	}

	n.mergeCertificates(args.GetCertificates(), cert)
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

func (n *Node) mergeCertificates(certs []*gossip.Certificate, remoteCert *x509.Certificate) {
	n.evalCertificate(remoteCert)

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
	var accuserPeer *peer

	sign := a.GetSignature()
	epoch := a.GetEpoch()
	accuser := a.GetAccuser()
	accused := a.GetAccused()
	mask := a.GetMask()
	ringNum := a.GetRingNum()

	accusedKey := string(accused[:])
	accuserKey := string(accuser[:])

	if n.peerId.equal(&peerId{id: accused}) {
		n.setEpoch((epoch + 1))
		n.getProtocol().Rebuttal(n)
		return
	}

	p := n.getLivePeer(accusedKey)
	if p == nil || p.getNote() == nil {
		return
	}

	if accuserKey == n.key {
		accuserPeer = n.peer
	} else {
		accuserPeer = n.getLivePeer(accuserKey)
		if accuserPeer == nil || accuserPeer.getNote() == nil {
			return
		}
	}

	tmp := &gossip.Accusation{
		Epoch:   epoch,
		Accuser: accuser,
		Accused: accused,
		Mask:    mask,
		RingNum: ringNum,
	}

	b, err := proto.Marshal(tmp)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	valid, err := validateSignature(sign.GetR(), sign.GetS(), b, accuserPeer.publicKey)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	if !valid {
		n.log.Debug.Println("Accuser: ", accuserPeer.addr)
		n.log.Debug.Println("Accused: ", p.addr)
		n.log.Info.Println("Invalid signature on accusation, ignoring")
		return
	}

	peerNote := p.getNote()
	if epoch == peerNote.epoch {
		acc := p.getRingAccusation(ringNum)
		if acc != nil && accuserPeer.peerId.equal(acc.peerId) && acc.epoch == epoch {
			n.log.Info.Println("Already have accusation, discard")
			return
		}

		/*
			err := validMask(mask, ringNum)
			if err != nil {
				n.log.Err.Println(err)
				return
			}
		*/
		if !n.isPrev(p, accuserPeer, ringNum) {
			n.log.Err.Println("Accuser is not pre-decessor of accused on given ring, invalid accusation")
			return
		}

		a := &accusation{
			peerId:  p.peerId,
			epoch:   epoch,
			accuser: accuserPeer.peerId,
			mask:    mask,
			signature: &signature{
				r: sign.GetR(),
				s: sign.GetS(),
			},
			ringNum: ringNum,
		}

		err = p.setAccusation(a)
		if err != nil {
			n.log.Err.Println(err)
			return
		}
		n.log.Debug.Printf("Added accusation for: %s on ring %d", p.addr, a.ringNum)

		if !n.timerExist(accusedKey) {
			n.log.Debug.Println("Started timer for: ", p.addr)
			n.startTimer(p.key, p.recentNote, accuserPeer, p.addr)
		}
	}
}

func (n *Node) evalNote(gossipNote *gossip.Note) {
	epoch := gossipNote.GetEpoch()
	sign := gossipNote.GetSignature()
	id := gossipNote.GetId()
	mask := gossipNote.GetMask()

	peerKey := string(id[:])

	p := n.getViewPeer(peerKey)
	if p == nil {
		return
	}

	tmp := &gossip.Note{
		Epoch: epoch,
		Mask:  mask,
		Id:    id,
	}

	b, err := proto.Marshal(tmp)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	valid, err := validateSignature(sign.GetR(), sign.GetS(), b, p.publicKey)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	if !valid {
		n.log.Info.Println("Invalid signature on note, ignoring, ", p.addr)
		return
	}

	peerAccuse := p.getAnyAccusation()
	//Not accused, only need to check if newnote is more recent
	if peerAccuse == nil {
		currNote := p.getNote()

		//Want to store the most recent note
		if currNote == nil || currNote.epoch < epoch {
			newNote := &note{
				mask:   mask,
				peerId: p.peerId,
				epoch:  epoch,
				signature: &signature{
					r: sign.GetR(),
					s: sign.GetS(),
				},
			}
			p.setNote(newNote)
			if n.getLivePeer(peerKey) == nil {
				n.addLivePeer(p)
			}
		}
	} else {
		//Peer is accused, need to check if this note invalidates accusation
		if peerAccuse.epoch < epoch {
			n.log.Info.Println("Rebuttal received for:", p.addr)
			n.deleteTimeout(peerKey)
			newNote := &note{
				mask:   mask,
				peerId: p.peerId,
				epoch:  epoch,
				signature: &signature{
					r: sign.GetR(),
					s: sign.GetS(),
				},
			}
			p.setNote(newNote)
			p.removeAccusations()

			if n.getLivePeer(peerKey) == nil {
				n.addLivePeer(p)
			}
		}
	}
}

func (n *Node) evalCertificate(cert *x509.Certificate) {
	if len(cert.Subject.Locality) == 0 {
		return
	}

	if len(cert.SubjectKeyId) == 0 || n.peerId.equal(&peerId{id: cert.SubjectKeyId}) {
		return
	}

	peerKey := string(cert.SubjectKeyId[:])

	p := n.getViewPeer(peerKey)
	if p == nil {
		p, err := newPeer(nil, cert, n.numRings)
		if err != nil {
			n.log.Err.Println(err)
			return
		}
		n.addViewPeer(p)
	}
}

func validMask(mask []byte, idx uint32) error {
	if mask == nil {
		return errNoMask
	}

	//Ringnumbers start on 1
	maskIdx := idx - 1
	if maskIdx >= (uint32(len(mask))-1) || maskIdx < 0 {
		return errNonExistingRing
	}

	if mask[maskIdx] == 0 {
		return errDeactivatedRing
	}

	return nil
}
