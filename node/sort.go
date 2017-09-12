package node


import (
	"math/big"
	_"fmt"
)

func insert(slice []*ringId, newId *ringId) ([]*ringId, int){
	length := len(slice)
	currIdx := 0
	maxIdx := length - 1

	if maxIdx == -1 {
		slice = append(slice, newId)
		return slice, currIdx
	}

	for {
		if currIdx >= maxIdx {
			slice = append(slice, nil)
			copy(slice[currIdx+1:], slice[currIdx:])

			if slice[currIdx].id.Cmp(newId.id) == -1 {
				currIdx += 1
			}
			slice[currIdx] = newId

			return slice, currIdx
		}
		mid := (currIdx + maxIdx) / 2

		if slice[mid].id.Cmp(newId.id) == -1 {
			currIdx = mid + 1
		} else {
			maxIdx = mid - 1
		}
	}
}


func test(input []*ringId, num *big.Int) int {
	var idx int
	for idx, val := range input {
		if val.id.Cmp(num) == -1 {
			return idx
		}
	}
	return idx
}
