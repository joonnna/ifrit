package core

import (
	"crypto/ecdsa"
	"os"
	"testing"

	log "github.com/inconshreveable/log15"
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
	var i, mask uint32

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

	for i = 0; i < numRings; i++ {
		mask = setBit(mask, i)
	}

	localNote := &note{
		epoch:  1,
		mask:   mask,
		peerId: p.peerId,
	}

	_, err = localNote.signAndMarshal(peerPrivKey)
	if err != nil {
		panic(err)
	}

	p.recentNote = localNote

	return p, peerPrivKey
}

func (suite *NodeTestSuite) SetupTest() {
	var numRings uint32 = 6

	p, priv := newTestPeer("mainPeer1234", numRings, "localhost:1000")

	v, err := newView(numRings, p.peerId, p.addr)
	require.NoError(suite.T(), err, "Failed to create view")

	n := &Node{
		peer:     p,
		privKey:  priv,
		view:     v,
		protocol: correct{},
	}

	suite.Node = n
}

func TestNodeTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	suite.Run(t, new(NodeTestSuite))
}
