package communication


import (
	"fmt"
	"os"
	"strings"
	"net"
	"errors"
	"sync"
	"github.com/joonnna/capstone/util"
	"github.com/joonnna/capstone/logger"
	"bytes"
	"encoding/json"
)

const (
	startPort = 8345
)

var (
	errReachable = errors.New("Remote entity not reachable")
)

type Comm struct {
	allConnections map[string]net.Conn
	connectionMutex sync.RWMutex
	
	localAddr string
	listener net.Listener	

	log *logger.Log
}

func NewComm(log *logger.Log) *Comm {
	var l net.Listener
	var err error

	host, _ := os.Hostname()
	hostName := strings.Split(host, ".")[0]

	port := startPort

	for {
		l, err = net.Listen("tcp", fmt.Sprintf("%s:%d", hostName, port))
		if err != nil {
			log.Err.Println(err)
		} else {
			break
		}
		port += 1
	}


	comm := &Comm{
		allConnections: make(map[string]net.Conn),
		localAddr: hostName,
		listener: l,
		log: log,
	}

	return comm
}

func newMsg(localAddr string, addrSlice []string) ([]byte, error) {
	msg := &util.Message{
		AddrSlice: addrSlice,
		LocalAddr: localAddr,
	}

	marsh, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(marsh)

	err = json.NewEncoder(buf).Encode(msg)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (c *Comm) Ping(addr string, addrSlice []string) error {
	var conn net.Conn
	var err error

	if !c.existConnection(addr) {
		conn, err = c.dial(addr)	
		if err != nil {
			return errReachable
		}
	} else {
		conn = c.getConnection(addr)
	}
	
	data, err := newMsg(addr, addrSlice)
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	if err != nil {
		return err
	}
	return nil
}


func (c *Comm) dial(addr string) (net.Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		c.log.Err.Println(err)
		return nil, errReachable
	} else {
		c.addConnection(addr, conn)
	}

	return conn, nil
}


func (c Comm) ReceivePings(cb func(data []byte)) {
	defer c.listener.Close()

	for {
		conn, err := c.listener.Accept()
		if err != nil {
			c.log.Err.Println(err)
			continue
		}
	
		go c.handleConn(conn, cb)
	}

}

func (c Comm) handleConn(conn net.Conn, cb func(data []byte)) {
	data := make([]byte, 100)
	
	_, err := conn.Read(data)
	if err != nil {
		c.log.Err.Println(err)
		return
	} else {
		cb(data)
	}
}
