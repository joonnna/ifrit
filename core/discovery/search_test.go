package discovery

import (
	"os"
	"testing"

	log "github.com/inconshreveable/log15"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SearchTestSuite struct {
	suite.Suite

	succList []*ringId
}

func TestSearchTestSuite(t *testing.T) {
	r := log.Root()

	r.SetHandler(log.CallerFileHandler(log.StreamHandler(os.Stdout, log.TerminalFormat())))

	suite.Run(t, new(SearchTestSuite))
}

func (suite *SearchTestSuite) SetupTest() {
	var prev, prevId *ringId
	var succList []*ringId
	var hash []byte

	for i := 0; i < 100; i++ {
		if prev != nil {
			hash = genHigherId(prev.hash, 2)
		} else {
			hash = hashId(1, []byte("testId"))
		}

		id := &ringId{
			hash: hash,
		}

		succList = append(succList, id)

		prev = id
	}

	for _, rId := range succList {
		if prevId != nil {
			require.Equal(suite.T(), 1, rId.compare(prevId), "Higher index should have higher id.")
		}
		prevId = rId
	}

	suite.succList = succList
}

func (suite *SearchTestSuite) TestInsert() {
	var insertIdx int
	var prev *ringId

	id := &ringId{
		hash: hashId(1, []byte("testId")),
	}

	slice, idx := insert(nil, id, 0)
	require.NotNil(suite.T(), slice, "Returned slice was nil.")
	require.Equal(suite.T(), 1, len(slice), "Returned slice should have 1 element.")
	assert.Equal(suite.T(), id, slice[0], "Did not add the supplied id to slice.")
	assert.Zero(suite.T(), idx, "New element should have index 0.")

	prevSlice := make([]*ringId, 0, len(suite.succList))
	copy(prevSlice, suite.succList)

	for _, rId := range prevSlice {
		newId := &ringId{
			hash: genHigherId(rId.hash, 1),
		}

		foundBigger := false

		for i, rId2 := range suite.succList {
			if cmp := rId2.compare(newId); cmp == 1 {
				insertIdx = i
				foundBigger = true
				break
			} else if cmp == 0 {
				log.Error("equal", "idx", i)
			}
		}

		if !foundBigger {
			insertIdx = len(suite.succList)
		}

		prevLen := len(suite.succList)
		slice, idx = insert(suite.succList, newId, prevLen)
		assert.NotNil(suite.T(), slice, "Returned slice was nil.")
		assert.Equal(suite.T(), prevLen+1, len(slice), "New element not added.")

		require.Equal(suite.T(), idx, insertIdx, "Element not added at appropriate index.")
		suite.succList = slice
	}

	for _, rId := range suite.succList {
		if prev != nil {
			require.Equal(suite.T(), 1, rId.compare(prev), "Higher index should have higher id.")
		}
		prev = rId
	}
}

func (suite *SearchTestSuite) TestSearch() {
	idx, err := search(nil, nil, 0)
	assert.EqualError(suite.T(), err, errZeroLength.Error(), "Should return error with zero length.")
	assert.Zero(suite.T(), idx, "Should return zero index when returning an error.")

	id := &ringId{
		hash: hashId(1, []byte("anotherTestId")),
	}

	idx, err = search(suite.succList, id, len(suite.succList))
	assert.EqualError(suite.T(), err, errIdNotFound.Error(), "Should return error with searching for a non-existing id.")
	assert.Zero(suite.T(), idx, "Should return zero index when returning an error.")

	for i, rId := range suite.succList {
		idx, err = search(suite.succList, rId, len(suite.succList))
		require.NoError(suite.T(), err, "Returned error when searching for existing id.")
		require.Equal(suite.T(), i, idx, "Returned wrong index.")
	}
}

func (suite *SearchTestSuite) TestFindSuccAndPrev() {
	var succ, prev *ringId
	var insertIdx int

	length := len(suite.succList)

	for i, rId := range suite.succList {
		if i == length-1 {
			succ = suite.succList[0]
			prev = suite.succList[i-1]
		} else if i == 0 {
			succ = suite.succList[i+1]
			prev = suite.succList[length-1]
		} else {
			succ = suite.succList[i+1]
			prev = suite.succList[i-1]
		}

		findSucc, findPrev := findSuccAndPrev(suite.succList, rId, length)
		require.Equal(suite.T(), findSucc, succ, "Found wrong successor.")
		require.Equal(suite.T(), findPrev, prev, "Found wrong predecessor.")
	}

	for _, rId := range suite.succList {
		id := &ringId{
			hash: genHigherId(rId.hash, 1),
		}

		foundBigger := false

		for i, rId2 := range suite.succList {
			if rId2.compare(id) == 1 {
				insertIdx = i
				foundBigger = true
				break
			}
		}

		if !foundBigger {
			insertIdx = length
		}

		if insertIdx == length {
			succ = suite.succList[0]
			prev = suite.succList[length-1]
		} else if insertIdx == 0 {
			succ = suite.succList[0]
			prev = suite.succList[length-1]
		} else {
			succ = suite.succList[insertIdx]
			prev = suite.succList[insertIdx-1]
		}

		findSucc, findPrev := findSuccAndPrev(suite.succList, id, length)
		require.Equal(suite.T(), findSucc, succ, "Found wrong successor.")
		require.Equal(suite.T(), findPrev, prev, "Found wrong predecessor.")
	}

}

func (suite *SearchTestSuite) TestSucc() {
	var successor *ringId

	length := len(suite.succList)

	for i, _ := range suite.succList {
		if i == length-1 {
			successor = suite.succList[0]
		} else if i == 0 {
			successor = suite.succList[i+1]
		} else {
			successor = suite.succList[i+1]
		}

		assert.Equal(suite.T(), successor, succ(suite.succList, i, length), "Returned wrong successor.")
	}
}

func (suite *SearchTestSuite) TestPrev() {
	var predecessor *ringId

	length := len(suite.succList)

	for i, _ := range suite.succList {
		if i == length-1 {
			predecessor = suite.succList[i-1]
		} else if i == 0 {
			predecessor = suite.succList[length-1]
		} else {
			predecessor = suite.succList[i-1]
		}

		assert.Equal(suite.T(), predecessor, prev(suite.succList, i, length), "Returned wrong predecessor.")
	}
}
