package node

import (
	"crypto/x509"
	"fmt"
	"time"

	"github.com/joonnna/capstone/protobuf"
)

type correct struct {
}

type spamAccusations struct {
}

func (c correct) Rebuttal(n *Node) {
	neighbours := n.getNeighbours()

	noteMsg := n.localNoteToPbMsg()

	msg := &gossip.GossipMsg{
		Notes: []*gossip.Note{noteMsg},
	}

	for _, addr := range neighbours {
		if addr == n.addr {
			continue
		}
		_, err := n.client.Gossip(addr, msg)
		if err != nil {
			n.log.Err.Println(err)
			continue
		}
	}
}

func (c correct) Gossip(n *Node) {
	msg, err := n.collectGossipContent()
	if err != nil {
		return
	}
	neighbours := n.getNeighbours()

	for _, addr := range neighbours {
		if addr == n.addr {
			continue
		}
		reply, err := n.client.Gossip(addr, msg)
		if err != nil {
			n.log.Err.Println(err, addr)
			continue
		}

		certs := reply.GetCertificates()
		if certs == nil {
			continue
		} else {
			for _, c := range certs {
				cert, err := x509.ParseCertificate(c.GetRaw())
				if err != nil {
					n.log.Err.Println(err)
					continue
				}

				if n.viewPeerExist(string(cert.SubjectKeyId[:])) {
					continue
				}

				p, err := newPeer(nil, cert)
				if err != nil {
					n.log.Err.Println(err)
					continue
				}

				n.addViewPeer(p)
				n.addLivePeer(p)
			}
		}
	}
}

func (c correct) Monitor(n *Node) {
	msg := &gossip.Ping{}

	for _, ring := range n.ringMap {
		succ, err := ring.getRingSucc()
		if err != nil {
			n.log.Err.Println(err)
			continue
		}

		if succ.addr == n.addr {
			continue
		}

		//TODO maybe udp?
		_, err = n.client.Monitor(succ.addr, msg)
		if err != nil {
			n.log.Info.Printf("%s is dead, accusing", succ.addr)
			p := n.getLivePeer(succ.peerKey)
			if p == nil {
				continue
			}
			peerNote := p.getNote()
			if peerNote == nil {
				continue
			}

			a := &accusation{
				peerId:  p.peerId,
				epoch:   peerNote.epoch,
				accuser: n.peerId,
				mask:    peerNote.mask,
			}

			err = a.sign(n.privKey)
			if err != nil {
				n.log.Err.Println(err)
				continue
			}

			err = p.setAccusation(a)
			if err != nil {
				n.log.Err.Println(err)
				continue
			}
			if !n.timerExist(p.key) {
				n.startTimer(p.key, peerNote, n.peer, p.addr)
			}
		}
	}
}

func (c correct) Timeouts(n *Node) {
	timeouts := n.getAllTimeouts()
	for key, t := range timeouts {
		n.log.Debug.Println("Have timeout for: ", t.addr)
		since := time.Since(t.timeStamp)
		if since.Seconds() > n.nodeDeadTimeout {
			n.log.Debug.Printf("%s timeout expired, removing from live", t.addr)
			n.deleteTimeout(key)
			n.removeLivePeer(key)
		}
	}
}

func (sa spamAccusations) Gossip(n *Node) {
	msg, err := createFalseAccusations(n)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	allNodes := n.getView()

	for _, p := range allNodes {
		_, err := n.client.Gossip(p.addr, msg)
		if err != nil {
			n.log.Err.Println(err)
			continue
		}
	}
}

func (sa spamAccusations) Monitor(n *Node) {
	for _, ring := range n.ringMap {
		succ, err := ring.getRingSucc()
		if err != nil {
			n.log.Err.Println(err)
			continue
		}

		p := n.getLivePeer(succ.peerKey)
		if p == nil {
			continue
		}

		peerNote := p.getNote()
		if peerNote == nil {
			continue
		}

		a := &accusation{
			peerId:  p.peerId,
			epoch:   peerNote.epoch,
			accuser: n.peerId,
			mask:    peerNote.mask,
		}

		err = a.sign(n.privKey)
		if err != nil {
			n.log.Err.Println(err)
			continue
		}

		err = p.setAccusation(a)
		if err != nil {
			n.log.Err.Println(err)
			return
		}

		if !n.timerExist(p.key) {
			n.startTimer(p.key, peerNote, n.peer, p.addr)
		}

	}
}

func (sa spamAccusations) Rebuttal(n *Node) {
	neighbours := n.getNeighbours()

	noteMsg := n.localNoteToPbMsg()

	msg := &gossip.GossipMsg{
		Notes: []*gossip.Note{noteMsg},
	}

	for _, addr := range neighbours {
		if addr == n.addr {
			continue
		}
		_, err := n.client.Gossip(addr, msg)
		if err != nil {
			n.log.Err.Println(err)
			continue
		}
	}
}

func (sa spamAccusations) Timeouts(n *Node) {
}

func createFalseAccusations(n *Node) (*gossip.GossipMsg, error) {
	msg := &gossip.GossipMsg{}

	view := n.getView()

	for _, p := range view {
		peerNote := p.getNote()
		if peerNote == nil {
			continue
		}

		a := &gossip.Accusation{
			Accuser: n.id,
			Epoch:   peerNote.epoch,
			Accused: peerNote.id,
			Mask:    peerNote.mask,
		}

		b := []byte(fmt.Sprintf("%v", a))
		signature, err := signContent(b, n.privKey)
		if err != nil {
			n.log.Err.Println(err)
			return nil, err
		}

		a.Signature = &gossip.Signature{
			R: signature.r,
			S: signature.s,
		}

		msg.Accusations = append(msg.Accusations, a)
	}

	noteMsg := n.localNoteToPbMsg()

	msg.Notes = append(msg.Notes, noteMsg)

	return msg, nil
}
