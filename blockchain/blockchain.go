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
	"github.com/mkohlhaas/golang-blockchain/bcerror"
)

const (
	dbPath      = "./tmp/blocks_%s"
	genesisData = "The Times 03/Jan/2009: Chancellor on brink of second bailout for banks."
)

var (
	lastHashEntry = []byte("lhentry")
)

// BlockChain contains a reference to the actual blockchain DB and the last hash.
type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

// Returns true if BadgerDB already exists in 'path'.
func dbExists(path string) bool {
	// BadgerDB creates a "MANIFEST" file automatically.
	if _, err := os.Stat(path + "/MANIFEST"); os.IsNotExist(err) {
		return false
	}
	return true
}

// Returns true if BadgerDB does NOT exist in 'path'.
func dbDoesNotExist(path string) bool {
	return !dbExists(path)
}

// OpenBlockChain opens existing blockchain.
// Every node has its own blockchain.
func OpenBlockChain(nodeID string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeID)
	if dbDoesNotExist(path) {
		fmt.Println("No existing blockchain found. Please create one first!")
		runtime.Goexit()
	}
	opts := badger.DefaultOptions(path)
	db, err := openDB(&opts)
	bcerror.Handle(err)
	var lastHash []byte
	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(lastHashEntry)
		bcerror.Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = append([]byte{}, val...)
			return nil
		})
		return err
	})
	bcerror.Handle(err)
	return &BlockChain{lastHash, db}
}

// CreateBlockChain creates a new blockchain for a specific node.
// 'address' will get the mining reward.
func CreateBlockChain(address, nodeID string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeID)
	if dbExists(path) {
		fmt.Println("Blockchain already exists!")
		runtime.Goexit()
	}
	var lastHash []byte
	opts := badger.DefaultOptions(path)
	db, err := openDB(&opts)
	bcerror.Handle(err)
	err = db.Update(func(txn *badger.Txn) error {
		cbtx := CoinbaseTx(address, genesisData)
		genesis := genesis(cbtx)
		log.Printf("Genesis block: %+v", genesis)
		err = txn.Set(genesis.Hash, genesis.SerializeBlock())
		bcerror.Handle(err)
		err = txn.Set(lastHashEntry, genesis.Hash)
		lastHash = genesis.Hash
		return err
	})
	bcerror.Handle(err)
	blockchain := BlockChain{lastHash, db}
	fmt.Println("Genesis block created!")
	return &blockchain
}

// AddBlock adds a block to the blockchain.
func (bc *BlockChain) AddBlock(block *Block) {
	err := bc.Database.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get(block.Hash); err == nil {
			fmt.Printf("Block %x already in the blockchain.\n", block.Hash)
			return nil
		}
		blockData := block.SerializeBlock()
		// Store block in DB.
		err := txn.Set(block.Hash, blockData)
		bcerror.Handle(err)
		// Get last block.
		item, err := txn.Get(lastHashEntry)
		bcerror.Handle(err)
		var lastHash []byte
		err = item.Value(func(val []byte) error {
			lastHash = append([]byte{}, val...)
			return nil
		})
		item, err = txn.Get(lastHash)
		bcerror.Handle(err)
		var lastBlockData []byte
		err = item.Value(func(val []byte) error {
			lastBlockData = append([]byte{}, val...)
			return nil
		})
		lastBlock := DeserializeBlock(lastBlockData)
		// New block should have a height bigger than the last block. Not necessarily by one.
		if block.Height > lastBlock.Height {
			// Update last hash entry if we have a new top.
			err = txn.Set(lastHashEntry, block.Hash)
			bcerror.Handle(err)
			bc.LastHash = block.Hash
		}
		return nil
	})
	bcerror.Handle(err)
	fmt.Printf("Added block %x.\n", block.Hash)
}

// BestHeight returns the height of the last block.
func (bc *BlockChain) BestHeight() uint64 {
	var lastBlock Block
	err := bc.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(lastHashEntry)
		bcerror.Handle(err)
		var lastHash []byte
		err = item.Value(func(val []byte) error {
			lastHash = append([]byte{}, val...)
			return nil
		})
		item, err = txn.Get(lastHash)
		bcerror.Handle(err)
		var lastBlockData []byte
		err = item.Value(func(val []byte) error {
			lastBlockData = append([]byte{}, val...)
			return nil
		})
		bcerror.Handle(err)
		lastBlock = *DeserializeBlock(lastBlockData)
		return nil
	})
	bcerror.Handle(err)
	return lastBlock.Height
}

// GetBlock retrieves block from blockchain DB.
func (bc *BlockChain) GetBlock(blockHash []byte) *Block {
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
	if err != nil {
		return nil
	}
	return block
}

// GetBlockHashes returns all block hashes.
func (bc *BlockChain) GetBlockHashes() [][]byte {
	var hashes [][]byte
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
	var lastHash []byte
	var lastHeight uint64

	// Check validity of transactions.
	for _, tx := range transactions {
		if !bc.VerifyTransaction(tx) {
			log.Panic("Invalid Transaction")
		}
	}
	// Retrieve last height from blockchain.
	err := bc.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(lastHashEntry)
		bcerror.Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = append([]byte{}, val...)
			return nil
		})
		item, err = txn.Get(lastHash)
		bcerror.Handle(err)
		var lastBlockData []byte
		err = item.Value(func(val []byte) error {
			lastBlockData = append([]byte{}, val...)
			return nil
		})
		lastBlock := DeserializeBlock(lastBlockData)
		lastHeight = lastBlock.Height
		return err
	})
	bcerror.Handle(err)
	// Create new block in blockchain. CreateBlock() executes proof of work.
	newBlock := createBlock(transactions, lastHash, lastHeight+1)
	err = bc.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.SerializeBlock())
		bcerror.Handle(err)
		err = txn.Set(lastHashEntry, newBlock.Hash)
		bc.LastHash = newBlock.Hash
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

/// Signs transaction with private key.
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
