package node

import (
	"crypto/ecdsa"
	"testing"

	"github.com/joonnna/firechain/logger"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type NodeTestSuite struct {
	suite.Suite
	*Node
}

func newTestPeer(id string, numRings uint32, addr string) (*peer, *ecdsa.PrivateKey) {
	var i uint32

	peerPrivKey, err := genKeys()
	if err != nil {
		panic(err)
	}

	p := &peer{
		publicKey:   &peerPrivKey.PublicKey,
		peerId:      newPeerId([]byte(id)),
		addr:        addr,
		accusations: make([]*accusation, numRings),
	}

	localNote := &note{
		epoch:  1,
		mask:   make([]byte, numRings),
		peerId: p.peerId,
	}

	for i = 0; i < numRings; i++ {
		localNote.mask[i] = 1
	}

	_, err = localNote.signAndMarshal(peerPrivKey)
	if err != nil {
		panic(err)
	}

	p.recentNote = localNote

	return p, peerPrivKey
}

// Make sure that VariableThatShouldStartAtFive is set to five
// before each test
func (suite *NodeTestSuite) SetupTest() {
	var numRings uint32

	numRings = 3

	p, priv := newTestPeer("mainPeer1234", numRings, "localhost:1000")

	logger := logger.CreateLogger("test", "testlog")

	n := &Node{
		peer:    p,
		privKey: priv,
		log:     logger,
		view:    newView(numRings, logger, p.peerId, p.addr),
	}

	suite.Node = n
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestNodeTestSuite(t *testing.T) {
	suite.Run(t, new(NodeTestSuite))
}