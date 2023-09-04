// Package blockchain provides all necessary functions for dealing with the Bitcoin blockchain.
package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dgraph-io/badger"
	"github.com/mkohlhaas/gobc/bcerror"
)

type Hash []byte

// BlockChain is a wrapper around a key-value DB.
type BlockChain struct {
	// DB stores key-value pairs:
	// key:   block hash
	// value: block
	// Also entries for UTXOs with key prefix "utxo-"
	Database *badger.DB
}

const (
	dbPath      = "./tmp/blocks_%s"
	genesisData = "The Times 03/Jan/2009: Chancellor on brink of second bailout for banks."
)

// lastHashEntry is the key in the database for the last block.
var lastHashEntry = Hash("lhentry")

func (bc *BlockChain) getLastHash() Hash {
	var lastHash Hash
	err := bc.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(lastHashEntry)
		bcerror.Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = append(make([]byte, 0), val...)
			return nil
		})
		return err
	})
	bcerror.Handle(err)
	return lastHash
}

func (bc *BlockChain) getLastBlock() *Block {
	lastHash := bc.getLastHash()
	lastBlock, err := bc.GetBlock(lastHash)
	bcerror.Handle(err)
	return lastBlock
}

// AddBlock adds a block to the blockchain.
func (bc *BlockChain) AddBlock(block *Block) {
	lastBlock := bc.getLastBlock()
	_, err := bc.GetBlock(block.Hash)
	if err == nil {
		fmt.Printf("Block %x already in the blockchain.\n", block.Hash)
		return
	}
	// Write block into blockchain.
	bc.Database.Update(func(txn *badger.Txn) error {
		blockData := block.Serialize()
		err = txn.Set(block.Hash, blockData)
		bcerror.Handle(err)
		// The new block should have a height bigger than the last block.
		if block.Height > lastBlock.Height {
			err = txn.Set(lastHashEntry, block.Hash)
			bcerror.Handle(err)
		}
		return nil
	})
	fmt.Printf("Added block %x.\n", block.Hash)
}

// BestHeight returns the height of the last block.
func (bc *BlockChain) BestHeight() uint64 {
	lastBlock := bc.getLastBlock()
	return lastBlock.Height
}

// GetBlock retrieves block from blockchain DB.
func (bc *BlockChain) GetBlock(blockHash Hash) (*Block, error) {
	var block *Block
	var item *badger.Item
	var err error
	var blockData []byte
	err = bc.Database.View(func(txn *badger.Txn) error {
		if item, err = txn.Get(blockHash); err != nil {
			return fmt.Errorf("block %s not found", blockHash)
		}
		err = item.Value(func(val []byte) error {
			blockData = append([]byte{}, val...)
			return nil
		})
		block = DeserializeBlock(blockData)
		return nil
	})
	return block, err
}

// GetBlockHashes returns all block hashes.
func (bc *BlockChain) GetBlockHashes() []Hash {
	var hashes []Hash
	iter := bc.CreateBCIterator()
	for iter.HasNext() {
		block := iter.GetNext()
		hashes = append(hashes, block.Hash)
	}
	return hashes
}

// MineBlock creates a new block in the database.
// The actual mining (proof of concept) happens in `CreateBlock(...)`.
func (bc *BlockChain) MineBlock(transactions []*Transaction) *Block {
	// Check validity of transactions.
	for _, tx := range transactions {
		if !bc.VerifyTransaction(tx) {
			log.Panic("Invalid Transaction")
		}
	}
	// Retrieve last height from blockchain.
	lastHash := bc.getLastHash()
	lastBlock := bc.getLastBlock()
	lastHeight := lastBlock.Height
	// Create new block in blockchain. CreateBlock() executes proof of work.
	newBlock := createBlock(transactions, lastHash, lastHeight+1)
	err := bc.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		bcerror.Handle(err)
		err = txn.Set(lastHashEntry, newBlock.Hash)
		return err
	})
	bcerror.Handle(err)
	return newBlock
}

