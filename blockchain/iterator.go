package blockchain

import (
	"github.com/dgraph-io/badger"
	"github.com/mkohlhaas/golang-blockchain/bcerror"
)

// iterator interface
type iterator interface {
	HasNext() bool
	GetNext() *Block
}

// Blockchain iterator
type BlockChainIterator struct {
	currentBlock *Block
	database     *badger.DB
}

// Creates new iterator from a blockchain.
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
	return &BlockChainIterator{
		currentBlock: block,
		database:     bc.Database,
	}
}

// Returns true if there is another block in the blockchain.
func (iter *BlockChainIterator) HasNext() bool {
	return iter.currentBlock.IsNotGenesisBlock()
}

// Returns the next block from the blockchain.
func (iter *BlockChainIterator) GetNext() *Block {
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
