package discovery

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math"
	"math/big"
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
	signer   signer
}

func (suite *PeerTestSuite) SetupTest() {
	var i uint32
	var numRings uint32 = 5

	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	require.NoError(suite.T(), err, "Failed to generate private key.")

	suite.priv = privKey

	id := "selfId"

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
	suite.signer = &signerMock{
		priv: suite.priv,
	}
}

func TestPeerTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	//r.SetHandler(log.DiscardHandler())
	suite.Run(t, new(PeerTestSuite))
}

func (suite *PeerTestSuite) TestNewPeer() {
	validCert := &x509.Certificate{
		SubjectKeyId: []byte("testId"),
		Subject: pkix.Name{
			Locality: []string{"rpcAddr", "pingAddr", "httpAddr"},
		},
		PublicKey: suite.priv.Public(),
	}

	noSubjectCert := &x509.Certificate{
		SubjectKeyId: []byte("testId"),
		PublicKey:    suite.priv.Public(),
	}

	noIdCert := &x509.Certificate{
		Subject: pkix.Name{
			Locality: []string{"rpcAddr", "pingAddr", "httpAddr"},
		},
		PublicKey: suite.priv.Public(),
	}

	noPubCert := &x509.Certificate{
		SubjectKeyId: []byte("testId"),
		Subject: pkix.Name{
			Locality: []string{"rpcAddr", "pingAddr", "httpAddr"},
		},
	}

	tests := []struct {
		in       *x509.Certificate
		numRings uint32
		err      error

		addr      string
		pingAddr  string
		httpAddr  string
		accMapLen int
		id        string
		pub       ecdsa.PublicKey
	}{
		{
			in:        validCert,
			numRings:  suite.numRings,
			err:       nil,
			addr:      validCert.Subject.Locality[0],
			pingAddr:  validCert.Subject.Locality[1],
			httpAddr:  validCert.Subject.Locality[2],
			accMapLen: int(suite.numRings),
			id:        string(validCert.SubjectKeyId),
		},

		{
			in:  validCert,
			err: errNoRings,
		},

		{
			in:       nil,
			numRings: suite.numRings,
			err:      errNoCert,
		},

		{
			in:       noSubjectCert,
			numRings: suite.numRings,
			err:      errPeerAddr,
		},

		{
			in:       noIdCert,
			numRings: suite.numRings,
			err:      errPeerId,
		},

		{
			in:       noPubCert,
			numRings: suite.numRings,
			err:      errPubKey,
		},
	}

	for i, t := range tests {
		p, err := newPeer(t.in, t.numRings)
		require.Equalf(suite.T(), t.err, err, "Invalid error for test %d", i)

		if t.err != nil {
			require.Nilf(suite.T(), p, "Invalid output for test %d", i)
		} else {
			require.NotNilf(suite.T(), p, "Invalid output for test %d", i)

			require.Equalf(suite.T(), t.addr, p.Addr, "Invalid addr for test %d", i)
			require.Equalf(suite.T(), t.pingAddr, p.PingAddr,
				"Invalid pingAddr for test %d", i)
			require.Equalf(suite.T(), t.httpAddr, p.HttpAddr, "Invalid error for test %d", i)
			require.Equalf(suite.T(), t.accMapLen, len(p.accusations),
				"Invalid len of accusations for test %d", i)
			require.Equalf(suite.T(), t.id, p.Id, "Invalid id for test %d", i)
		}

	}
}

func (suite *PeerTestSuite) TestCertificate() {
	p := &Peer{
		cert: &x509.Certificate{
			Raw: []byte("testCertificate"),
		},
	}

	nilCertPeer := &Peer{}

	tests := []struct {
		p   *Peer
		out []byte
	}{
		{
			p:   p,
			out: p.cert.Raw,
		},

		{
			p:   nilCertPeer,
			out: nil,
		},
	}

	for i, t := range tests {
		require.Equalf(suite.T(), t.out, t.p.Certificate(), "Invalid output for test %d", i)
	}
}

