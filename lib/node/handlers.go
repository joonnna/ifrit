package node

import (
	"crypto/x509"
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/joonnna/ifrit/lib/protobuf"
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
	errInvalidMaskLength       = errors.New("Mask is of invalid length")
)

func (n *Node) Spread(ctx context.Context, args *gossip.State) (*gossip.StateResponse, error) {
	reply := &gossip.StateResponse{}

	cert, err := n.validateCtx(ctx)
	if err != nil {
		return nil, err
	}

	key := string(cert.SubjectKeyId[:])
	pId := newPeerId(cert.SubjectKeyId)

	neighbours := n.shouldBeNeighbours(pId)
	exist := n.viewPeerExist(key)

	//Peers that have already been seen should be rejected
	if !neighbours && exist {
		if record := n.stats.getRecordFlag(); record {
			n.stats.incrementFailed()
		}

		n.evalNote(args.GetOwnNote())

		return nil, errNotMyNeighbour
	}

	//Help new peer integrate into the network
	if !neighbours && !exist {
		if valid := n.evalCertificate(cert); !valid {
			return nil, errNoCert
		}

		n.evalNote(args.GetOwnNote())
		reply.Certificates = append(reply.Certificates, &gossip.Certificate{Raw: n.localCert.Raw})
		reply.Notes = append(reply.Notes, n.localNoteToPbMsg())

		peerKeys := n.findNeighbours(pId)
		for _, k := range peerKeys {
			if k == n.key {
				continue
			}

			p := n.getLivePeer(k)
			if p != nil {
				reply.Certificates = append(reply.Certificates, &gossip.Certificate{Raw: p.cert.Raw})
				if note := p.getNote(); note != nil {
					reply.Notes = append(reply.Notes, note.toPbMsg())
				}
			}
		}

		if record := n.stats.getRecordFlag(); record {
			n.stats.incrementCompleted()
		}

		return reply, nil
	}

	if valid := n.evalCertificate(cert); !valid {
		return nil, errNoCert
	}

	//Already evaluated the new note, can return an empty reply
	if isRebuttal := n.evalNote(args.GetOwnNote()); isRebuttal && args.GetExistingHosts() == nil {
		return reply, nil
	}

	n.mergeViews(args.GetExistingHosts(), reply)

	reply.ExternalGossip, err = n.msgHandler(args.GetExternalGossip())
	if err != nil {
		return nil, err
	}

	if record := n.stats.getRecordFlag(); record {
		n.stats.incrementCompleted()
	}

	return reply, nil
}

func (n *Node) Messenger(ctx context.Context, args *gossip.Msg) (*gossip.MsgResponse, error) {
	_, err := n.validateCtx(ctx)
	if err != nil {
		return nil, err
	}

	replyContent, err := n.msgHandler(args.GetContent())
	if err != nil {
		return nil, err
	}

	return &gossip.MsgResponse{Content: replyContent}, nil
}

func (n *Node) mergeViews(given map[string]uint64, reply *gossip.StateResponse) {
	for _, p := range n.getView() {
		if _, exists := given[p.key]; !exists {
			reply.Certificates = append(reply.Certificates, &gossip.Certificate{Raw: p.cert.Raw})
			if note := p.getNote(); note != nil {
				reply.Notes = append(reply.Notes, note.toPbMsg())
			}
		} else {
			if note := p.getNote(); note != nil && note.epoch > given[p.key] {
				reply.Notes = append(reply.Notes, note.toPbMsg())
			}
		}

		//No solution yet to avoid transferring all accusations
		//Transferring all notes are avoided by checking epoch numbers
		accs := p.getAllAccusations()
		for _, a := range accs {
			if a != nil {
				reply.Accusations = append(reply.Accusations, a.toPbMsg())
			}
		}
	}

	if _, exists := given[n.key]; !exists {
		reply.Notes = append(reply.Notes, n.localNoteToPbMsg())
	} else if given[n.key] < n.getEpoch() {
		reply.Notes = append(reply.Notes, n.localNoteToPbMsg())
	}

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
		err := checkDisabledRings(mask, ringNum, n.maxByz, n.numRings)
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

func (n *Node) evalNote(gossipNote *gossip.Note) bool {
	if gossipNote == nil {
		n.log.Err.Println("Got nil note")
		return false
	}

	epoch := gossipNote.GetEpoch()
	sign := gossipNote.GetSignature()
	id := gossipNote.GetId()
	mask := gossipNote.GetMask()

	peerKey := string(id[:])

	p := n.getViewPeer(peerKey)
	if p == nil {
		return false
	}

	//Skip valdiating signature etc if we already have the note
	//If we already have a more recent note or the same,
	//this one won't change anything
	currNote := p.getNote()
	if currNote != nil && currNote.epoch >= epoch {
		return false
	}

	tmp := &gossip.Note{
		Epoch: epoch,
		Mask:  mask,
		Id:    id,
	}

	b, err := proto.Marshal(tmp)
	if err != nil {
		n.log.Err.Println(err)
		return false
	}

	valid, err := validateSignature(sign.GetR(), sign.GetS(), b, p.publicKey)
	if err != nil {
		n.log.Err.Println(err)
		return false
	}

	if !valid {
		n.log.Info.Println("Invalid signature on note, ignoring, ", p.addr)
		return false
	}

	err = validMask(mask, n.maxByz, n.numRings)
	if err != nil {
		n.log.Err.Println(err)
		return false
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

			return true
		}
	}

	return false
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

func (n *Node) validateCtx(ctx context.Context) (*x509.Certificate, error) {
	var tlsInfo credentials.TLSInfo
	var ok bool

	p, ok := grpcPeer.FromContext(ctx)
	if !ok {
		if record := n.stats.getRecordFlag(); record {
			n.stats.incrementFailed()
		}
		return nil, errNoPeerInCtx
	}

	if tlsInfo, ok = p.AuthInfo.(credentials.TLSInfo); !ok {
		if record := n.stats.getRecordFlag(); record {
			n.stats.incrementFailed()
		}
		return nil, errNoTLSInfo
	}

	if len(tlsInfo.State.PeerCertificates) < 1 {
		if record := n.stats.getRecordFlag(); record {
			n.stats.incrementFailed()
		}
		return nil, errNoCert
	}

	return tlsInfo.State.PeerCertificates[0], nil
}

/*
func (n *Node) mergeGossip(set []*gossip.Data) {
	for _, entry := range set {
		n.evalGossip(entry)
	}
}

func (n *Node) evalGossip(item *gossip.Data) {
	if exists := n.gossipExists(string(item.Id)); exists {
		return
	}

	n.addGossip(item.Content, string(item.Id))
}
*/
func validMask(mask []byte, maxByz uint32, numRings uint32) error {
	var deactivated uint32

	if mask == nil {
		return errNoMask
	}

	if uint32(len(mask)) != numRings {
		return errInvalidMaskLength
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

	return nil
}

func checkDisabledRings(mask []byte, ringNum uint32, maxByz uint32, numRings uint32) error {
	err := validMask(mask, maxByz, numRings)
	if err != nil {
		return err
	}

	idx := ringNum - 1

	maxIdx := uint32(len(mask) - 1)

	if idx > maxIdx || idx < 0 {
		return errNonExistingRing
	}

	if mask[idx] == 0 {
		return errDeactivatedRing
	}

	return nil
}
