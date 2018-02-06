package ifrit

import (
	"errors"

	"github.com/joonnna/ifrit/core"
	"github.com/joonnna/ifrit/rpc"
)

type Client struct {
	node *core.Node
}

var (
	errNoData      = errors.New("Supplied data is of length 0")
	errNoCaAddress = errors.New("Config does not contain address of CA")
)

// Creates and returns a new ifrit client instance
// entryAddr is expected to be the address of the trusted CA (ip:port)
// May return an error if the CA is not reachable
// See config documentation for config description
func NewClient(entryAddr string, conf *Config) (*Client, error) {
	if entryAddr == "" {
		return nil, errNoCaAddress
	}
	nodeConf := parseConfig(conf, entryAddr)

	c := rpc.NewClient()

	s, err := rpc.NewServer()
	if err != nil {
		return nil, err
	}

	n, err := core.NewNode(nodeConf, c, s)
	if err != nil {
		return nil, err
	}

	client := &Client{
		node: n,
	}

	return client, nil
}

// Registers the given function as the message handler
// Each time the ifrit client receives an application specific message(another client sent it through SendTo/SendToAll/gossipcontent), this callback will be invoked
func (c *Client) RegisterMsgHandler(msgHandler func([]byte) ([]byte, error)) {
	c.node.SetMsgHandler(msgHandler)
}

// Shutsdown the client and all held resources
func (c *Client) ShutDown() {
	c.node.ShutDownNode()
}

// Client starts operating
func (c *Client) Start() {
	c.node.Start()
}

// Returns the address of all other ifrit clients in the network which is currently
// belivied to be alive
func (c *Client) Members() []string {
	return c.node.LiveMembers()
}

// Sends the given data to the given destinations.
// The caller must ensure that the given data is not modified after calling this function.
// The returned channel will be populated with the response from each message.
// If a destination could not be reached, a nil response will be sent through the channel.
// Recipients might be slow and might eventually not respond at all, if a timeout is exceeded
// a nil response will be sent through the channel for that particular message.
// The response data can be safely modified after receiving it.
func (c *Client) SendTo(dest []string, data []byte) chan []byte {
	ch := make(chan []byte)

	go func() {
		c.node.SendMessages(dest, ch, data)
	}()

	return ch
}

// Sends the given data to all members of the network belivied to be alive.
// The returned channel functions as described in SendTo().
// The returned integer represents the amount of members the message was sent to.
func (c *Client) SendToAll(data []byte) (chan []byte, int) {
	ch := make(chan []byte)

	members := c.node.LiveMembers()

	go func() {
		c.node.SendMessages(members, ch, data)
	}()

	return ch, len(members)
}

// Adds the given data to the gossip set.
// This data will be exchanged with neighbors in each gossip interaction.
// Recipients will receive it through the message handler callback.
func (c *Client) SetGossipContent(data []byte) error {
	if len(data) <= 0 {
		return errNoData
	}

	c.node.SetExternalGossipContent(data)

	return nil
}

func (c *Client) Id() string {
	return c.node.Id()
}