func (suite *PeerTestSuite) TestPublicKey() {
	pb, ok := suite.priv.Public().(*ecdsa.PublicKey)
	require.True(suite.T(), ok, "Failed to cast cert public key to ecdsa pub key")

	p := &Peer{
		publicKey: pb,
	}

	nilPubPeer := &Peer{}

	tests := []struct {
		p   *Peer
		out *ecdsa.PublicKey
	}{
		{
			p:   p,
			out: pb,
		},

		{
			p:   nilPubPeer,
			out: nil,
		},
	}

	for i, t := range tests {
		require.Equalf(suite.T(), t.out, t.p.PublicKey(), "Invalid output for test %d", i)
	}
}
func (suite *PeerTestSuite) TestCreateAccusation() {
	var i uint32

	accId := "testId"

	accused := &Peer{
		Id: accId,
		note: &Note{
			epoch: 10,
			id:    accId,
		},
		accusations: make(map[uint32]*Accusation),
	}

	for i = 1; i <= suite.numRings; i++ {
		accused.accusations[i] = nil
	}

	tests := []struct {
		accused *Peer
		self    *Peer
		note    *Note
		ringNum uint32
		s       signer
		err     error
	}{
		{
			accused: accused,
			self:    suite.p,
			note:    accused.Note(),
			ringNum: suite.numRings,
			s:       suite.signer,
			err:     nil,
		},

		{

			accused: accused,
			self:    suite.p,
			note:    accused.Note(),
			ringNum: suite.numRings,
			s:       suite.signer,
			err:     ErrAccAlreadyExists,
		},

		{

			accused: accused,
			self:    suite.p,
			note:    accused.Note(),
			ringNum: suite.numRings + 1,
			s:       suite.signer,
			err:     errInvalidRing,
		},
	}

	for i, t := range tests {
		require.Equalf(suite.T(), t.err,
			t.accused.CreateAccusation(t.note, t.self, t.ringNum, t.s),
			"Invalid output for test %d", i)

		if t.err == nil {
			a, ok := t.accused.accusations[t.ringNum]
			require.Truef(suite.T(), ok, "Accusation not added to map, test %d", i)

			require.Equalf(suite.T(), a.accused, t.accused.Id,
				"Wrong accused id, test %d", i)

			require.Equalf(suite.T(), a.accuser, t.self.Id,
				"Wrong accuser id, test %d", i)

			require.Equalf(suite.T(), a.epoch, t.note.epoch,
				"Wrong epoch, test %d", i)

			require.Equalf(suite.T(), a.ringNum, t.ringNum,
				"Wrong ringNum, test %d", i)

			require.NotNilf(suite.T(), a.signature, "Signature should not be nil, test %d", i)
		} else if t.err != ErrAccAlreadyExists {
			_, ok := t.accused.accusations[t.ringNum]
			require.Falsef(suite.T(), ok,
				"Accusation should not be added when error is returned, test %d", i)
		}
	}
}

func (suite *PeerTestSuite) TestAddAccusation() {
	peer := suite.p

	tests := []struct {
		p       *Peer
		acc     string
		accuser string
		epoch   uint64
		ringNum uint32
		r       []byte
		s       []byte
		err     error
	}{
		{
			p:       peer,
			acc:     peer.Id,
			accuser: "accuserId",
			epoch:   0,
			ringNum: suite.numRings,
			r:       []byte("signature"),
			s:       []byte("signature"),
			err:     errOldEpoch,
		},

		{
			p:       peer,
			acc:     peer.Id,
			accuser: "accuserId",
			epoch:   peer.note.epoch,
			ringNum: suite.numRings + 1,
			r:       []byte("signature"),
			s:       []byte("signature"),
			err:     errInvalidRing,
		},

		{
			p:       peer,
			acc:     peer.Id,
			accuser: "accuserId",
			epoch:   peer.note.epoch,
			ringNum: suite.numRings,
			s:       []byte("signature"),
			err:     errAccuserSign,
		},

		{
			p:       peer,
			acc:     peer.Id,
			accuser: "accuserId",
			epoch:   peer.note.epoch,
			ringNum: suite.numRings,
			r:       []byte("signature"),
			err:     errAccuserSign,
		},

		{
			p:       peer,
			accuser: "accuserId",
			epoch:   peer.note.epoch,
			ringNum: suite.numRings,
			r:       []byte("signature"),
			s:       []byte("signature"),
			err:     errAccusedId,
		},

		{
			p:       peer,
			acc:     peer.Id,
			epoch:   peer.note.epoch,
			ringNum: suite.numRings,
			r:       []byte("signature"),
			s:       []byte("signature"),
			err:     errAccuserId,
		},

		{
			p:       peer,
			acc:     peer.Id,
			accuser: "accuserId",
			epoch:   peer.note.epoch,
			ringNum: suite.numRings,
			r:       []byte("signature"),
			s:       []byte("signature"),
			err:     nil,
		},
	}

	for i, t := range tests {
		err := t.p.AddAccusation(t.acc, t.accuser, t.epoch, t.ringNum, t.r, t.s)
		require.Equalf(suite.T(), t.err, err, "Invalid error for test %d", i)

		if t.err == nil {
			a, ok := t.p.accusations[t.ringNum]
			require.Truef(suite.T(), ok, "Accusation not added to map, test %d", i)

			require.Equalf(suite.T(), a.accused, t.acc,
				"Wrong accused id, test %d", i)

			require.Equalf(suite.T(), a.accuser, t.accuser,
				"Wrong accuser id, test %d", i)

			require.Equalf(suite.T(), a.epoch, t.p.note.epoch,
				"Wrong epoch, test %d", i)

			require.Equalf(suite.T(), a.ringNum, t.ringNum,
				"Wrong ringNum, test %d", i)

			require.NotNilf(suite.T(), a.signature, "Signature should not be nil, test %d", i)

			require.Equalf(suite.T(), a.signature.r, t.r,
				"Wrong signature r component, test %d", i)

			require.Equalf(suite.T(), a.signature.s, t.s,
				"Wrong signature s component, test %d", i)

		} else {
			acc := t.p.accusations[t.ringNum]
			require.Nil(suite.T(), acc,
				"Accusation should not be added when error is returned, test %d", i)
		}
	}
}