// Returns UTXOs as a map: transaction ID -> transaction outputs.
func (bc *BlockChain) findUTXO() map[string]TxOutputs {
	log.Println("Entering FindUTXO")
	defer log.Println("Returning from FindUTXO")
	UTXO := make(map[string]TxOutputs)  // map: transaction ID -> transaction outputs
	spentTXOs := make(map[string][]int) // map: transaction ID -> indexes of spent output transactions

	// iterate through entire blockchain
	iter := bc.CreateBCIterator()
	for iter.HasNext() {
		block := iter.GetNext()
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)
		Outputs:
			for outIdx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}
				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}
			// save spent TXOs
			if tx.isNotCoinbase() {
				for _, in := range tx.Inputs {
					inTxID := hex.EncodeToString(in.ID)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Out) // in.Out has alredy been spent
				}
			}
		}
	}
	return UTXO
}

// Find transaction by ID.
func (bc *BlockChain) findTransaction(ID []byte) (Transaction, error) {
	iter := bc.CreateBCIterator()
	for iter.HasNext() {
		block := iter.GetNext()
		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}
	}
	return Transaction{}, errors.New("Transaction does not exist")
}

// / Signs transaction with private key.
func (bc *BlockChain) signTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	// map: Transaction ID -> transaction
	prevTXs := make(map[string]Transaction)
	for _, in := range tx.Inputs {
		prevTX, err := bc.findTransaction(in.ID)
		bcerror.Handle(err)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	tx.Sign(privKey, prevTXs)
}

// VerifyTransaction verifies transaction.
func (bc *BlockChain) VerifyTransaction(tx *Transaction) bool {
	if tx.isCoinbase() {
		return true
	}
	prevTXs := make(map[string]Transaction)
	for _, in := range tx.Inputs {
		prevTX, err := bc.findTransaction(in.ID)
		bcerror.Handle(err)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	return tx.verify(prevTXs)
}

// OpenBlockChain opens existing blockchain.
// Every node has its own blockchain.
// In the blockchain DB we map block's hashes to its blocks with all transactions.
// Transactions are therefore not stored separately.
// There are also entries for UTXOs which are prefixed by "utxo-".
func OpenBlockChain(nodeID string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeID)
	if dbNotExists(path) {
		fmt.Println("No blockchain found. Please create one first!")
		runtime.Goexit()
	}
	opts := badger.DefaultOptions(path)
	db, err := openDB(&opts)
	bcerror.Handle(err)
	return &BlockChain{db}
}

// CreateBlockChain creates a new blockchain for a specific node.
// 'address' will get the mining reward.
func CreateBlockChain(address, nodeID string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeID)
	if dbExists(path) {
		fmt.Println("Blockchain already exists!")
		runtime.Goexit()
	}
	opts := badger.DefaultOptions(path)
	db, err := openDB(&opts)
	bcerror.Handle(err)
	err = db.Update(func(txn *badger.Txn) error {
		cbtx := CoinbaseTx(address, genesisData)
		genesis := genesis(cbtx)
		log.Printf("Genesis block: %+v", genesis)
		err = txn.Set(genesis.Hash, genesis.Serialize())
		bcerror.Handle(err)
		err = txn.Set(lastHashEntry, genesis.Hash)
		return err
	})
	bcerror.Handle(err)
	blockchain := &BlockChain{db}
	fmt.Println("Genesis block created!")
	return blockchain
}

// Returns true if BadgerDB already exists in 'path'.
func dbExists(path string) (exists bool) {
	// BadgerDB creates a "MANIFEST" file automatically.
	if _, err := os.Stat(path + "/MANIFEST"); os.IsExist(err) {
		exists = true
	}
	return
}

// Returns true if BadgerDB does NOT exist in 'path'.
func dbNotExists(path string) bool {
	return !dbExists(path)
}

// Opens BadgerDB.
func openDB(opts *badger.Options) (*badger.DB, error) {
	var db *badger.DB
	var err error
	if db, err = badger.Open(*opts); err != nil {
		if strings.Contains(err.Error(), "LOCK") {
			if db, err := retry(*opts); err == nil {
				log.Println("database unlocked, value log truncated")
				return db, nil
			}
			log.Println("could not unlock database:", err)
		}
		return nil, err
	}
	return db, nil
}

// Retries opening the BadgerDB.
func retry(opts badger.Options) (*badger.DB, error) {
	lockPath := filepath.Join(opts.Dir, "LOCK")
	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf(`removing "LOCK": %s`, err)
	}
	opts.Truncate = true
	db, err := badger.Open(opts)
	return db, err
}
