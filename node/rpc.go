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
	for addr, val := range notes {
		newNote := createNote(val.GetEpoch(), val.GetMask())
		currPeer := n.getViewPeer(addr)
		if currPeer == nil {
			p := newPeer(addr, newNote)
			n.addViewPeer(p)
			n.addLivePeer(p)
		} else {
			currNote := currPeer.getNote()
			if currNote == nil || currNote.isMoreRecent(newNote.epoch) {
				currPeer.setNote(newNote)
			}
		}
		

		/*
		else if currPeer.isRecentNote(val.GetEpoch()) {
			currPeer.setNote(newNote)
		}
		*/
	}
}

func (n *Node) mergeAccusations(accusations map[string]*gossip.Accusation) {
	for addr, val := range accusations {
		//n.log.Debug.Println("Got accusation for:", addr)
		if addr == n.localAddr {
			//TODO rebbuttal shit note.epoch += 1?
			continue
		}
		currPeer := n.getViewPeer(addr)
		if currPeer != nil {
			accuseNote := val.GetRecentNote()
			newEpoch := accuseNote.GetEpoch()
			
			newNote := createNote(newEpoch, accuseNote.GetMask())
			accuse := currPeer.getAccusation() 
			if accuse == nil || accuse.recentNote.isMoreRecent(newEpoch) {
				n.log.Debug.Println("Added accusation for: ", currPeer.addr)
				currPeer.setAccusation(val.GetAccuser(), newNote)

				n.log.Debug.Println("Started timer for: ", currPeer.addr)
				n.startTimer(currPeer.addr, newNote, val.GetAccuser())
			}

			/*
			if currPeer.isRecentAccusation(newEpoch) {
				currPeer.setAccusation(val.GetAccuser(), newNote)
			}
			*/
		}
	}
}
