package core

import (
	"crypto/tls"
	"os"
	"testing"

	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"

	"github.com/joonnna/ifrit/protobuf"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type NodeTestSuite struct {
	suite.Suite
	nodes []*Node
}

func TestNodeTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	viper.Set("use_ca", false)
	viper.Set("use_viz", false)

	suite.Run(t, new(NodeTestSuite))
}

func (suite *NodeTestSuite) SetupTest() {
	for i := 0; i < 30; i++ {
		n, err := NewNode(&clientStub{}, &serverStub{})
		require.NoError(suite.T(), err, "Failed to create node.")

		suite.nodes = append(suite.nodes, n)
	}
}

func (suite *NodeTestSuite) TestGossip() {
	// Everyone gossips with everyone,
	// then assert that everyone has the same view.

	for _, n := range suite.nodes {
		for _, n2 := range suite.nodes {
			reply, err := n2.Spread(context.Background(), n.collectGossipContent())
			if err != nil {
				continue
			}
			n.mergeCertificates(reply.GetCertificates())
			n.mergeNotes(reply.GetNotes())
			n.mergeAccusations(reply.GetAccusations())
		}
	}

	for _, n := range suite.nodes {
		for _, n2 := range suite.nodes {
			require.NoError(suite.T(), n.view.Compare(n2.view), "Views are not equal.")
		}
	}

}

type clientStub struct {
}

func (cs *clientStub) Init(config *tls.Config) {
}

func (cs *clientStub) Gossip(addr string, args *gossip.State) (*gossip.StateResponse, error) {
	return nil, nil
}

func (cs *clientStub) SendMsg(addr string, args *gossip.Msg) (*gossip.MsgResponse, error) {
	return nil, nil
}

func (cs *clientStub) CloseConn(addr string) {
}

type serverStub struct {
}

func (ss *serverStub) Init(config *tls.Config, n interface{}, maxConcurrent uint32) error {
	return nil
}

func (ss *serverStub) Addr() string {
	return "127.0.0.1:8000"
}

func (ss *serverStub) Start() error {
	return nil
}

func (ss *serverStub) ShutDown() {
}
