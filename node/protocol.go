package node

import (
	"github.com/joonnna/capstone/protobuf"
)

type correct struct {

}

type spamAccusations struct {

}

func (c correct) Gossip(n *Node) {
	notes, accusations := n.collectGossipContent()
	args := &gossip.GossipMsg{Notes: notes, Accusations: accusations}

	neighbours := n.getNeighbours()

	for _, addr := range neighbours {
		_, err := n.Communication.Gossip(addr, args)
		if err != nil {
			n.log.Err.Println(err)
			continue
		}
	}
}

func (c correct) Monitor(n *Node) {
	msg := &gossip.Ping{}

	for _, ring := range n.ringMap {
		succ, err :=  ring.getRingSucc()
		if err != nil {
			n.log.Err.Println(err)
			continue
		}

		//TODO maybe udp?
		_, err = n.Communication.Monitor(succ.nodeId, msg)
		if err != nil {
			n.log.Info.Printf("%s is dead, accusing", succ.nodeId)
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
}

func (sa spamAccusations) Gossip(n *Node) {
	notes, accusations := createFalseAccusations(n)
	args := &gossip.GossipMsg{Notes: notes, Accusations: accusations}

	allNodes := n.getView()

	for _, p := range allNodes {
		_, err := n.Communication.Gossip(p.addr, args)
		if err != nil {
			n.log.Err.Println(err)
			continue
		}
	}
}

func (sa spamAccusations) Monitor(n *Node) {
	for _, ring := range n.ringMap {
		succ, err :=  ring.getRingSucc()
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


func createFalseAccusations(n *Node) (map[string] *gossip.Note, map[string] *gossip.Accusation) {
	noteMap := make(map[string]*gossip.Note)
	accuseMap := make(map[string]*gossip.Accusation)

	view := n.getView()

	for _, p := range view {
		peerNote := p.getNote()

		noteEntry := &gossip.Note {
			Epoch: peerNote.epoch,
			Addr: p.addr,
			Mask: peerNote.mask,
		}

		accuseEntry := &gossip.Accusation {
			Accuser: n.localAddr,
			RecentNote: noteEntry,
		}

		accuseMap[p.addr] = accuseEntry
		noteMap[p.addr] = noteEntry
	}

	noteMap[n.localAddr] = &gossip.Note {
		Epoch: n.epoch,
		Addr: n.localAddr,
	}

	return noteMap, accuseMap
}
