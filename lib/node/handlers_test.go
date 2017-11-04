package node

import (
	"crypto/ecdsa"

	"github.com/golang/protobuf/proto"
	"github.com/joonnna/firechain/lib/protobuf"
	"github.com/stretchr/testify/assert"
)

func newPbNote(n *note, priv *ecdsa.PrivateKey) *gossip.Note {
	noteMsg := &gossip.Note{
		Epoch: n.epoch,
		Id:    n.id,
		Mask:  n.mask,
	}

	b, err := proto.Marshal(noteMsg)
	if err != nil {
		panic(err)
	}

	signature, err := signContent(b, priv)
	if err != nil {
		panic(err)
	}

	n.signature = signature

	noteMsg.Signature = &gossip.Signature{
		R: signature.r,
		S: signature.s,
	}

	return noteMsg
}

func newPbAcc(a *accusation, priv *ecdsa.PrivateKey) *gossip.Accusation {
	acc := &gossip.Accusation{
		Epoch:   a.epoch,
		Accused: a.id,
		Mask:    a.mask,
		Accuser: a.accuser.id,
		RingNum: a.ringNum,
	}

	b, err := proto.Marshal(acc)
	if err != nil {
		panic(err)
	}

	signature, err := signContent(b, priv)
	if err != nil {
		panic(err)
	}

	a.signature = signature

	acc.Signature = &gossip.Signature{
		R: signature.r,
		S: signature.s,
	}

	return acc
}

func (suite *NodeTestSuite) TestAddValidNote() {
	p, priv := newTestPeer("test1234", suite.numRings, "localhost:123")

	prevNote := p.getNote()

	newNote := &note{
		epoch:  prevNote.epoch + 1,
		mask:   prevNote.mask,
		peerId: p.peerId,
	}

	noteMsg := newPbNote(newNote, priv)

	suite.addViewPeer(p)

	suite.evalNote(noteMsg)

	assert.NotNil(suite.T(), p.getNote(), "Valid note not added to peer, eval failed")
	assert.NotNil(suite.T(), suite.getLivePeer(p.key), "Valid not dosen't add peer to live")
	assert.Equal(suite.T(), p.getNote().epoch, newNote.epoch, "New note with higher epoch does not replace old one")
}

func (suite *NodeTestSuite) TestAddInvalidNote() {
	p, priv := newTestPeer("test1234", suite.numRings, "localhost:123")

	prevNote := p.getNote()

	newNote := &note{
		epoch:  prevNote.epoch - 1,
		mask:   prevNote.mask,
		peerId: p.peerId,
	}

	noteMsg := newPbNote(newNote, priv)

	suite.addViewPeer(p)

	suite.evalNote(noteMsg)

	assert.NotEqual(suite.T(), p.getNote().epoch, newNote.epoch, "Invalid note replaces a valid one")
}

func (suite *NodeTestSuite) TestAddNoteToNonExistingPeer() {
	p, priv := newTestPeer("test1234", suite.numRings, "localhost:123")

	prevNote := p.getNote()

	newNote := &note{
		epoch:  prevNote.epoch + 1,
		mask:   prevNote.mask,
		peerId: p.peerId,
	}

	noteMsg := newPbNote(newNote, priv)

	suite.evalNote(noteMsg)

	assert.NotEqual(suite.T(), p.getNote().epoch, newNote.epoch, "Note was added to non-existing peer")
}

func (suite *NodeTestSuite) TestInvalidRebuttal() {
	p, priv := newTestPeer("test1234", suite.numRings, "localhost:123")

	prevNote := p.getNote()

	a := &accusation{
		epoch:   prevNote.epoch,
		ringNum: 1,
		mask:    prevNote.mask,
	}

	err := p.setAccusation(a)
	assert.Nil(suite.T(), err)

	newNote := &note{
		epoch:  a.epoch - 1,
		mask:   prevNote.mask,
		peerId: p.peerId,
	}

	noteMsg := newPbNote(newNote, priv)

	suite.addViewPeer(p)

	suite.evalNote(noteMsg)

	assert.NotEqual(suite.T(), p.getNote().epoch, newNote.epoch, "Invalid note acts as rebuttal")
}

func (suite *NodeTestSuite) TestValidRebuttal() {
	p, priv := newTestPeer("test1234", suite.numRings, "localhost:123")

	prevNote := p.getNote()

	a := &accusation{
		epoch:   prevNote.epoch,
		ringNum: 1,
	}

	err := p.setAccusation(a)
	assert.Nil(suite.T(), err)

	newNote := &note{
		epoch:  a.epoch + 1,
		mask:   prevNote.mask,
		peerId: p.peerId,
	}

	noteMsg := newPbNote(newNote, priv)

	suite.addViewPeer(p)

	suite.evalNote(noteMsg)

	assert.Equal(suite.T(), p.getNote().epoch, newNote.epoch, "Valid note does not act as rebuttal")
}

func (suite *NodeTestSuite) TestAccusedNotInView() {
	accused, _ := newTestPeer("accused1234", suite.numRings, "localhost:123")

	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.addViewPeer(accuser)

	n := accused.getNote()

	acc := &accusation{
		epoch:   n.epoch,
		mask:    n.mask,
		peerId:  n.peerId,
		accuser: accuser.peerId,
		ringNum: 1,
	}

	accMsg := newPbAcc(acc, priv)

	suite.evalAccusation(accMsg)

	assert.Nil(suite.T(), accused.getAnyAccusation(), "Added accusation for peer not in view")
}

