package core

import (
	"crypto/x509"
	"errors"
	"math/bits"

	"github.com/golang/protobuf/proto"
	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/protobuf"
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
	p := n.getViewPeer(key)

	if !neighbours && exist && p != nil && p.isAccused() {
		n.evalNote(args.GetOwnNote())

		for _, a := range p.getAllAccusations() {
			if a != nil {
				reply.Accusations = append(reply.Accusations, a.toPbMsg())
			}
		}

		return reply, nil
	}

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

	extGossip := args.GetExternalGossip()

	//Already evaluated the new note, can return an empty reply
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

	if record := n.stats.getRecordFlag(); record {
		n.stats.incrementCompleted()
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

	if epoch, exists := given[n.key]; !exists || epoch < n.getEpoch() {
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
		if e := n.getEpoch(); epoch != e {
			log.Debug("Received accusation for myself on old note", "epoch", epoch, "local epoch", e)
			return
		}
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
		log.Debug("Already have accusation, discard")
		if !n.timerExist(accusedKey) {
			log.Debug("Started timer for: %s", p.addr)
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
		log.Error(err.Error())
		return
	}

	valid, err := validateSignature(sign.GetR(), sign.GetS(), b, accuserPeer.publicKey)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if !valid {
		log.Debug("Accuser: %s", accuserPeer.addr)
		log.Debug("Accused: %s", p.addr)
		log.Debug("Invalid signature on accusation, ignoring", "accuser", accuserPeer.addr, "accused", p.addr)
		return
	}

	peerNote := p.getNote()
	if epoch == peerNote.epoch {
		err := checkDisabledRings(mask, ringNum, n.maxByz, n.numRings)
		if err != nil {
			log.Error(err.Error())
			return
		}

		if !n.isPrev(p, accuserPeer, ringNum) {
			log.Error("Accuser is not pre-decessor of accused on given ring, invalid accusation")
			return
		}

		err = p.setAccusation(newAcc)
		if err != nil {
			log.Error(err.Error())
			return
		}
		log.Debug("Added accusation", "addr", p.addr, "ring", newAcc.ringNum)

		if !n.timerExist(accusedKey) {
			log.Debug("Started timer", "addr", p.addr)
			n.startTimer(p.key, p.recentNote, accuserPeer, p.addr)
		}
	}
}

func (n *Node) evalNote(gossipNote *gossip.Note) bool {
	if gossipNote == nil {
		log.Debug("Got nil note")
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
		log.Error(err.Error())
		return false
	}

	valid, err := validateSignature(sign.GetR(), sign.GetS(), b, p.publicKey)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	if !valid {
		log.Debug("Invalid signature on note, ignoring, %s", p.addr)
		return false
	}

	err = validMask(mask, n.maxByz, n.numRings)
	if err != nil {
		log.Debug(err.Error())
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
			log.Debug("Rebuttal received", "addr", p.addr)
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
			p.resetPing()

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
		log.Debug("Got nil cert")
		return false
	}

	if len(cert.Subject.Locality) == 0 {
		return false
	}

	if len(cert.SubjectKeyId) == 0 || n.peerId.equal(&peerId{id: cert.SubjectKeyId}) {
		return false
	}

	err := checkSignature(cert, n.caCert)
	if err != nil {
		log.Error(err.Error())
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

func validMask(mask uint32, maxByz uint32, numRings uint32) error {
	active := bits.OnesCount32(mask)
	disabled := numRings - uint32(active)

	if disabled > maxByz {
		return errTooManyDeactivatedRings
	}

	return nil
}

func checkDisabledRings(mask uint32, ringNum uint32, maxByz uint32, numRings uint32) error {
	err := validMask(mask, maxByz, numRings)
	if err != nil {
		return err
	}

	idx := ringNum - 1

	maxIdx := uint32(numRings - 1)

	if idx > maxIdx || idx < 0 {
		return errNonExistingRing
	}

	if active := hasBit(mask, idx); !active {
		return errDeactivatedRing
	}

	return nil
}

func setBit(n uint32, pos uint32) uint32 {
	n |= (1 << pos)
	return n
}

func clearBit(n uint32, pos uint32) uint32 {
	mask := uint32(^(1 << pos))
	n &= mask
	return n
}

func hasBit(n uint32, pos uint32) bool {
	val := n & (1 << pos)
	return (val > 0)
}