func (suite *PeerTestSuite) TestRemoveAccusation() {
	p := suite.p

	nonExistingAcc := &Accusation{
		ringNum: suite.numRings,
	}

	invalidRingNumAcc := &Accusation{
		ringNum: suite.numRings + 1,
	}

	validAcc := &Accusation{
		ringNum: suite.numRings - 1,
	}

	p.accusations[validAcc.ringNum] = validAcc

	tests := []struct {
		in     *Accusation
		pre    bool
		exists bool
	}{
		{
			in:     nonExistingAcc,
			pre:    false,
			exists: false,
		},

		{
			in:     invalidRingNumAcc,
			pre:    false,
			exists: false,
		},

		{
			in:     validAcc,
			pre:    true,
			exists: false,
		},
	}

	for i, t := range tests {
		acc := p.accusations[t.in.ringNum]

		if !t.pre {
			require.Nilf(suite.T(), acc, "Invalid pre status for test %d", i)
		} else {
			require.NotNilf(suite.T(), acc, "Invalid pre status for test %d", i)
		}

		p.RemoveAccusation(t.in)

		acc = p.accusations[t.in.ringNum]

		if !t.exists {
			require.Nilf(suite.T(), acc, "Invalid existence status for test %d", i)
		} else {
			require.NotNilf(suite.T(), acc, "Invalid existence status for test %d", i)
		}
	}
}

func (suite *PeerTestSuite) TestRemoveRingAccusation() {
	p := suite.p

	acc := &Accusation{
		ringNum: suite.numRings,
	}

	p.accusations[acc.ringNum] = acc

	tests := []struct {
		in     uint32
		pre    bool
		exists bool
	}{
		{
			in:     acc.ringNum,
			pre:    true,
			exists: false,
		},

		{
			in:     suite.numRings + 1,
			pre:    false,
			exists: false,
		},

		{
			in:     suite.numRings - 1,
			pre:    false,
			exists: false,
		},
	}

	for i, t := range tests {
		acc := p.accusations[t.in]

		if !t.pre {
			require.Nilf(suite.T(), acc, "Invalid pre status for test %d", i)
		} else {
			require.NotNilf(suite.T(), acc, "Invalid pre status for test %d", i)
		}

		p.RemoveRingAccusation(t.in)

		acc = p.accusations[t.in]

		if !t.exists {
			require.Nilf(suite.T(), acc, "Invalid existence status for test %d", i)
		} else {
			require.NotNilf(suite.T(), acc, "Invalid existence status for test %d", i)
		}
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
		assert.Equal(suite.T(), i, acc.ringNum,
			"Returned wrong accusation, different ring number.")
	}

	require.Nil(suite.T(), suite.p.RingAccusation(suite.numRings+1),
		"Returned non-nil accusation with invalid ring number.")
}

func (suite *PeerTestSuite) TestAnyAccusations() {
	assert.Nil(suite.T(), suite.p.AnyAccusation(),
		"Returned non-nil accusations, when non exists.")

	a := &Accusation{
		ringNum: suite.numRings,
	}
	suite.p.accusations[a.ringNum] = a

	acc := suite.p.AnyAccusation()
	require.NotNil(suite.T(), acc, "Returned nil accusation when there exists 1")

	assert.Equal(suite.T(), acc.ringNum, a.ringNum,
		"Returned an accusation with different ring number.")
}

