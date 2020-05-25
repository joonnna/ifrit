package core

import (
	"errors"
	"time"

	"github.com/joonnna/ifrit/protobuf"
)

var (
	errNotFound     = errors.New("No node info found")
	errPeerNotFound = errors.New("No peer info found")
)

func (n *Node) collectGossipContent() *gossip.State {
	msg := n.view.State()

	msg.ExternalGossip = n.getExternalGossip()

	return msg
}

func (n *Node) setProtocol(pr protocol) {
	n.protocolMutex.Lock()
	defer n.protocolMutex.Unlock()

	n.p = pr
}

func (n *Node) protocol() protocol {
	n.protocolMutex.RLock()
	defer n.protocolMutex.RUnlock()

	return n.p
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

// Exposed to let ifrit client set directly
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

// Expose so that client can set new handler directly
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

// Expose so that client can set new handler directly
func (n *Node) SetGossipHandler(newHandler processMsg) {
	n.gossipHandlerMutex.Lock()
	defer n.gossipHandlerMutex.Unlock()

	n.gossipHandler = newHandler
}

func (n *Node) getGossipHandler() processMsg {
	n.gossipHandlerMutex.RLock()
	defer n.gossipHandlerMutex.RUnlock()

	return n.gossipHandler
}

// Expose so that client can set new handler directly
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

// Expose so that client can set new handler directly
func (n *Node) SetStreamHandler(newHandler func(<-chan []byte) ([]byte, error)) {
	n.streamHandlerMutex.Lock()
	defer n.streamHandlerMutex.Unlock()

	n.streamHandler = newHandler
}

func (n *Node) getStreamHandler() func(<-chan []byte) ([]byte, error) {
	n.streamHandlerMutex.RLock()
	defer n.streamHandlerMutex.RUnlock()

	return n.streamHandler
}