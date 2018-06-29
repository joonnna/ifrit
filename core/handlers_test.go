package core

import (
	"net/rpc"
	"os"
	"testing"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/rpc"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type HandlerTestSuite struct {
	suite.Suite
	n *Node
}

func TestHandlerTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	suite.Run(t, new(HandlerTestSuite))
}

func (suite *HandlerTestSuite) SetupTest() {
	c := rpc.NewClient()

	s, err := rpc.NewServer()
	require.NoError(suite.T(), err, "Failed to create rpc server.")

	n, err := NewNode(c, s)
	require.NoError(suite.T(), err, "Failed to create node.")

	suite.n = n
}

func (suite *HandlerTestSuite) TestSpread() {

}

func (suite *HandlerTestSuite) TestMessenger() {

}

func (suite *HandlerTestSuite) TestMergeViews() {

}

func (suite *HandlerTestSuite) TestMergeNotes() {

}

func (suite *HandlerTestSuite) TestMergeAccusations() {

}

func (suite *HandlerTestSuite) TestMergeCertificates() {

}

func (suite *HandlerTestSuite) TestEvalAccusation() {

}

func (suite *HandlerTestSuite) TestEvalAccusation() {

}

func (suite *HandlerTestSuite) TestEvalNote() {

}

func (suite *HandlerTestSuite) TestEvalCertificate() {

}

func (suite *HandlerTestSuite) TestValidateCtx() {

}

/*
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

	suite.viewMap[p.key] = p

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

	suite.viewMap[p.key] = p

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

	suite.viewMap[p.key] = p

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

	suite.viewMap[p.key] = p

	suite.evalNote(noteMsg)

	assert.Equal(suite.T(), p.getNote().epoch, newNote.epoch, "Valid note does not act as rebuttal")
}

func (suite *NodeTestSuite) TestAccusedNotInView() {
	accused, _ := newTestPeer("accused1234", suite.numRings, "localhost:123")

	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.viewMap[accuser.key] = accuser

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

	suite.viewMap[accused.key] = accused

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

	suite.viewMap[accused.key] = accused
	suite.viewMap[accuser.key] = accuser

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
}

func (suite *NodeTestSuite) TestInvaldAccDoesNotStartTimer() {
	accused, _ := newTestPeer("accused1234", suite.numRings, "localhost:123")

	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.viewMap[accused.key] = accused
	suite.viewMap[accuser.key] = accuser

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

func (suite *NodeTestSuite) TestInvalidMask() {
	accused, _ := newTestPeer("accused1234", suite.numRings, "localhost:123")

	accuser, priv := newTestPeer("accuser1234", suite.numRings, "localhost:124")

	suite.viewMap[accused.key] = accused
	suite.viewMap[accuser.key] = accuser

	suite.addLivePeer(accused)
	suite.addLivePeer(accuser)

	n := accused.getNote()

	acc := &accusation{
		epoch:   n.epoch,
		mask:    0,
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

	suite.viewMap[accuser.key] = accuser

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
	var ringNum uint32 = 1

	log.Debug("mask", "val", suite.recentNote.mask)
	log.Debug("Deactivated", "val", suite.deactivatedRings)
	suite.deactivateRing(ringNum)
	log.Debug("Deactivated", "val", suite.deactivatedRings)
	log.Debug("mask", "val", suite.recentNote.mask)

	assert.Equal(suite.T(), suite.deactivatedRings, uint32(1), "Deactivated rings not incremented.")
	assert.False(suite.T(), hasBit(suite.recentNote.mask, (ringNum-1)), "Deactivated ring still has bit present.")
}

func (suite *NodeTestSuite) TestDeactivateInvalidRing() {
	var ringNum, i uint32
	ringNum = 312312
	suite.deactivateRing(ringNum)

	assert.Equal(suite.T(), suite.deactivatedRings, uint32(0), "Deactivated rings incremented on invalid deactivation")

	for i = 0; i < suite.numRings; i++ {
		assert.True(suite.T(), hasBit(suite.recentNote.mask, i), "Ring deactivated on invalid deactivation.")
	}
}

func (suite *NodeTestSuite) TestDeactivateTooManyRings() {
	var r1 uint32 = 1
	var r2 uint32 = 2
	var r3 uint32 = 3

	suite.deactivateRing(r1)
	suite.deactivateRing(r2)

	assert.Equal(suite.T(), suite.deactivatedRings, suite.Node.maxByz, "Allowed to deactivate too many rings.")
	assert.False(suite.T(), hasBit(suite.recentNote.mask, r1-1), "Failed to deactivate first ring")
	assert.False(suite.T(), hasBit(suite.recentNote.mask, r2-1), "Failed to deactivate second ring")

	suite.deactivateRing(r3)
	assert.False(suite.T(), hasBit(suite.recentNote.mask, r3-1), "Failed to deactivate ring when exceeding limit")

	if hasBit(suite.recentNote.mask, r1-1) {
		assert.False(suite.T(), hasBit(suite.recentNote.mask, r2-1), "Failed to re-activate ring upon exceeding limit.")
	} else {
		assert.False(suite.T(), hasBit(suite.recentNote.mask, r1-1), "Failed to re-activate ring upon exceeding limit.")
	}

}
*/
