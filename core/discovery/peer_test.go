package discovery

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"os"
	"testing"

	log "github.com/inconshreveable/log15"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PeerTestSuite struct {
	suite.Suite
	priv *ecdsa.PrivateKey

	numRings uint32
	p        *Peer
}

func (suite *PeerTestSuite) SetupTest() {
	var i uint32
	var numRings uint32 = 5

	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(suite.T(), err, "Failed to generate private key.")

	suite.priv = privKey

	id := "testId"

	p := &Peer{
		Id:          id,
		accusations: make(map[uint32]*Accusation),
		note: &Note{
			id:    id,
			epoch: 5,
			mask:  6,
		},
		publicKey: suite.priv.Public().(*ecdsa.PublicKey),
		cert: &x509.Certificate{
			SubjectKeyId: []byte(id),
			Subject: pkix.Name{
				Locality: []string{"rpcAddr", "pingAddr", "httpAddr"},
			},
			PublicKey: suite.priv.Public(),
		},
	}

	require.NoError(suite.T(), signNote(p.note, suite.priv), "Failed to sign note")

	for i = 1; i <= numRings; i++ {
		p.accusations[i] = nil
	}

	suite.p = p
	suite.numRings = numRings
}

func TestPeerTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	suite.Run(t, new(PeerTestSuite))
}

func (suite *PeerTestSuite) TestNewPeer() {
	cert := &x509.Certificate{
		SubjectKeyId: []byte("testId"),
		Subject: pkix.Name{
			Locality: []string{"rpcAddr", "pingAddr", "httpAddr"},
		},
		PublicKey: suite.priv.Public(),
	}

	p, err := newPeer(cert, suite.numRings)
	require.NoError(suite.T(), err, "Failed to create new peer with valid paramters.")

	assert.Equal(suite.T(), cert.Subject.Locality[0], p.Addr, "First element of locality is no set as the peer's main address.")

	assert.Equal(suite.T(), cert.Subject.Locality[1], p.PingAddr, "Second element of locality is no set as the peer's ping address.")

	assert.Equal(suite.T(), cert.Subject.Locality[2], p.HttpAddr, "Third element of locality is no set as the peer's http address.")

	assert.Equal(suite.T(), int(suite.numRings), len(p.accusations), "Size of accusation length is not equal to number of ring.")

	assert.Equal(suite.T(), string(cert.SubjectKeyId), p.Id, "Certificate SubjectKeyId is not set as the peer's id.")

	cert2 := &x509.Certificate{
		SubjectKeyId: []byte("testId"),
		PublicKey:    suite.priv.Public(),
	}

	_, err = newPeer(cert2, suite.numRings)
	require.EqualError(suite.T(), err, errPeerAddr.Error(), "No error returned with no Subject in certificate.")

	cert3 := &x509.Certificate{
		SubjectKeyId: []byte("testId"),
		Subject: pkix.Name{
			Locality: []string{"rpcAddr"},
		},
		PublicKey: suite.priv.Public(),
	}

	_, err = newPeer(cert3, suite.numRings)
	require.EqualError(suite.T(), err, errPeerAddr.Error(), "No error returned with only 1 address in Subject field.")

	cert4 := &x509.Certificate{
		SubjectKeyId: []byte("testId"),
		Subject: pkix.Name{
			Locality: []string{"rpcAddr", "pingAddr", "httpAddr"},
		},
	}

	_, err = newPeer(cert4, suite.numRings)
	require.EqualError(suite.T(), err, errPubKey.Error(), "No error returned without public key in certificate.")

	cert5 := &x509.Certificate{
		Subject: pkix.Name{
			Locality: []string{"rpcAddr", "pingAddr", "httpAddr"},
		},
		PublicKey: suite.priv.Public(),
	}

	_, err = newPeer(cert5, suite.numRings)
	require.EqualError(suite.T(), err, errPeerId.Error(), "No error returned without SubjectKeyId in certificate.")

	cert6 := &x509.Certificate{
		SubjectKeyId: []byte("testId"),
		Subject: pkix.Name{
			Locality: []string{"rpcAddr", "pingAddr", "httpAddr"},
		},
		PublicKey: suite.priv.Public(),
	}
	_, err = newPeer(cert6, 0)
	require.EqualError(suite.T(), err, errNoRings.Error(), "No error returned with numRings set to 0.")
}

