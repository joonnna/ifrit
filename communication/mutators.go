package communication


import (
	"github.com/joonnna/capstone/protobuf"
)

func (c *Comm) existConnection(addr string) bool {
	c.connectionMutex.RLock()
	defer c.connectionMutex.RUnlock()

	_, ok := c.allConnections[addr]

	return ok
}

func (c *Comm) getConnection(addr string) gossip.GossipClient {
	c.connectionMutex.RLock()
	defer c.connectionMutex.RUnlock()
	
	return c.allConnections[addr]
}


func (c *Comm) addConnection(addr string, conn gossip.GossipClient) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()

	if _, ok := c.allConnections[addr]; ok {
		c.log.Err.Printf("Connection already exists: %s", addr)
		return
	}

	c.allConnections[addr] = conn

}

func (c *Comm) removeConnection(addr string) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()

	delete(c.allConnections, addr)
}
