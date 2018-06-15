package discovery

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func genHigherId(id []byte) []byte {
	var val big.Int

	tmp := make([]byte, len(id))

	one := big.NewInt(1)

	copy(tmp, id)
	tmpVal := val.SetBytes(tmp)

	val.Add(tmpVal, one)

	return val.Bytes()
}

func genLowerId(id []byte) []byte {
	var val big.Int

	tmp := make([]byte, len(id))

	one := big.NewInt(1)

	copy(tmp, id)

	tmpVal := val.SetBytes(tmp)

	val.Sub(tmpVal, one)

	return val.Bytes()
}

type RingTestSuite struct {
	suite.Suite
	*ring
}

func (suite *RingTestSuite) SetupTest() {
	var err error
	addr := "localhost:1234"
	id := genId()

	suite.ring, err = newRing(1, id, addr)
	require.NoError(suite.T(), err, "Failed to create ring")
	require.Equal(suite.T(), suite.ring.ownIdx, 0)
}

func TestRingTestSuite(t *testing.T) {
	suite.Run(t, new(RingTestSuite))
}

func (suite *RingTestSuite) TestValidAdd() {
	id := genId()
	addr := "localhost:8000"

	err := suite.add(id, addr)
	assert.NoError(suite.T(), err, "Valid add fails")
}

func (suite *RingTestSuite) TestAddSameIdTwice() {
	id := genId()
	addr := "localhost:8000"

	err := suite.add(id, addr)
	require.NoError(suite.T(), err, "Valid add returns error")

	err = suite.add(id, addr)
	assert.Error(suite.T(), err, "Adding same id twice returns non-nil error")
}

func (suite *RingTestSuite) TestAddSelf() {
	err := suite.add([]byte(suite.localRingId.key), suite.localRingId.addr)
	assert.Error(suite.T(), err, "Adding self returns non-nil error")
}

func (suite *RingTestSuite) TestOwnIdxOnFailedAdd() {
	val := suite.ownIdx

	err := suite.add([]byte(suite.localRingId.key), suite.localRingId.addr)
	require.Error(suite.T(), err, "Adding self returns non-nil error")

	assert.Equal(suite.T(), val, suite.ownIdx, "ownIdx changed on failed add")
}

func (suite *RingTestSuite) TestAddUpdatesExistMap() {
	id := genId()
	key := string(id[:])
	addr := "localhost:8000"

	err := suite.add(id, addr)
	require.NoError(suite.T(), err, "Valid add fails")

	assert.True(suite.T(), suite.existsMap[key], "Existmap not updated")
}

func (suite *RingTestSuite) TestValidRemove() {
	id := genId()
	addr := "localhost:8000"

	err := suite.add(id, addr)
	require.NoError(suite.T(), err, "Valid add fails")

	err = suite.remove(id)
	assert.NoError(suite.T(), err, "Valid remove fails")
}

func (suite *RingTestSuite) TestRemoveSelf() {
	err := suite.remove(suite.localRingId.id)
	assert.Error(suite.T(), err, "Removing self returns non-nil error")
}

func (suite *RingTestSuite) TestRemoveUpdatesExistMap() {
	id := genId()
	key := string(id[:])
	addr := "localhost:8000"

	err := suite.add(id, addr)
	require.NoError(suite.T(), err, "Valid add fails")

	require.True(suite.T(), suite.existsMap[key], "Existmap not updated on add")

	err = suite.remove(id)
	require.NoError(suite.T(), err, "Valid remove fails")

	assert.False(suite.T(), suite.existsMap[key], "Existmap not updated on remove")
}

func (suite *RingTestSuite) TestIsBetween() {
	node := &ringId{
		id: genId(),
	}

	prev := &ringId{
		id: genLowerId(node.id),
	}

	succ := &ringId{
		id: genHigherId(node.id),
	}

	assert.False(suite.T(), isBetween(node, succ, prev), "prev should not be between node and succ")
	assert.False(suite.T(), isBetween(prev, node, succ), "succ should not be between node and prev")
	assert.True(suite.T(), isBetween(node, prev, succ), "wrap around not working")
	assert.True(suite.T(), isBetween(succ, node, prev), "wrap around not working")
	assert.True(suite.T(), isBetween(succ, node, node), "equal end and new should return true")
	assert.True(suite.T(), isBetween(succ, succ, node), "equal start and end should return true")
}

func (suite *RingTestSuite) TestBetweenNeighboursWhenAlone() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")

	assert.True(suite.T(), suite.betweenNeighbours(p.id), "Should be between when alone")
}

func (suite *RingTestSuite) TestBetweenNeighboursWithTwo() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")

	err := suite.add(p.id, p.addr)
	require.NoError(suite.T(), err, "Valid add fails")

	assert.True(suite.T(), suite.betweenNeighbours(p.id), "Should be between when only 2 in ring")

}

func (suite *RingTestSuite) TestBetweenNeighbours() {
	pSlice := make([]*peer, 0)
	for i := 0; i < 10; i++ {
		id := genId()
		p, _ := newTestPeer(string(id[:]), 1, "")
		pSlice = append(pSlice, p)

		err := suite.add(p.id, p.addr)
		require.NoError(suite.T(), err, "Valid add fails")
	}

	succ, err := suite.getRingSucc()
	require.NoError(suite.T(), err, "Can't find successor")

	prev, err := suite.getRingPrev()
	require.NoError(suite.T(), err, "Can't find prev")

	for _, p := range pSlice {
		if p.key != succ.key && p.key != prev.key {
			assert.False(suite.T(), suite.betweenNeighbours(p.id), "Shouldn't be neighbours")
		} else {
			assert.True(suite.T(), suite.betweenNeighbours(p.id), "Should be neighbours")
		}
	}
}