func (suite *NodeTestSuite) TestAccuserNotInView() {
	accused, _ := newTestPeer("accused1234", suite.numRings, "localhost:123")

	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.addViewPeer(accused)

	n := accused.getNote()

	acc := &accusation{
		epoch:   n.epoch,
		mask:    n.mask,
		peerId:  n.peerId,
		accuser: accuser.peerId,
		ringNum: 1,
	}

	accMsg := newPbAcc(acc, priv)

	suite.evalAccusation(accMsg)

	assert.Nil(suite.T(), accused.getAnyAccusation(), "Added accusation when accuser not in view")
}

func (suite *NodeTestSuite) TestValidAccStartsTimer() {
	accused, _ := newTestPeer("accused1234", suite.numRings, "localhost:123")

	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.addViewPeer(accused)
	suite.addViewPeer(accuser)
	suite.addLivePeer(accused)
	suite.addLivePeer(accuser)

	n := accused.getNote()

	acc := &accusation{
		epoch:   n.epoch,
		mask:    n.mask,
		peerId:  accused.peerId,
		accuser: accuser.peerId,
		ringNum: 1,
	}

	accMsg := newPbAcc(acc, priv)

	suite.evalAccusation(accMsg)

	assert.True(suite.T(), suite.timerExist(accused.key), "Valid accusation does not start timer")

	suite.log.Debug.Println(len(suite.getView()))
}

func (suite *NodeTestSuite) TestInvaldAccDoesNotStartTimer() {
	accused, _ := newTestPeer("accused1234", suite.numRings, "localhost:123")

	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.addViewPeer(accused)
	suite.addViewPeer(accuser)
	suite.addLivePeer(accused)
	suite.addLivePeer(accuser)

	n := accused.getNote()

	acc := &accusation{
		epoch:   n.epoch - 1,
		mask:    n.mask,
		peerId:  accused.peerId,
		accuser: accuser.peerId,
		ringNum: 1,
	}

	accMsg := newPbAcc(acc, priv)

	suite.evalAccusation(accMsg)

	assert.False(suite.T(), suite.timerExist(accused.key), "Invalid accusation starts timer")
}

func (suite *NodeTestSuite) TestNonExistingMask() {
	accused, _ := newTestPeer("accused1234", suite.numRings, "localhost:123")

	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.addViewPeer(accused)
	suite.addViewPeer(accuser)
	suite.addLivePeer(accused)
	suite.addLivePeer(accuser)

	n := accused.getNote()

	acc := &accusation{
		epoch:   n.epoch,
		mask:    nil,
		peerId:  accused.peerId,
		accuser: accuser.peerId,
		ringNum: 1,
	}

	accMsg := newPbAcc(acc, priv)

	suite.evalAccusation(accMsg)

	assert.False(suite.T(), suite.timerExist(accused.key), "Invalid accusation starts timer")
}

func (suite *NodeTestSuite) TestInvalidMask() {
	accused, _ := newTestPeer("accused1234", suite.numRings, "localhost:123")

	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.addViewPeer(accused)
	suite.addViewPeer(accuser)
	suite.addLivePeer(accused)
	suite.addLivePeer(accuser)

	n := accused.getNote()

	acc := &accusation{
		epoch:   n.epoch,
		mask:    make([]byte, suite.numRings),
		peerId:  accused.peerId,
		accuser: accuser.peerId,
		ringNum: 1,
	}

	accMsg := newPbAcc(acc, priv)

	suite.evalAccusation(accMsg)

	assert.False(suite.T(), suite.timerExist(accused.key), "Invalid accusation starts timer")
}

func (suite *NodeTestSuite) TestRebuttal() {
	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.setProtocol(1)

	suite.addViewPeer(accuser)

	n := suite.getNote()

	acc := &accusation{
		epoch:   n.epoch,
		mask:    n.mask,
		peerId:  n.peerId,
		accuser: accuser.peerId,
		ringNum: 1,
	}

	accMsg := newPbAcc(acc, priv)

	suite.evalAccusation(accMsg)

	assert.False(suite.T(), suite.timerExist(suite.key), "Accusation concerning yourself starts a timer")
	assert.Equal(suite.T(), suite.getNote().epoch, (acc.epoch + 1), "No rebuttal started when receiving accusation about yourself")
}

func (suite *NodeTestSuite) TestDeactivateRing() {
	var ringNum uint32
	ringNum = 0
	suite.deactivateRing(ringNum)

	assert.Equal(suite.T(), suite.deactivatedRings, uint32(1), "Deactivated rings not incremented")
	assert.Equal(suite.T(), suite.recentNote.mask[ringNum], uint8(0), "Deactivated rings not incremented")
}

func (suite *NodeTestSuite) TestDeactivateInvalidRing() {
	var ringNum uint32
	ringNum = 312312
	suite.deactivateRing(ringNum)

	assert.Equal(suite.T(), suite.deactivatedRings, uint32(0), "Deactivated rings incremented on invalid deactivation")

	for _, b := range suite.recentNote.mask {
		assert.Equal(suite.T(), b, uint8(1), "Deactivated ring on invalid deactivation")
	}
}

func (suite *NodeTestSuite) TestDeactivateTooManyRings() {
	var ringNum uint32
	ringNum = 0
	prevLen := len(suite.recentNote.mask)
	suite.deactivateRing(ringNum)
	suite.deactivateRing(ringNum + 1)

	assert.Equal(suite.T(), prevLen, len(suite.recentNote.mask), "mask changes length after deactivations ?!?!")
	assert.Equal(suite.T(), suite.deactivatedRings, uint32(1), "Deactivated rings reflects wrong value")
	assert.Equal(suite.T(), suite.recentNote.mask[ringNum], uint8(1), "Previous ring deactivation not re-activated")
	assert.Equal(suite.T(), suite.recentNote.mask[ringNum+1], uint8(0), "Ring not deactivated")

}