func (suite *PeerTestSuite) TestCertificate() {
	p := &Peer{
		cert: &x509.Certificate{
			Raw: []byte("testCertificate"),
		},
	}

	cert := p.Certificate()

	assert.NotNil(suite.T(), cert, "Returned certificate is nil when internal certificate is non-nil.")

	assert.Equal(suite.T(), cert, p.cert.Raw, "Returned certificate is not equal to the internal certificate.")

	p.cert = nil

	assert.Nil(suite.T(), p.Certificate(), "Returned certificate is non-nil when internal certificate is nil.")
}

func (suite *PeerTestSuite) TestValidateSignature() {
	data := []byte("test_signature_data_that_is_very_very_very_long")

	b := hashContent(data)

	rInt, sInt, err := ecdsa.Sign(rand.Reader, suite.priv, b)
	require.NoError(suite.T(), err, "Failed to sign content")

	r := rInt.Bytes()
	s := sInt.Bytes()

	assert.True(suite.T(), suite.p.ValidateSignature(r, s, data), "Returned false when signature was valid.")

	assert.False(suite.T(), suite.p.ValidateSignature(r, s, []byte("not_valid")), "Returned true when signature was invalid.")

	suite.p.publicKey = nil

	assert.False(suite.T(), suite.p.ValidateSignature(r, s, data), "Returned true when peer had no public key.")
}

func (suite *PeerTestSuite) TestCreateAccusation() {
	var i uint32
	accuseRing := suite.numRings

	accused := &Peer{
		Id: "selfId",
		note: &Note{
			epoch: 10,
		},
		accusations: make(map[uint32]*Accusation),
	}

	for i = 1; i <= suite.numRings; i++ {
		accused.accusations[i] = nil
	}

	require.NoError(suite.T(), accused.CreateAccusation(accused.note, suite.p, accuseRing, suite.priv), "Failed to add a valid accusation.")

	assert.EqualError(suite.T(), accused.CreateAccusation(accused.note, suite.p, accuseRing, suite.priv), ErrAccAlreadyExists.Error(), "Adding the same accusation twice returned no error.")

	assert.EqualError(suite.T(), accused.CreateAccusation(accused.note, suite.p, suite.numRings+1, suite.priv), errInvalidRing.Error(), "Adding an accusation on an invalid ring returned no error.")

	for i = 1; i <= suite.numRings; i++ {
		accused.accusations[i] = nil
	}

	assert.Error(suite.T(), suite.p.CreateAccusation(accused.note, suite.p, accuseRing, nil), "Adding an accusation without a private key returned no error.")
}

func (suite *PeerTestSuite) TestAddAccusation() {
	p := suite.p

	accuserId := "accuserId"
	accusedId := "accusedId"

	err := p.AddAccusation(accusedId, accuserId, p.note.epoch, suite.numRings, p.note.signature.r, p.note.signature.s)
	require.NoError(suite.T(), err, "Failed to add valid accusation")

	err = p.AddAccusation(accusedId, accuserId, p.note.epoch+1, suite.numRings, p.note.signature.r, p.note.signature.s)
	require.EqualError(suite.T(), err, errOldEpoch.Error(), "Accusation with different epoch compared to note hould return an error.")

	err = p.AddAccusation(accusedId, accuserId, p.note.epoch, suite.numRings+1, p.note.signature.r, p.note.signature.s)
	require.EqualError(suite.T(), err, errInvalidRing.Error(), "Accusation with invalid ring number returned no error.")

	err = p.AddAccusation(accusedId, accuserId, p.note.epoch, suite.numRings, nil, nil)
	require.EqualError(suite.T(), err, errAccuserSign.Error(), "Accusation without signature returned no error.")

	err = p.AddAccusation(accusedId, "", p.note.epoch, suite.numRings, p.note.signature.r, p.note.signature.s)
	require.EqualError(suite.T(), err, errAccuserId.Error(), "Accusation without accuser id returned no error.")

	err = p.AddAccusation("", accuserId, p.note.epoch, suite.numRings, p.note.signature.r, p.note.signature.s)
	require.EqualError(suite.T(), err, errAccusedId.Error(), "Accusation without accused id returned no error.")

}

