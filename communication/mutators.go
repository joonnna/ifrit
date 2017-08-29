package communication


import (
	"net"
)

func (c *Comm) existConnection(addr string) bool {
	c.connectionMutex.RLock()

	_, ok := c.allConnections[addr]

	c.connectionMutex.RUnlock()

	return ok
}

func (c *Comm) getConnection(addr string) net.Conn {
	c.connectionMutex.RLock()

	ret := c.allConnections[addr]

	c.connectionMutex.RUnlock()

	return ret
}


func (c *Comm) addConnection(addr string, conn net.Conn) {
	c.connectionMutex.Lock()

	c.allConnections[addr] = conn

	c.connectionMutex.Unlock()
}
