package blockchain

import (
	"github.com/dgraph-io/badger"
	"github.com/mkohlhaas/golang-blockchain/bcerror"
)

type iterator interface {
	HasNext() bool
	GetNext() *Block
}

type BlockChainIterator struct {
	CurrentBlock *Block
	Database     *badger.DB
}

func (bc *BlockChain) CreateBCIterator() iterator {
	var block *Block
	err := bc.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(bc.LastHash)
		bcerror.Handle(err)
		encodedBlock, err := item.Value()
		block = Deserialize(encodedBlock)
		return err
	})
	bcerror.Handle(err)
	return &BlockChainIterator{
		CurrentBlock: block,
		Database:     bc.Database,
	}
}

func (iter *BlockChainIterator) HasNext() bool {
	return iter.CurrentBlock.Height != 0
}

func (iter *BlockChainIterator) GetNext() *Block{
	var block *Block
	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentBlock.PrevHash)
		bcerror.Handle(err)
		encodedBlock, err := item.Value()
		block = Deserialize(encodedBlock)
		return err
	})
	bcerror.Handle(err)
  iter.CurrentBlock = block
	return block
}
