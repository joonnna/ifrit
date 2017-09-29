package node

import (
	"github.com/joonnna/capstone/protobuf"
)

type correct struct {
}

type spamAccusations struct {
}

func (c correct) Gossip(n *Node) {
	msg := n.collectGossipContent()
	neighbours := n.getNeighbours()

	for _, addr := range neighbours {
		if addr == n.localAddr {
			continue
		}
		_, err := n.NodeComm.Gossip(addr, msg)
		if err != nil {
			n.log.Err.Println(err)
			continue
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

		if succ.addr == n.localAddr {
			continue
		}

		//TODO maybe udp?
		_, err = n.NodeComm.Monitor(succ.addr, msg)
		if err != nil {
			n.log.Info.Printf("%s is dead, accusing", succ.addr)
			p := n.getViewPeer(succ.peerKey)
			if p != nil {
				peerNote := p.getNote()
				a, err := newAccusation(p.peerId, (peerNote.epoch + 1), n.NodeComm.OwnCertificate().Raw, n.peerId)
				if err != nil {
					n.log.Err.Println(err)
					return
				}
				err = p.setAccusation(a)
				if err != nil {
					n.log.Err.Println(err)
					return
				}
				if !n.timerExist(p.key) {
					//TODO change timeout observer to peer? create local peer in node struct
					n.startTimer(p.key, peerNote, nil)
				}
			}
		}
	}
}

/*
func (sa spamAccusations) Gossip(n *Node) {
	notes, accusations := createFalseAccusations(n)
	args := &gossip.GossipMsg{Notes: notes, Accusations: accusations}

	allNodes := n.getView()

	for _, p := range allNodes {
		_, err := n.NodeComm.Gossip(p.addr, args)
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

		p := n.getViewPeer(succ.nodeId)
		if p != nil {
			peerNote := p.getNote()
			p.setAccusation(n.localAddr, peerNote)
			if !n.timerExist(p.addr) {
				n.startTimer(p.addr, peerNote, n.localAddr)
			}
		}
	}
}

func createFalseAccusations(n *Node) (map[string]*gossip.Note, map[string]*gossip.Accusation) {
	noteMap := make(map[string]*gossip.Note)
	accuseMap := make(map[string]*gossip.Accusation)

	view := n.getView()

	for _, p := range view {
		peerNote := p.getNote()

		noteEntry := &gossip.Note{
			Epoch: peerNote.epoch,
			Addr:  p.addr,
			Mask:  peerNote.mask,
		}

		accuseEntry := &gossip.Accusation{
			Accuser:    n.localAddr,
			RecentNote: noteEntry,
		}

		accuseMap[p.addr] = accuseEntry
		noteMap[p.addr] = noteEntry
	}

	noteMap[n.localAddr] = &gossip.Note{
		Epoch: n.epoch,
		Addr:  n.localAddr,
	}

	return noteMap, accuseMap
}
*/
