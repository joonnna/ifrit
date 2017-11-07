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
	errNoPeerInCtx             = errors.New("No peer information found in provided context")
	errNoTLSInfo               = errors.New("No TLS info provided in peer context")
	errNoCert                  = errors.New("No local certificate present in request")
	errNeighbourPeerNotFound   = errors.New("Neighbour peer was  not found")
	errNotMyNeighbour          = errors.New("Invalid gossip partner, not my neighbour")
	errInvalidPeerInformation  = errors.New("Could not create local peer representation")
	errNonExistingRing         = errors.New("Accusation specifies non exisiting ring")
	errDeactivatedRing         = errors.New("Accusation on deactivated ring")
	errNoMask                  = errors.New("No mask provided")
	errTooManyDeactivatedRings = errors.New("Mask contains too many deactivated rings")
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

	if !n.shouldBeNeighbours(pId) && !n.viewPeerExist(key) {
		n.log.Err.Println(errNotMyNeighbour)
		n.log.Debug.Println(p.Addr)

		if valid := n.evalCertificate(cert); !valid {
			return nil, errNoCert
		}

		n.evalNote(args.GetOwnNote())

		peerKeys := n.findNeighbours(pId)
		for _, k := range peerKeys {
			if k == n.key {
				c := &gossip.Certificate{Raw: n.localCert.Raw}
				reply.Certificates = append(reply.Certificates, c)
				reply.Notes = append(reply.Notes, n.localNoteToPbMsg())
				continue
			}

			p := n.getLivePeer(k)
			if p != nil {
				c := &gossip.Certificate{Raw: p.cert.Raw}
				reply.Certificates = append(reply.Certificates, c)
				if p.getNote() != nil {
					reply.Notes = append(reply.Notes, p.getNote().toPbMsg())
				}
			}
		}
		n.log.Debug.Println(len(reply.Certificates))

		if record := n.getRecordFlag(); record {
			n.incrementRequestCounter()
		}

		return reply, nil
	}

	if valid := n.evalCertificate(cert); !valid {
		return nil, errNoCert
	}

	n.evalNote(args.GetOwnNote())
	n.mergeCertificates(args.GetCertificates())
	n.mergeNotes(args.GetNotes())
	n.mergeAccusations(args.GetAccusations())

	if record := n.getRecordFlag(); record {
		n.incrementRequestCounter()
	}

	return reply, nil
}

func (n *Node) Monitor(ctx context.Context, args *gossip.Ping) (*gossip.Pong, error) {
	reply := &gossip.Pong{}

	return reply, nil
}

func (n *Node) mergeNotes(notes []*gossip.Note) {
	if notes == nil {
		return
	}
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
	if certs == nil {
		return
	}
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
		n.deactivateRing(ringNum)
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

	acc := p.getRingAccusation(ringNum)

	newAcc := &accusation{
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

	if newAcc.equal(acc) {
		n.log.Info.Println("Already have accusation, discard")
		if !n.timerExist(accusedKey) {
			n.log.Debug.Println("Started timer for: ", p.addr)
			n.startTimer(p.key, p.getNote(), accuserPeer, p.addr)
		}
		return
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
		err := validMask(mask, ringNum, n.maxByz)
		if err != nil {
			n.log.Err.Println(err)
			return
		}

		if !n.isPrev(p, accuserPeer, ringNum) {
			n.log.Err.Println("Accuser is not pre-decessor of accused on given ring, invalid accusation")
			return
		}

		err = p.setAccusation(newAcc)
		if err != nil {
			n.log.Err.Println(err)
			return
		}
		n.log.Debug.Printf("Added accusation for: %s on ring %d", p.addr, newAcc.ringNum)

		if !n.timerExist(accusedKey) {
			n.log.Debug.Println("Started timer for: ", p.addr)
			n.startTimer(p.key, p.recentNote, accuserPeer, p.addr)
		}
	}
}

func (n *Node) evalNote(gossipNote *gossip.Note) {
	if gossipNote == nil {
		n.log.Err.Println("Got nil note")
		return
	}

	epoch := gossipNote.GetEpoch()
	sign := gossipNote.GetSignature()
	id := gossipNote.GetId()
	mask := gossipNote.GetMask()

	peerKey := string(id[:])

	p := n.getViewPeer(peerKey)
	if p == nil {
		return
	}

	//Skip valdiating signature etc if we already have the note
	//If we already have a more recent note or the same,
	//this one won't change anything
	currNote := p.getNote()
	if currNote != nil && currNote.epoch >= epoch {
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
			if !n.livePeerExist(peerKey) {
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

			if !n.livePeerExist(peerKey) {
				n.addLivePeer(p)
			}
		}
	}
}

func (n *Node) evalCertificate(cert *x509.Certificate) bool {
	if cert == nil {
		n.log.Err.Println("Got nil cert")
		return false
	}

	if len(cert.Subject.Locality) == 0 {
		return false
	}

	if len(cert.SubjectKeyId) == 0 || n.peerId.equal(&peerId{id: cert.SubjectKeyId}) {
		return false
	}

	err := cert.CheckSignatureFrom(n.caCert)
	if err != nil {
		n.log.Err.Println(err)
		return false
	}

	peerKey := string(cert.SubjectKeyId[:])

	if !n.viewPeerExist(peerKey) {
		n.addViewPeer(peerKey, nil, cert, n.numRings)
	}

	return true
}

func validMask(mask []byte, ringNum uint32, maxByz uint32) error {
	var deactivated uint32

	if mask == nil {
		return errNoMask
	}

	idx := ringNum - 1

	maxIdx := uint32(len(mask) - 1)

	if idx > maxIdx || idx < 0 {
		return errNonExistingRing
	}

	deactivated = 0
	for _, b := range mask {
		if b == 0 {
			deactivated++
		}
	}

	if deactivated > maxByz {
		return errTooManyDeactivatedRings
	}

	if mask[idx] == 0 {
		return errDeactivatedRing
	}

	return nil
}
