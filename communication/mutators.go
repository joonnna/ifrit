package communication


import (
	"github.com/joonnna/capstone/protobuf"
)

func (c *Comm) existConnection(addr string) bool {
	c.connectionMutex.RLock()

	_, ok := c.allConnections[addr]

	c.connectionMutex.RUnlock()

	return ok
}

func (c *Comm) getConnection(addr string) gossip.GossipClient {
	c.connectionMutex.RLock()

	ret := c.allConnections[addr]

	c.connectionMutex.RUnlock()

	return ret
}


func (c *Comm) addConnection(addr string, conn gossip.GossipClient) {
	c.connectionMutex.Lock()

	c.allConnections[addr] = conn

	c.connectionMutex.Unlock()
}