func (suite *PeerTestSuite) TestRemoveAccusation() {
	var i uint32

	acc := &Accusation{
		ringNum: suite.numRings,
	}

	suite.p.accusations[acc.ringNum] = acc

	suite.p.RemoveAccusation(acc)

	assert.Nil(suite.T(), suite.p.accusations[acc.ringNum], "Accusation was not removed from valid ring number")

	for i = 1; i <= suite.numRings; i++ {
		a := &Accusation{
			ringNum: i,
		}
		suite.p.accusations[a.ringNum] = a
	}

	outOfBounds := &Accusation{
		ringNum: suite.numRings + 1,
	}

	suite.p.RemoveAccusation(outOfBounds)

	for i = 1; i <= suite.numRings; i++ {
		assert.NotNil(suite.T(), suite.p.accusations[i], "Accusation was removed after removing an calling remove with invalid ring number.")
	}
}

func (suite *PeerTestSuite) TestRemoveRingAccusation() {
	var i uint32

	acc := &Accusation{
		ringNum: suite.numRings,
	}

	suite.p.accusations[acc.ringNum] = acc

	suite.p.RemoveRingAccusation(acc.ringNum)

	assert.Nil(suite.T(), suite.p.accusations[acc.ringNum], "Accusation was not removed from valid ring number")

	for i = 1; i <= suite.numRings; i++ {
		a := &Accusation{
			ringNum: i,
		}
		suite.p.accusations[a.ringNum] = a
	}

	suite.p.RemoveRingAccusation(suite.numRings + 1)

	for i = 1; i <= suite.numRings; i++ {
		assert.NotNil(suite.T(), suite.p.accusations[i], "Accusation was removed after removing an calling remove with invalid ring number.")
	}

}

func (suite *PeerTestSuite) TestClearAccusations() {
	var i uint32

	for i = 1; i <= suite.numRings; i++ {
		a := &Accusation{
			ringNum: i,
		}
		suite.p.accusations[a.ringNum] = a
	}

	suite.p.ClearAccusations()

	for i = 1; i <= suite.numRings; i++ {
		assert.Nil(suite.T(), suite.p.accusations[i], "Accusation was not removed after clearing all accusations.")
	}

}

func (suite *PeerTestSuite) TestRingAccusation() {
	var i uint32

	for i = 1; i <= suite.numRings; i++ {
		a := &Accusation{
			ringNum: i,
		}
		suite.p.accusations[a.ringNum] = a
	}

	for i = 1; i <= suite.numRings; i++ {
		acc := suite.p.RingAccusation(i)
		require.NotNil(suite.T(), acc, "Returned nil accusation.")
		assert.Equal(suite.T(), i, acc.ringNum, "Returned wrong accusation, different ring number.")
	}

	require.Nil(suite.T(), suite.p.RingAccusation(suite.numRings+1), "Returned non-nil accusation with invalid ring number.")
}

func (suite *PeerTestSuite) TestAnyAccusations() {
	assert.Nil(suite.T(), suite.p.AnyAccusation(), "Returned non-nil accusations, when non exists.")

	a := &Accusation{
		ringNum: suite.numRings,
	}
	suite.p.accusations[a.ringNum] = a

	acc := suite.p.AnyAccusation()
	require.NotNil(suite.T(), acc, "Returned nil accusation when there exists 1")

	assert.Equal(suite.T(), acc.ringNum, a.ringNum, "Returned an accusation with different ring number.")
}

func (suite *PeerTestSuite) TestAllAccusations() {
	var i uint32

	accs := suite.p.AllAccusations()
	require.NotNil(suite.T(), accs, "Returned nil slice with no accusations present.")
	assert.Zero(suite.T(), len(accs), "Returned slice with non-zero length with no accusations existed.")

	a := &Accusation{
		ringNum: suite.numRings,
	}
	suite.p.accusations[a.ringNum] = a

	accs2 := suite.p.AllAccusations()
	require.NotNil(suite.T(), accs2, "Returned nil slice with accusations present.")
	assert.Equal(suite.T(), 1, len(accs2), "Returned slice with different length than amount of accusations.")

	for i = 1; i <= suite.numRings; i++ {
		a := &Accusation{
			ringNum: i,
		}
		suite.p.accusations[a.ringNum] = a
	}

	accs3 := suite.p.AllAccusations()
	require.NotNil(suite.T(), accs3, "Returned nil slice with accusations present.")
	assert.Equal(suite.T(), int(suite.numRings), len(accs3), "Returned slice with different length than amount of accusations.")

}