func (suite *PeerTestSuite) TestAllAccusations() {
	var i uint32

	accs := suite.p.AllAccusations()
	require.NotNil(suite.T(), accs, "Returned nil slice with no accusations present.")
	assert.Zero(suite.T(), len(accs),
		"Returned slice with non-zero length with no accusations existed.")

	a := &Accusation{
		ringNum: suite.numRings,
	}
	suite.p.accusations[a.ringNum] = a

	accs2 := suite.p.AllAccusations()
	require.NotNil(suite.T(), accs2, "Returned nil slice with accusations present.")
	assert.Equal(suite.T(), 1, len(accs2),
		"Returned slice with different length than amount of accusations.")

	for i = 1; i <= suite.numRings; i++ {
		a := &Accusation{
			ringNum: i,
		}
		suite.p.accusations[a.ringNum] = a
	}

	accs3 := suite.p.AllAccusations()
	require.NotNil(suite.T(), accs3, "Returned nil slice with accusations present.")
	assert.Equal(suite.T(), int(suite.numRings), len(accs3),
		"Returned slice with different length than amount of accusations.")

}

func (suite *PeerTestSuite) TestIsAccused() {
	require.False(suite.T(), suite.p.IsAccused(), "Returned true when peer is not accused.")

	suite.p.accusations[suite.numRings] = &Accusation{ringNum: suite.numRings}

	require.True(suite.T(), suite.p.IsAccused(), "Returned false when peer is accused")
}

func (suite *PeerTestSuite) TestAddNote() {
	suite.p.note = nil

	tests := []struct {
		p       *Peer
		mask    uint32
		epoch   uint64
		r       []byte
		s       []byte
		replace bool
	}{
		{
			p:       suite.p,
			mask:    math.MaxUint32,
			epoch:   1,
			r:       []byte("signature r"),
			s:       []byte("signature s"),
			replace: true,
		},

		{
			p:       suite.p,
			mask:    math.MaxUint32,
			epoch:   2,
			r:       []byte("signature r"),
			s:       []byte("signature s"),
			replace: true,
		},

		{
			p:       suite.p,
			mask:    math.MaxUint32,
			epoch:   2,
			r:       []byte("signature r"),
			s:       []byte("signature s"),
			replace: false,
		},

		{
			p:       suite.p,
			mask:    math.MaxUint32,
			epoch:   1,
			r:       []byte("signature r"),
			s:       []byte("signature s"),
			replace: false,
		},
	}

	for i, t := range tests {
		old := t.p.Note()
		t.p.AddNote(t.mask, t.epoch, t.r, t.s)
		new := t.p.Note()

		if t.replace {
			require.Equalf(suite.T(), t.epoch, new.epoch, "Epoch not updated, test %d", i)
			require.Equalf(suite.T(), t.mask, new.mask, "Mask not updated, test %d", i)
			require.Equalf(suite.T(), t.r, new.signature.r,
				"Signature r not updated, test %d", i)
			require.Equalf(suite.T(), t.s, new.signature.s,
				"Signature s not updated, test %d", i)
		} else {
			require.Equalf(suite.T(), old, new,
				"Should be equal when not replacing notes, test %d", i)
		}
	}
}

func (suite *PeerTestSuite) TestNote() {
	p := suite.p

	assert.Equal(suite.T(), p.note, p.Note(), "Returns wrong note address value.")

	newNote := &Note{}
	p.note = newNote

	assert.Equal(suite.T(), newNote, p.Note(),
		"Returns wrong note address value after replacing note.")
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

type signerMock struct {
	priv *ecdsa.PrivateKey
}

func (sm *signerMock) Verify(data, r, s []byte, pub *ecdsa.PublicKey) bool {
	if pub == nil {
		log.Error("Peer had no publicKey")
		return false
	}

	var rInt, sInt big.Int

	b := hashContent(data)

	rInt.SetBytes(r)
	sInt.SetBytes(s)

	return ecdsa.Verify(pub, b, &rInt, &sInt)
}

func (sm *signerMock) Sign(data []byte) ([]byte, []byte, error) {
	hash := hashContent(data)

	r, s, err := ecdsa.Sign(rand.Reader, sm.priv, hash)
	if err != nil {
		return nil, nil, err
	}

	return r.Bytes(), s.Bytes(), nil
}
