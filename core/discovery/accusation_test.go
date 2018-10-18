package discovery

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	log "github.com/inconshreveable/log15"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AccTestSuite struct {
	suite.Suite
}

func TestAccTestSuite(t *testing.T) {
	r := log.Root()

	//r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	r.SetHandler(log.DiscardHandler())
	suite.Run(t, new(AccTestSuite))
}

func (suite *AccTestSuite) TestEqual() {
	acc := &Accusation{
		accused: "testid1",
		accuser: "testid2",
		ringNum: 3,
		epoch:   5,
	}

	assert.True(suite.T(), acc.Equal(acc.accused, acc.accuser, acc.ringNum, acc.epoch), "accusation compare returns true with 2 equal accusations")

	assert.False(suite.T(), acc.Equal(acc.accused, "wrong_accuser", acc.ringNum, acc.epoch), "accusation compare returns true when accuser is wrong")

	assert.False(suite.T(), acc.Equal("wrong_accused", acc.accuser, acc.ringNum, acc.epoch), "accusation compare returns true when accused is wrong")

	assert.False(suite.T(), acc.Equal(acc.accused, acc.accuser, acc.ringNum-1, acc.epoch), "accusation compare returns true when ringNum is wrong")

	assert.False(suite.T(), acc.Equal(acc.accused, acc.accuser, acc.ringNum, acc.epoch-1), "accusation compare returns true when epoch is wrong")
}

func (suite *AccTestSuite) TestIsMoreRecent() {
	acc := &Accusation{
		epoch: 5,
	}

	assert.True(suite.T(), acc.IsMoreRecent(acc.epoch+1), "returns false when epoch is higher than accusation epoch")

	assert.False(suite.T(), acc.IsMoreRecent(acc.epoch-1), "returns true when epoch is lower than accusation epoch")

	assert.False(suite.T(), acc.IsMoreRecent(acc.epoch), "returns true when epoch is equal to the accusation epoch")
}

func (suite *AccTestSuite) TestIsAccuser() {
	acc := &Accusation{
		accuser: "testid1",
	}

	assert.True(suite.T(), acc.IsAccuser(acc.accuser), "returns false when id is the accuser")

	assert.False(suite.T(), acc.IsAccuser(acc.accuser+"test"), "returns true when id is not the accuser")
}

func (suite *AccTestSuite) TestToPbMsg() {
	acc := &Accusation{
		accused: "testid1",
		accuser: "testid2",
		ringNum: 3,
		epoch:   5,
		signature: &signature{
			r: []byte("testR"),
			s: []byte("testS"),
		},
	}

	gossipAcc := acc.ToPbMsg()

	require.NotNil(suite.T(), gossipAcc, "returned protobuf message is nil")

	assert.Equal(suite.T(), acc.accused, string(gossipAcc.GetAccused()), "Protobuf message has different accused field.")

	assert.Equal(suite.T(), acc.accuser, string(gossipAcc.GetAccuser()), "Protobuf message has different accuser field.")

	assert.Equal(suite.T(), acc.epoch, gossipAcc.GetEpoch(), "Protobuf message has different epoch field.")

	assert.Equal(suite.T(), acc.ringNum, gossipAcc.GetRingNum(), "Protobuf message has different ringNum field.")

	assert.Equal(suite.T(), acc.signature.r, gossipAcc.Signature.GetR(), "Protobuf message has different signature r field.")

	assert.Equal(suite.T(), acc.signature.s, gossipAcc.Signature.GetS(), "Protobuf message has different signature s field.")
}

func (suite *AccTestSuite) TestSign() {
	acc := &Accusation{
		accused: "testid1",
		accuser: "testid2",
		ringNum: 5,
		epoch:   3,
	}

	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(suite.T(), err, "Failed to generate private key")

	assert.Error(suite.T(), signAcc(acc, nil), "Returns no error with no private key")

	assert.Nil(suite.T(), acc.signature, "Signature is not nil before signing")

	assert.NoError(suite.T(), signAcc(acc, privKey), "Returns error with non-nil private key")

	assert.NotNil(suite.T(), acc.signature, "Signature is still nil after signing accusation")

	assert.NotNil(suite.T(), acc.signature.r, "Signature r component is still nil after signing accusation")

	assert.NotNil(suite.T(), acc.signature.s, "Signature s component is still nil after signing accusation")
}
