package node

import (
	"github.com/joonnna/capstone/protobuf"
	"golang.org/x/net/context"
)

//TODO accusations and notes
func (n *Node) Spread (ctx context.Context, args *gossip.GossipMsg) (*gossip.Empty, error) {
	//Only for pull based
	/*
	remoteAddr := remoteNote.GetAddr()
	if !n.viewPeerExist(remoteAddr) {
		newNote := createNote(remoteNote.GetEpoch(), remoteNote.GetMask(), remoteNote.GetSignature())
		p := newPeer(remoteAddr, newNote)
		n.addViewPeer(p)
		n.addLivePeer(p)
	}
	*/

	reply := &gossip.Empty{}

	n.mergeNotes(args.GetNotes())
	n.mergeAccusations(args.GetAccusations())

	return reply, nil
}

func (n *Node) Monitor (ctx context.Context, args *gossip.Ping) (*gossip.Pong, error) {
	reply := &gossip.Pong {
		LocalAddr: n.localAddr,
	}

	return reply, nil
}

func (n *Node) mergeNotes(notes map[string]*gossip.Note) {
	for _, newNote:= range notes {
		if newNote.GetAddr() == n.localAddr {
			continue
		}
		n.evalNote(newNote)
	}
}

func (n *Node) mergeAccusations(accusations map[string]*gossip.Accusation) {
	for _, acc := range accusations {
		n.evalAccusation(acc)
	}
}

func (n *Node) evalAccusation(a *gossip.Accusation) {
	note := a.GetRecentNote()
	accuser := a.GetAccuser()
	accused := note.GetAddr()
	newEpoch := note.GetEpoch()
	mask := note.GetMask()

	//Rebuttal, TODO only gossip own note, not all?
	if accused == n.localAddr {
		n.setEpoch((newEpoch + 1))
		n.getProtocol().Gossip(n)
		return
	}

	p := n.getViewPeer(accused)
	if p != nil {
		peerAccuse := p.getAccusation()
		if peerAccuse == nil || peerAccuse.recentNote.isMoreRecent(newEpoch) {
			newNote := createNote(p.addr, newEpoch, mask)

			n.log.Debug.Println("Added accusation for: ", p.addr)
			p.setAccusation(accuser, newNote)

			if !n.timerExist(p.addr) {
				n.log.Debug.Println("Started timer for: ", p.addr)
				n.startTimer(p.addr, newNote, accuser)
			}
		}
	}
}

func (n *Node) evalNote(gossipNote *gossip.Note) {
	newEpoch := gossipNote.GetEpoch()
	addr := gossipNote.GetAddr()
	mask := gossipNote.GetMask()

	newNote := createNote(addr, newEpoch, mask)

	p := n.getViewPeer(addr)

	//Unseen peer, need to add to both live/view
	if p == nil {
		newPeer := newPeer(newNote)
		n.addViewPeer(newPeer)
		n.addLivePeer(newPeer)
	} else {
		peerAccuse := p.getAccusation()

		//Not accused, only check if newnote is more recent
		if peerAccuse == nil {
			currNote := p.getNote()

			//Want to store the most recent note
			if currNote == nil || currNote.isMoreRecent(newNote.epoch) {
				p.setNote(newNote)
			}
		} else {
			accuseNote := peerAccuse.getNote()

			//Peer is accused, need to check if this note invalidates accusation
			if accuseNote.isMoreRecent(newNote.epoch) {
				n.log.Info.Println("Rebuttal received for:", p.addr)
				p.removeAccusation()
				n.deleteTimeout(p.addr)
			}
		}
	}
}
