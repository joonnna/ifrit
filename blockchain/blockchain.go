package blockchain

type chain struct {
	blocks []*block
}

type block struct {
	prev []byte

	entries []*entry
	num     uint64
}

type entry struct {
	data []byte
}

func (c *chain) appendToChain(data []byte) error {
	return nil
}

func (b *block) addToBlock(data []byte) error {
	return nil
}
