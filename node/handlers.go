package node

import (
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/joonnna/capstone/protobuf"
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

	if !n.shouldBeNeighbours(cert.SubjectKeyId) {
		n.log.Err.Println(errNotMyNeighbour)
		if !n.viewPeerExist(key) {
			p, err := newPeer(nil, cert)
			if err != nil {
				n.log.Err.Println(err)
				return nil, errInvalidPeerInformation
			}
			n.addViewPeer(p)
			n.addLivePeer(p)
		}

		peerKeys := n.findNeighbours(cert.SubjectKeyId)
		for _, k := range peerKeys {
			p := n.getLivePeer(k)
			if p != nil {
				c := &gossip.Certificate{Raw: p.cert.Raw}
				reply.Certificates = append(reply.Certificates, c)
			}
		}
		return reply, nil
	}

	if !n.viewPeerExist(key) {
		p, err := newPeer(nil, cert)
		if err != nil {
			n.log.Err.Println(err)
			return nil, errInvalidPeerInformation
		}
		n.addViewPeer(p)
		n.addLivePeer(p)
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
	}
	b := []byte(fmt.Sprintf("%v", tmp))

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
		if !n.isPrev(accuserPeer, p, mask) {
			n.log.Err.Println("Accuser is not pre-decessor of accused, invalid accusation")
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
		}

		err = p.setAccusation(a)
		if err != nil {
			n.log.Err.Println(err)
			return
		}
		n.log.Debug.Println("Added accusation for: ", p.addr)

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
	b := []byte(fmt.Sprintf("%v", tmp))
	valid, err := validateSignature(sign.GetR(), sign.GetS(), b, p.publicKey)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	if !valid {
		n.log.Info.Println("Invalid signature on note, ignoring")
		return
	}

	peerAccuse := p.getAccusation()
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
			p.removeAccusation()
			n.addLivePeer(p)
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
		p, err := newPeer(nil, cert)
		if err != nil {
			n.log.Err.Println(err)
			return
		}
		n.addViewPeer(p)
	}
}
