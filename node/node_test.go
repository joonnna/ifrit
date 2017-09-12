package node


import (
	"testing"
	"math/big"
	"fmt"
)


func TestSort(t *testing.T) {
	newSlice := make([]*ringId, 0)

	var i int64

	for i = 0; i < 21; i+= 3 {
		newInt := big.NewInt(i)

		id := &ringId{
			id: newInt,
		}

		newSlice, _ = insert(newSlice, id)

		for _, val := range newSlice {
			fmt.Println(val.id.Int64())
		}
		fmt.Printf("LEN :%d", len(newSlice))
		fmt.Println("\n\n")
	}
}
