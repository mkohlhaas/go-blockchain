package blockchain

import (
	"github.com/mkohlhaas/gobc/bcerror"
)

type iterator interface {
	HasNext() bool
	GetNext() *Block
}

type blockChainIterator struct {
	currentBlock *Block
	blockchain   *BlockChain
}

// CreateBCIterator creates new iterator from a blockchain.
func (bc *BlockChain) CreateBCIterator() iterator {
	lastBlock := bc.getLastBlock()
	return &blockChainIterator{
		currentBlock: lastBlock,
		blockchain:   bc,
	}
}

func (iter *blockChainIterator) HasNext() bool {
	return iter.currentBlock.isNotGenesisBlock()
}

func (iter *blockChainIterator) GetNext() *Block {
	nextBlock, err := iter.blockchain.GetBlock(iter.currentBlock.PrevHash)
	bcerror.Handle(err)
	iter.currentBlock = nextBlock
	return nextBlock
}
