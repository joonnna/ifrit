package connection


import (
	"net"
	"errors"
)

type Connection struct {
	ip string
	port string
	addr string
	net.Conn
}

var (
	errReachable = errors.New("Remote node reachable")
)


func (c *Connection) Ping(data []byte) error {
	_, err := c.Conn.Write(data)
	if err != nil {
		conn, err := net.Dial("tcp", c.addr)
		if err != nil {
			return errReachable
		} else {
			c.Conn = conn
			c.Ping(data)
		}
	}
	return nil
}

func NewConn(addr string) *Connection {
	c := &Connection{
		addr: addr,
	}
	return c
}
/*
func dial(addr string) net.Conn {
	net.Dial("tcp", addr)
}
*/