func (suite *PeerTestSuite) TestIsAccused() {
	require.False(suite.T(), suite.p.IsAccused(), "Returned true when peer is not accused.")

	suite.p.accusations[suite.numRings] = &Accusation{ringNum: suite.numRings}

	require.True(suite.T(), suite.p.IsAccused(), "Returned false when peer is accused")
}

func (suite *PeerTestSuite) TestAddNote() {
	newNote := &Note{
		mask:  suite.p.note.mask + 1,
		epoch: suite.p.note.epoch + 1,
		signature: &signature{
			r: []byte("testSignR"),
			s: []byte("testSignS"),
		},
	}

	suite.p.AddNote(newNote.mask, newNote.epoch, newNote.r, newNote.s)
	assert.Equal(suite.T(), suite.p.note.epoch, newNote.epoch, "New note has invalid epoch.")
	assert.Equal(suite.T(), suite.p.note.mask, newNote.mask, "New note has invalid mask.")
	assert.Equal(suite.T(), suite.p.note.signature.r, newNote.signature.r, "New note has invalid signature R.")
	assert.Equal(suite.T(), suite.p.note.signature.s, newNote.signature.s, "New note has invalid signature S.")

	oldNote := &Note{
		mask:  newNote.mask + 1,
		epoch: 0,
		signature: &signature{
			r: []byte("testSignR2"),
			s: []byte("testSignS2"),
		},
	}

	suite.p.AddNote(oldNote.mask, oldNote.epoch, oldNote.r, oldNote.s)
	assert.NotEqual(suite.T(), suite.p.note.epoch, oldNote.epoch, "Adding old note replaces more recent one.")
	assert.NotEqual(suite.T(), suite.p.note.mask, oldNote.mask, "Adding old note replaces more recent one.")
	assert.NotEqual(suite.T(), suite.p.note.signature.r, oldNote.signature.r, "New note has invalid mask.")
	assert.NotEqual(suite.T(), suite.p.note.signature.s, oldNote.signature.s, "New note has invalid signature R.")

	suite.p.note = nil

	suite.p.AddNote(oldNote.mask, oldNote.epoch, oldNote.r, oldNote.s)
	assert.Equal(suite.T(), suite.p.note.epoch, oldNote.epoch, "Adding note when there exists none fails.")
	assert.Equal(suite.T(), suite.p.note.mask, oldNote.mask, "Adding note when there exists none fails.")
	assert.Equal(suite.T(), suite.p.note.signature.r, oldNote.signature.r, "Adding note when there exists one fails.")
	assert.Equal(suite.T(), suite.p.note.signature.s, oldNote.signature.s, "Adding note when there exists one fails.")

}

func (suite *PeerTestSuite) TestNote() {
	p := suite.p

	assert.Equal(suite.T(), p.note, p.Note(), "Returns wrong note address value.")

	newNote := &Note{}
	p.note = newNote

	assert.Equal(suite.T(), newNote, p.Note(), "Returns wrong note address value after replacing note.")
}

func (suite *PeerTestSuite) TestInfo() {
	suite.p.accusations[suite.numRings] = &Accusation{
		ringNum: suite.numRings,
		signature: &signature{
			r: []byte("signatureR"),
			s: []byte("signatureS"),
		},
	}

	cert, note, accs := suite.p.Info()

	assert.NotNil(suite.T(), cert, "Returned certificate was nil.")
	assert.NotNil(suite.T(), note, "Returned note was nil.")
	require.NotNil(suite.T(), accs, "Returned accusations was nil.")
	assert.NotZero(suite.T(), len(accs), "Returned accusations was empty.")
}

func (suite *PeerTestSuite) TestIncrementPing() {
	prevPing := suite.p.nPing

	suite.p.IncrementPing()

	assert.Equal(suite.T(), suite.p.nPing, prevPing+1, "Num ping was not incremented.")
}

func (suite *PeerTestSuite) TestResetPing() {
	suite.p.nPing = 10

	suite.p.ResetPing()

	assert.Zero(suite.T(), suite.p.nPing, "Num ping was not reset.")
}

func (suite *PeerTestSuite) TestNumPing() {
	suite.p.nPing = 10

	currPing := suite.p.nPing

	assert.Equal(suite.T(), suite.p.NumPing(), currPing, "Retreived wrong ping value.")
}

func (suite *PeerTestSuite) TestHashContent() {
	data := []byte("testbyteslice")

	assert.NotNil(suite.T(), hashContent(data), "Returned hash is nil.")
}
