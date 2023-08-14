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
	lastHashEntry = []byte("lh")
)

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

func DBexists(path string) bool {
	if _, err := os.Stat(path + "/MANIFEST"); os.IsNotExist(err) {
		return false
	}
	return true
}
func ContinueBlockChain(nodeId string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeId)
	if DBexists(path) == false {
		fmt.Println("No existing blockchain found, create one!")
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
			// fmt.Printf("LastHash is: %v\n", val)
			lastHash = append([]byte{}, val...)
			return nil
		})
		return err
	})
	bcerror.Handle(err)
	return &BlockChain{lastHash, db}
}
func InitBlockChain(address, nodeId string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeId)
	if DBexists(path) {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}
	var lastHash []byte
	opts := badger.DefaultOptions(path)
	db, err := openDB(&opts)
	bcerror.Handle(err)
	err = db.Update(func(txn *badger.Txn) error {
		cbtx := CoinbaseTx(address, genesisData)
		genesis := genesis(cbtx)
		fmt.Println("Genesis created")
		err = txn.Set(genesis.Hash, genesis.Serialize())
		bcerror.Handle(err)
		err = txn.Set(lastHashEntry, genesis.Hash)
		lastHash = genesis.Hash
		return err
	})
	bcerror.Handle(err)
	blockchain := BlockChain{lastHash, db}
	return &blockchain
}
func (chain *BlockChain) AddBlock(block *Block) {
	err := chain.Database.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get(block.Hash); err == nil {
			return nil
		}
		blockData := block.Serialize()
		err := txn.Set(block.Hash, blockData)
		bcerror.Handle(err)
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
		lastBlock := Deserialize(lastBlockData)
		if block.Height > lastBlock.Height {
			err = txn.Set(lastHashEntry, block.Hash)
			bcerror.Handle(err)
			chain.LastHash = block.Hash
		}
		return nil
	})
	bcerror.Handle(err)
}
func (chain *BlockChain) GetBestHeight() int {
	var lastBlock Block
	err := chain.Database.View(func(txn *badger.Txn) error {
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
		lastBlock = *Deserialize(lastBlockData)
		return nil
	})
	bcerror.Handle(err)
	return lastBlock.Height
}
func (chain *BlockChain) GetBlock(blockHash []byte) (Block, error) {
	var block Block
	err := chain.Database.View(func(txn *badger.Txn) error {
		if item, err := txn.Get(blockHash); err != nil {
			return errors.New("Block not found")
		} else {
			var blockData []byte
			err = item.Value(func(val []byte) error {
				blockData = append([]byte{}, val...)
				return nil
			})
			block = *Deserialize(blockData)
		}
		return nil
	})
	if err != nil {
		return block, err
	}
	return block, nil
}
func (chain *BlockChain) GetBlockHashes() [][]byte {
	var blocks [][]byte
	iter := chain.CreateBCIterator()
	for iter.HasNext() {
		block := iter.GetNext()
		blocks = append(blocks, block.Hash)
	}
	return blocks
}
func (chain *BlockChain) MineBlock(transactions []*Transaction) *Block {
	var lastHash []byte
	var lastHeight int
	for _, tx := range transactions {
		if chain.VerifyTransaction(tx) != true {
			log.Panic("Invalid Transaction")
		}
	}
	err := chain.Database.View(func(txn *badger.Txn) error {
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
		lastBlock := Deserialize(lastBlockData)
		lastHeight = lastBlock.Height
		return err
	})
	bcerror.Handle(err)
	newBlock := CreateBlock(transactions, lastHash, lastHeight+1)
	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		bcerror.Handle(err)
		err = txn.Set(lastHashEntry, newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	})
	bcerror.Handle(err)
	return newBlock
}
func (chain *BlockChain) FindUTXO() map[string]TxOutputs {
	UTXO := make(map[string]TxOutputs)
	spentTXOs := make(map[string][]int)
	iter := chain.CreateBCIterator()
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
			if tx.IsCoinbase() == false {
				for _, in := range tx.Inputs {
					inTxID := hex.EncodeToString(in.ID)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Out)
				}
			}
		}
	}
	return UTXO
}
func (bc *BlockChain) FindTransaction(ID []byte) (Transaction, error) {
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
func (bc *BlockChain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]Transaction)
	for _, in := range tx.Inputs {
		prevTX, err := bc.FindTransaction(in.ID)
		bcerror.Handle(err)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	tx.Sign(privKey, prevTXs)
}
func (bc *BlockChain) VerifyTransaction(tx *Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}
	prevTXs := make(map[string]Transaction)
	for _, in := range tx.Inputs {
		prevTX, err := bc.FindTransaction(in.ID)
		bcerror.Handle(err)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}
	return tx.Verify(prevTXs)
}
func retry(opts badger.Options) (*badger.DB, error) {
	lockPath := filepath.Join(opts.Dir, "LOCK")
	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf(`removing "LOCK": %s`, err)
	}
	opts.Truncate = true
	db, err := badger.Open(opts)
	return db, err
}
func openDB(opts *badger.Options) (*badger.DB, error) {
	if db, err := badger.Open(*opts); err != nil {
		if strings.Contains(err.Error(), "LOCK") {
			if db, err := retry(*opts); err == nil {
				log.Println("database unlocked, value log truncated")
				return db, nil
			}
			log.Println("could not unlock database:", err)
		}
		return nil, err
	} else {
		return db, nil
	}
}
