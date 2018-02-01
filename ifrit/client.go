package ifrit

import (
	"errors"
	"io"

	"github.com/joonnna/ifrit/lib/node"
	"github.com/joonnna/ifrit/lib/rpc"
)

type Client struct {
	node *node.Node
}

var (
	errNoData = errors.New("Supplied data is of length 0")
)

func NewClient(entryAddr string) (*Client, error) {
	c := rpc.NewClient()

	s, err := rpc.NewServer()
	if err != nil {
		return nil, err
	}

	n, err := node.NewNode(entryAddr, c, s)
	if err != nil {
		return nil, err
	}

	client := &Client{
		node: n,
	}

	return client, nil
}

func (c *Client) RegisterMsgHandler(msgHandler func([]byte) ([]byte, error)) {
	c.node.SetMsgHandler(msgHandler)
}

func (c *Client) ShutDown() {
	c.node.ShutDownNode()
}

func (c *Client) Start() {
	c.node.Start()
}

func (c *Client) Members() []string {
	return c.node.LiveMembers()
}

func (c *Client) SendTo(dest []string, data io.Reader) (chan io.Reader, error) {
	var bytes []byte

	_, err := data.Read(bytes)
	if err != nil {
		return nil, nil
	}

	ch := make(chan io.Reader)

	go func() {
		c.node.SendMessages(dest, ch, bytes)
	}()

	return ch, nil
}

func (c *Client) SendToAll(data io.Reader) (chan io.Reader, error) {
	var bytes []byte

	_, err := data.Read(bytes)
	if err != nil {
		return nil, nil
	}

	ch := make(chan io.Reader)

	go func() {
		c.node.SendMessages(c.node.LiveMembers(), ch, bytes)
	}()

	return ch, nil
}

func (c *Client) SetGossipContent(data []byte) error {
	/*
		buf := make([]byte, data.Len())

		n, err := data.Read(buf)
		if err != nil {
			return err
		}
	*/
	if len(data) <= 0 {
		return errNoData
	}

	c.node.SetExternalGossipContent(data)

	return nil
}

func (c *Client) Id() string {
	return c.node.Id()
}

/*
func (c *Client) AddGossip(id []byte, data io.Reader) error {
	if len(id) <= 0 {
		return errInvalidId
	}

	return c.node.AppendGossipData(id, data)
}
*/
