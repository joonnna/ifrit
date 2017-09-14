package node

import (
	"io"
	"github.com/joonnna/capstone/util"
)


func (n *Node) handleGossip(reader io.Reader) {
	msg := &gossip{}

	err := json.NewDecoder(reader).Decode(msg)
	if err != nil {
		n.log.Err.Println(err)
		return
	}

	diff := util.SliceDiff(n.getLiveNodes(), msg.AddrSlice)
	for _, addr := range diff {
		if addr == n.localAddr {
			continue
		}
		
		for id, ring := range n.ringMap {
			err = ring.add(addr)
			if err != nil {
				n.log.Err.Println(err)
			}
			n.updateState(id)
		}
		n.addViewPeer(addr)
	}
}
