package node

import (
	"time"

	"github.com/joonnna/firechain/lib/protobuf"
)

type correct struct {
}

type spamAccusations struct {
}

func (c correct) Rebuttal(n *Node) {
	neighbours := n.getNeighbours()

	noteMsg := n.localNoteToPbMsg()

	msg := &gossip.GossipMsg{
		OwnNote: noteMsg,
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
		n.mergeCertificates(reply.GetCertificates())
		n.mergeNotes(reply.GetNotes())
	}
}

func (c correct) Monitor(n *Node) {
	n.log.Debug.Println(len(n.getView()))
	n.log.Debug.Println(len(n.getLivePeers()))
	n.log.Debug.Println(n.getNeighbours())
	for _, ring := range n.ringMap {
		succ, err := ring.getRingSucc()
		if err != nil {
			n.log.Err.Println(err)
			continue
		}

		p := n.getLivePeer(succ.key)
		if p == nil {
			continue
		}

		err = n.ping(p)
		if err != nil {
			if err != errDead {
				continue
			}

			n.log.Info.Printf("%s is dead, accusing", p.addr)
			peerNote := p.getNote()
			//Will always have note for a peer in our liveView, except when the peer stems
			//from the initial contact list of the CA, if it's dead
			//we should remove it to ensure it doesn't stay in our liveView.
			//Not possible to accuse a peer without a note.
			if peerNote == nil {
				n.removeLivePeer(p.key)
				continue
			}

			a := &accusation{
				peerId:  p.peerId,
				epoch:   peerNote.epoch,
				accuser: n.peerId,
				mask:    peerNote.mask,
				ringNum: ring.ringNum,
			}

			acc := p.getRingAccusation(ring.ringNum)
			if a.equal(acc) {
				n.log.Info.Println("Already accused peer on this ring")
				if !n.timerExist(p.key) {
					n.startTimer(p.key, peerNote, n.peer, p.addr)
				}
				continue
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
		reply, err := n.client.Gossip(p.addr, msg)
		if err != nil {
			n.log.Err.Println(err, p.addr)
			continue
		}
		n.mergeCertificates(reply.GetCertificates())
		n.mergeNotes(reply.GetNotes())
	}
}

func (sa spamAccusations) Monitor(n *Node) {
	for _, ring := range n.ringMap {
		succ, err := ring.getRingSucc()
		if err != nil {
			n.log.Err.Println(err)
			continue
		}

		p := n.getLivePeer(succ.key)
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
			ringNum: ring.ringNum,
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
	var i uint32
	msg := &gossip.GossipMsg{}

	view := n.getView()

	for _, p := range view {
		peerNote := p.getNote()
		if peerNote == nil {
			continue
		}

		for i = 1; i <= n.numRings; i++ {
			a := &accusation{
				peerId:  p.peerId,
				epoch:   peerNote.epoch,
				accuser: n.peerId,
				mask:    peerNote.mask,
				ringNum: i,
			}

			err := a.sign(n.privKey)
			if err != nil {
				n.log.Err.Println(err)
				continue
			}

			err = p.setAccusation(a)
			if err != nil {
				n.log.Err.Println(err)
				continue
			}

			msg.Accusations = append(msg.Accusations, a.toPbMsg())
		}
	}

	noteMsg := n.localNoteToPbMsg()

	msg.OwnNote = noteMsg

	return msg, nil
}
