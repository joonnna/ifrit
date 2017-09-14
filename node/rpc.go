package node

import (
	"github.com/joonnna/capstone/protobuf"
	"golang.org/x/net/context"
)


func (n *Node) Spread (ctx context.Context, args *gossip.NodeInfo) (*gossip.Nodes, error) {
	if !n.viewPeerExist(args.LocalAddr) {
		n.addViewPeer(args.LocalAddr)
	}

	view := n.getViewAddrs()

	fullView := append(view, n.localAddr)

	reply := &gossip.Nodes {
		LocalAddr: n.localAddr,
		AddrList: fullView,
	}

	return reply, nil
}

func (n *Node) Monitor (ctx context.Context, args *gossip.Ping) (*gossip.Pong, error) {
	reply := &gossip.Pong {
		LocalAddr: n.localAddr,
	}

	return reply, nil
}

func (n *Node) Accuse (ctx context.Context, args *gossip.Accusation) (*gossip.Empty, error) {
	reply := &gossip.Empty {}

	accused := args.GetAccused()

	p, err := n.getViewPeer(accused)
	if err != nil {
		n.log.Info.Println("Got accusation from uknown node: %s", accused)
		return reply, nil
	}

	p.addAccusation(args.GetAccuser())
	
	return reply, nil
}






