package core

import (
	"errors"
	"time"

	"github.com/joonnna/ifrit/protobuf"
)

var (
	errNotFound           = errors.New("No node info found")
	errPeerNotFound       = errors.New("No peer info found")
	errAlreadyDeactivated = errors.New("Ring was already deactivated")
)

func (n *Node) setEpoch(newEpoch uint64) {
	n.noteMutex.Lock()
	defer n.noteMutex.Unlock()

	n.recentNote.epoch = newEpoch

	err := n.recentNote.sign(n.privKey)
	if err != nil {
		n.log.Err.Println(err)
	}
}

func (n *Node) getEpoch() uint64 {
	n.noteMutex.RLock()
	defer n.noteMutex.RUnlock()

	return n.recentNote.epoch
}

func (n *Node) deactivateRing(idx uint32) {
	n.noteMutex.Lock()
	defer n.noteMutex.Unlock()

	if n.maxByz == 0 {
		return
	}

	ringNum := idx - 1

	len := uint32(len(n.recentNote.mask))

	maxIdx := len - 1

	if ringNum > maxIdx || ringNum < 0 {
		n.log.Err.Println(errNonExistingRing)
		return
	}

	if n.recentNote.mask[ringNum] == 0 {
		n.log.Err.Println(errAlreadyDeactivated)
		return
	}

	if n.deactivatedRings == n.maxByz {
		var idx uint32
		for idx = 0; idx < len; idx++ {
			if idx != ringNum && n.recentNote.mask[idx] == 0 {
				break
			}
		}
		n.recentNote.mask[idx] = 1
	} else {
		n.deactivatedRings++
	}

	n.recentNote.mask[ringNum] = 0

	err := n.recentNote.sign(n.privKey)
	if err != nil {
		n.log.Err.Println(err)
	}
}

func (n *Node) getMask() []byte {
	n.noteMutex.RLock()
	defer n.noteMutex.RUnlock()

	ret := make([]byte, len(n.recentNote.mask))

	copy(ret, n.recentNote.mask)

	return ret
}

func (n *Node) collectGossipContent() (*gossip.State, error) {
	msg := &gossip.State{
		ExistingHosts:  make(map[string]uint64),
		OwnNote:        n.localNoteToPbMsg(),
		ExternalGossip: n.getExternalGossip(),
	}

	view := n.getView()

	for _, p := range view {
		peerEpoch := uint64(0)

		if peerNote := p.getNote(); peerNote != nil {
			peerEpoch = peerNote.epoch
		}

		msg.ExistingHosts[p.key] = peerEpoch
	}

	return msg, nil
}

func (n *Node) setProtocol(p protocol) {
	n.protocolMutex.Lock()
	defer n.protocolMutex.Unlock()

	n.protocol = p
}

func (n *Node) getProtocol() protocol {
	n.protocolMutex.RLock()
	defer n.protocolMutex.RUnlock()

	return n.protocol
}

func (n *Node) localNoteToPbMsg() *gossip.Note {
	n.noteMutex.RLock()
	defer n.noteMutex.RUnlock()

	return n.recentNote.toPbMsg()
}

func (n *Node) setGossipTimeout(timeout int) {
	n.gossipTimeoutMutex.Lock()
	defer n.gossipTimeoutMutex.Unlock()

	n.gossipTimeout = (time.Duration(timeout) * time.Second)
}

func (n *Node) getGossipTimeout() time.Duration {
	n.gossipTimeoutMutex.RLock()
	defer n.gossipTimeoutMutex.RUnlock()

	return n.gossipTimeout
}

//Exposed to let ifrit client set directly
func (n *Node) SetExternalGossipContent(data []byte) {
	n.externalGossipMutex.Lock()
	defer n.externalGossipMutex.Unlock()

	n.externalGossip = data
}

func (n *Node) getExternalGossip() []byte {
	n.externalGossipMutex.RLock()
	defer n.externalGossipMutex.RUnlock()

	return n.externalGossip
}

//Expose so that client can set new handler directly
func (n *Node) SetMsgHandler(newHandler processMsg) {
	n.msgHandlerMutex.Lock()
	defer n.msgHandlerMutex.Unlock()

	n.msgHandler = newHandler
}

func (n *Node) getMsgHandler() processMsg {
	n.msgHandlerMutex.RLock()
	defer n.msgHandlerMutex.RUnlock()

	return n.msgHandler
}

//Expose so that client can set new handler directly
func (n *Node) SetResponseHandler(newHandler func([]byte)) {
	n.responseHandlerMutex.Lock()
	defer n.responseHandlerMutex.Unlock()

	n.responseHandler = newHandler
}

func (n *Node) getResponseHandler() func([]byte) {
	n.responseHandlerMutex.RLock()
	defer n.responseHandlerMutex.RUnlock()

	return n.responseHandler
}
