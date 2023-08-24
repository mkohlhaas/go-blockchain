package blockchain

import (
	"github.com/dgraph-io/badger"
	"github.com/mkohlhaas/golang-blockchain/bcerror"
)

type iterator interface {
	HasNext() bool
	GetNext() *Block
}

type blockChainIterator struct {
	currentBlock *Block
	database     *badger.DB
}

// CreateBCIterator creates new iterator from a blockchain.
func (bc *BlockChain) CreateBCIterator() iterator {
	var block *Block
	err := bc.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(bc.LastHash)
		bcerror.Handle(err)
		var encodedBlock []byte
		err = item.Value(func(val []byte) error {
			encodedBlock = append([]byte{}, val...)
			return nil
		})
		block = DeserializeBlock(encodedBlock)
		return err
	})
	bcerror.Handle(err)
	return &blockChainIterator{
		currentBlock: block,
		database:     bc.Database,
	}
}

func (iter *blockChainIterator) HasNext() bool {
	return iter.currentBlock.isNotGenesisBlock()
}

func (iter *blockChainIterator) GetNext() *Block {
	var block *Block
	err := iter.database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.currentBlock.PrevHash)
		bcerror.Handle(err)
		var encodedBlock []byte
		err = item.Value(func(val []byte) error {
			encodedBlock = append([]byte{}, val...)
			return nil
		})
		block = DeserializeBlock(encodedBlock)
		return err
	})
	bcerror.Handle(err)
	iter.currentBlock = block
	return block
}
