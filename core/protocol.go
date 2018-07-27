package core

import (
	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/core/discovery"
	"github.com/joonnna/ifrit/protobuf"
)

type correct struct {
}

type spamAccusations struct {
}

type experiment struct {
	addr    string
	maxConc int
}

func (c correct) Rebuttal(n *Node) {
	neighbours := n.view.GossipPartners()

	noteMsg := n.self.Note().ToPbMsg()

	msg := &gossip.State{
		OwnNote: noteMsg,
	}

	for _, p := range neighbours {
		_, err := n.client.Gossip(p.Addr, msg)
		if err != nil {
			log.Error(err.Error(), "addr", p.Addr)
			continue
		}
	}
}

func (c correct) Gossip(n *Node) {
	msg := n.collectGossipContent()

	neighbours := n.view.GossipPartners()

	for _, p := range neighbours {
		reply, err := n.client.Gossip(p.Addr, msg)
		if err != nil {
			log.Error(err.Error(), "addr", p.Addr)
			continue
		}

		//log.Debug("Gossiped", "addr", p.Addr)

		n.mergeCertificates(reply.GetCertificates())
		n.mergeNotes(reply.GetNotes())
		n.mergeAccusations(reply.GetAccusations())

		if handler := n.getResponseHandler(); handler != nil {
			if r := reply.GetExternalGossip(); r != nil {
				handler(r)
			}
		}
	}
}

func (c correct) Monitor(n *Node) {
	var i uint32

	/*
		p, ringNum := n.view.MonitorTarget()
		if p == nil {
			log.Debug("No peers to monitor, must be alone")
			return
		}
	*/

	for i = 1; i <= n.view.NumRings(); i++ {
		p, ringNum := n.view.MonitorTarget()
		if p == nil {
			continue
		}
		err := n.failureDetector.ping(p)
		if err == errDead {
			log.Debug("Successor dead, accusing", "succ", p.Addr, "ringNum", ringNum)
			peerNote := p.Note()

			// Will always have note for a peer in our liveView, except when the peer stems
			// from the initial contact list of the CA, if it's dead
			// we should remove it to ensure it doesn't stay in our liveView.
			// Not possible to accuse a peer without a note.
			if peerNote == nil {
				n.view.RemoveLive(p.Id)
				log.Debug("Removing live peer due to not having note and being accused", "addr", p.Addr)
				continue
			}

			err := p.CreateAccusation(peerNote, n.self, ringNum, n.privKey)
			if err == discovery.ErrAccAlreadyExists {
				if exists := n.view.HasTimer(p.Id); !exists {
					n.view.StartTimer(p, peerNote, n.self)
				}
			} else if err != nil {
				log.Error(err.Error())
			}
		}
	}
}

/*
func (sa spamAccusations) Gossip(n *Node) {
	msg, err := createFalseAccusations(n)
	if err != nil {
		log.Error(err.Error())
		return
	}

	allNodes := n.getView()

	for _, p := range allNodes {
		reply, err := n.client.Gossip(p.addr, msg)
		if err != nil {
			log.Error(err.Error())
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
			log.Error(err.Error())
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
			log.Error(err.Error())
			continue
		}

		err = p.setAccusation(a)
		if err != nil {
			log.Error(err.Error())
			return
		}

		if !n.timerExist(p.key) {
			n.startTimer(p.key, peerNote, n.peer, p.addr)
		}

	}
}

func (sa spamAccusations) Rebuttal(n *Node) {
	var err error
	var neighbours []string

	for {
		neighbours, err = n.getGossipPartners()
		if err != nil {
			log.Error(err.Error())
		} else {
			break
		}
	}

	noteMsg := n.localNoteToPbMsg()

	msg := &gossip.State{
		OwnNote: noteMsg,
	}

	for _, addr := range neighbours {
		if addr == n.addr {
			continue
		}
		_, err = n.client.Gossip(addr, msg)
		if err != nil {
			log.Error(err.Error())
			continue
		}
	}
}

func (sa spamAccusations) Timeouts(n *Node) {
}

func createFalseAccusations(n *Node) (*gossip.State, error) {
	var i uint32
	msg := &gossip.State{}

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
				log.Error(err.Error())
				continue
			}

			err = p.setAccusation(a)
			if err != nil {
				log.Error(err.Error())
				continue
			}

			//msg.Accusations = append(msg.Accusations, a.toPbMsg())
		}
	}

	noteMsg := n.localNoteToPbMsg()

	msg.OwnNote = noteMsg

	return msg, nil
}

func (e experiment) Rebuttal(n *Node) {
	var err error
	var neighbours []string

	for {
		neighbours, err = n.getGossipPartners()
		if err != nil {
			log.Error(err.Error())
		} else {
			break
		}
	}

	noteMsg := n.localNoteToPbMsg()

	msg := &gossip.State{
		OwnNote: noteMsg,
	}

	for _, addr := range neighbours {
		if addr == n.addr {
			continue
		}
		_, err = n.client.Gossip(addr, msg)
		if err != nil {
			log.Error(err.Error())
			continue
		}
	}
}

func dos(addr string, msg *gossip.State, n *Node) {
	_, err := n.client.Gossip(addr, msg)
	if err != nil {
		log.Error("%s, addr: %s", err.Error(), addr)
	}
}

func (e experiment) Gossip(n *Node) {
		msg, err := n.collectGossipContent()
		if err != nil {
			return
		}

	msg := &gossip.State{}

	for i := 0; i < e.maxConc; i++ {
		go dos(e.addr, msg, n)
	}
}

func (e experiment) Monitor(n *Node) {
	for _, ring := range n.ringMap {
		succ, err := ring.getRingSucc()
		if err != nil {
			log.Error(err.Error())
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

			log.Debug("%s is dead, accusing", p.addr)
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
				log.Debug("Already accused peer on this ring")
				if !n.timerExist(p.key) {
					n.startTimer(p.key, peerNote, n.peer, p.addr)
				}
				continue
			}

			err = a.sign(n.privKey)
			if err != nil {
				log.Error(err.Error())
				continue
			}

			err = p.setAccusation(a)
			if err != nil {
				log.Error(err.Error())
				continue
			}
			if !n.timerExist(p.key) {
				n.startTimer(p.key, peerNote, n.peer, p.addr)
			}
		}
	}
}

func (e experiment) Timeouts(n *Node) {
	timeouts := n.getAllTimeouts()
	for key, t := range timeouts {
		log.Debug("Have timeout for: %s", t.addr)
		since := time.Since(t.timeStamp)
		if since.Seconds() > n.nodeDeadTimeout {
			log.Debug("%s timeout expired, removing from live", t.addr)
			n.deleteTimeout(key)
			n.removeLivePeer(key)
		}
	}
}
*/
