package core

import (
	"crypto/ecdsa"
	"testing"

	"github.com/stretchr/testify/require"
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
		accusations: make(map[uint32]*accusation),
	}

	for i = 1; i <= numRings; i++ {
		p.accusations[i] = nil
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

func (suite *NodeTestSuite) SetupTest() {
	var numRings uint32

	numRings = 3

	p, priv := newTestPeer("mainPeer1234", numRings, "localhost:1000")

	v, err := newView(numRings, logger, p.peerId, p.addr)
	require.NoError(suite.T(), err, "Failed to create view")

	n := &Node{
		peer:    p,
		privKey: priv,
		view:    v,
	}

	suite.Node = n
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestNodeTestSuite(t *testing.T) {
	suite.Run(t, new(NodeTestSuite))
}
