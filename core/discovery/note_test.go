package discovery

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"os"
	"testing"

	log "github.com/inconshreveable/log15"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NoteTestSuite struct {
	suite.Suite
}

func TestNoteTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	suite.Run(t, new(NoteTestSuite))
}

func (suite *NoteTestSuite) TestSameEpoch() {
	n := &Note{
		epoch: 3,
	}

	assert.True(suite.T(), n.SameEpoch(n.epoch), "Returns false when epochs are equal")

	assert.False(suite.T(), n.SameEpoch(n.epoch+1), "Returns true when epochs are not equal")
}

func (suite *NoteTestSuite) TestIsMoreRecent() {
	n := &Note{
		epoch: 5,
	}

	assert.True(suite.T(), n.IsMoreRecent(n.epoch+1), "returns false when epoch is higher than note epoch")

	assert.False(suite.T(), n.IsMoreRecent(n.epoch-1), "returns true when epoch is lower than note epoch")

	assert.False(suite.T(), n.IsMoreRecent(n.epoch), "returns true when epoch is equal to the note epoch")
}

func (suite *NoteTestSuite) TestToPbMsg() {
	n := &Note{
		id:    "testid1",
		mask:  1,
		epoch: 3,
		signature: &signature{
			r: []byte("testR"),
			s: []byte("testS"),
		},
	}

	gossipNote := n.ToPbMsg()
	require.NotNil(suite.T(), gossipNote, "Protobuf message is nil")

	assert.Equal(suite.T(), n.id, string(gossipNote.GetId()), "Protobuf id is not equal")

	assert.Equal(suite.T(), n.mask, gossipNote.GetMask(), "Protobuf mask is not equal")

	assert.Equal(suite.T(), n.epoch, gossipNote.GetEpoch(), "Protobuf epoch is not equal")

	assert.Equal(suite.T(), n.signature.r, gossipNote.GetSignature().GetR(), "Protobuf signature r component is not equal")

	assert.Equal(suite.T(), n.signature.s, gossipNote.GetSignature().GetS(), "Protobuf signature s component is not equal")
}

func (suite *NoteTestSuite) TestSign() {
	n := &Note{
		id:    "testid1",
		mask:  1,
		epoch: 3,
	}

	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(suite.T(), err, "Failed to generate private key")

	assert.Error(suite.T(), n.sign(nil), "Returns no error with no private key")

	assert.Nil(suite.T(), n.signature, "Signature is not nil before signing")

	assert.NoError(suite.T(), n.sign(privKey), "Returns error with non-nil private key")

	assert.NotNil(suite.T(), n.signature, "Signature is still nil after signing accusation")

	assert.NotNil(suite.T(), n.signature.r, "Signature r component is still nil after signing accusation")

	assert.NotNil(suite.T(), n.signature.s, "Signature s component is still nil after signing accusation")
}