func (suite *RingTestSuite) TestFindNeighboursWhenAlone() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")

	succ, prev, err := suite.findNeighbours(p.id)
	require.NoError(suite.T(), err, "Failed to find neighbours when alone")
	assert.Equal(suite.T(), succ, suite.localRingId.key, "I should be successor when alone")
	assert.Equal(suite.T(), prev, suite.localRingId.key, "I should be prev when alone")
}

func (suite *RingTestSuite) TestFindNeighbours() {
	peers := make(map[string]bool)
	peers[suite.localRingId.key] = true

	pSlice := make([]*peer, 0)
	for i := 0; i < 10; i++ {
		id := genId()
		p, _ := newTestPeer(string(id[:]), 1, "")
		pSlice = append(pSlice, p)
		peers[p.key] = true

		err := suite.add(p.id, p.addr)
		require.NoError(suite.T(), err, "Valid add fails")
	}

	for _, p := range pSlice {
		succ, prev, err := suite.findNeighbours(p.id)
		require.NoError(suite.T(), err, "Failed to find neighbours")

		_, succExist := peers[succ]
		_, prevExist := peers[prev]

		assert.True(suite.T(), succExist, "found invalid successor neighbour")
		assert.True(suite.T(), prevExist, "found invalid prev neighbour")
	}
}

func (suite *RingTestSuite) TestFindPrevWhenAlone() {
	id := []byte(suite.localRingId.key)
	prev, err := suite.findPrev(id)
	require.NoError(suite.T(), err, "Failed to find prev when alone")
	assert.Equal(suite.T(), prev, suite.localRingId.key, "I should be prev when alone")
}

func (suite *RingTestSuite) TestFindPrevWhenTwoInRing() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")
	err := suite.add(p.id, p.addr)
	require.NoError(suite.T(), err, "Valid add fails")

	prev, err := suite.findPrev([]byte(suite.localRingId.key))
	require.NoError(suite.T(), err, "Failed to find prev when 2 in ring")
	assert.Equal(suite.T(), prev, p.key, "Should be prev when only 2 in ring")
}

func (suite *RingTestSuite) TestFindPrevWhenNotInRing() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")

	_, err := suite.findPrev(p.id)
	require.Error(suite.T(), err, "Should fail to find prev of non-existing id")
}

func (suite *RingTestSuite) TestFindPrev() {
	peers := make(map[string]bool)
	peers[suite.localRingId.key] = true

	pSlice := make([]*peer, 0)
	for i := 0; i < 10; i++ {
		id := genId()
		p, _ := newTestPeer(string(id[:]), 1, "")
		pSlice = append(pSlice, p)
		peers[p.key] = true

		err := suite.add(p.id, p.addr)
		require.NoError(suite.T(), err, "Valid add fails")
	}

	for _, p := range pSlice {
		prev, err := suite.findPrev(p.id)
		require.NoError(suite.T(), err, "Failed to find prev")

		_, prevExist := peers[prev]

		assert.True(suite.T(), prevExist, "found invalid prev")
	}
}

func (suite *RingTestSuite) TestIsPrevMyself() {
	id := []byte(suite.localRingId.key)
	prev, err := suite.isPrev(id, id)
	require.NoError(suite.T(), err, "Failed to find prev when alone")
	assert.True(suite.T(), prev, "I should be prev when alone")
}

func (suite *RingTestSuite) TestIsPrevWhenNotInRing() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")
	_, err := suite.isPrev(p.id, p.id)
	require.Error(suite.T(), err, "Should fail to find prev when id is not in ring")
}

func (suite *RingTestSuite) TestIsPrevWhenTwoInRing() {
	p, _ := newTestPeer("test1234", 1, "localhost:123")
	err := suite.add(p.id, p.addr)
	require.NoError(suite.T(), err, "Valid add fails")

	prev, err := suite.isPrev([]byte(suite.localRingId.key), p.id)
	require.NoError(suite.T(), err, "Failed to find prev when 2 in ring")
	assert.True(suite.T(), prev, "Should be prev when only 2 in ring")
}

func (suite *RingTestSuite) TestIsPrev() {
	pSlice := make([]*peer, 0)
	for i := 0; i < 10; i++ {
		id := genId()
		p, _ := newTestPeer(string(id[:]), 1, "")
		pSlice = append(pSlice, p)

		err := suite.add(p.id, p.addr)
		require.NoError(suite.T(), err, "Valid add fails")
	}

	for _, p := range pSlice {
		prev, err := suite.findPrev(p.id)
		require.NoError(suite.T(), err, "Couldn't find prev")

		for _, p2 := range pSlice {
			isPrev, err := suite.isPrev(p.id, p2.id)
			require.NoError(suite.T(), err, "Couldn't check who is prev")
			if p2.key == prev {
				assert.True(suite.T(), isPrev, "Should be prev")
			} else {
				assert.False(suite.T(), isPrev, "Should not be prev")
			}

		}
	}
}
