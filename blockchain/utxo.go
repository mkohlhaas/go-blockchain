package blockchain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/dgraph-io/badger"

	"github.com/mkohlhaas/gobc/bcerror"
)

var (
	// To separate entries in BadgerDB.
	// BadgerDB does not have namespaces.
	utxoPrefix = []byte("utxo-")
)

// UTXOSet just uses the underlying blockchain.
// UTXO = Unspent Transaction Outputs
type UTXOSet struct {
	Blockchain *BlockChain
}

// FindSpendableOutputs returns accumulated amount and a map: Transaction ID -> List of Indexes in Transaction.
func (u UTXOSet) FindSpendableOutputs(pubKeyHash Hash, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	accumulated := 0
	db := u.Blockchain.Database
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			k := item.Key()
			var v []byte
			err := item.Value(func(val []byte) error {
				v = append([]byte{}, val...)
				return nil
			})
			bcerror.Handle(err)
			k = bytes.TrimPrefix(k, utxoPrefix)
			txID := hex.EncodeToString(k)
			outs := deserializeOutputs(v)
			for outIdx, out := range outs.Outputs {
				if out.IsLockedWith(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				}
			}
		}
		return nil
	})
	bcerror.Handle(err)
	return accumulated, unspentOuts
}

// FindUnspentTransactions returnds all unused transactions.
func (u UTXOSet) FindUnspentTransactions(pubKeyHash Hash) []TxOutput {
	var UTXOs []TxOutput
	db := u.Blockchain.Database
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			var v []byte
			err := item.Value(func(val []byte) error {
				v = append([]byte{}, val...)
				return nil
			})
			bcerror.Handle(err)
			outs := deserializeOutputs(v)
			for _, out := range outs.Outputs {
				if out.IsLockedWith(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}
		return nil
	})
	bcerror.Handle(err)
	return UTXOs
}

// CountTransactions returns number of transaction in UTXO set.
func (u UTXOSet) CountTransactions() int {
	db := u.Blockchain.Database
	counter := 0
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			counter++
		}
		return nil
	})
	bcerror.Handle(err)
	return counter
}

// Reindex rebuilds the index of unspent transaction outputs.
func (u UTXOSet) Reindex() {
	db := u.Blockchain.Database
	u.deleteByPrefix(utxoPrefix)
	log.Printf("Before UTXO\n")
	UTXO := u.Blockchain.findUTXO()
	log.Printf("UTXO %v\n", UTXO)
	err := db.Update(func(txn *badger.Txn) error {
		for txID, outs := range UTXO {
			key, err := hex.DecodeString(txID)
			bcerror.Handle(err)
			key = append(utxoPrefix, key...)
			fmt.Printf("Reindex, key: %v\n", key)
			err = txn.Set(key, outs.Serialize())
			bcerror.Handle(err)
		}
		return nil
	})
	bcerror.Handle(err)
}

// Update ... (TODO)
func (u *UTXOSet) Update(block *Block) {
	db := u.Blockchain.Database
	err := db.Update(func(txn *badger.Txn) error {
		for _, tx := range block.Transactions {
			if tx.isCoinbase() == false {
				for _, in := range tx.Inputs {
					updatedOuts := TxOutputs{}
					inID := append(utxoPrefix, in.ID...)
					item, err := txn.Get(inID)
					bcerror.Handle(err)
					var v []byte
					err = item.Value(func(val []byte) error {
						v = append([]byte{}, val...)
						return nil
					})
					bcerror.Handle(err)
					outs := deserializeOutputs(v)
					for outIdx, out := range outs.Outputs {
						if outIdx != in.Out {
							updatedOuts.Outputs = append(updatedOuts.Outputs, out)
						}
					}
					if len(updatedOuts.Outputs) == 0 {
						if err := txn.Delete(inID); err != nil {
							log.Panic(err)
						}
					} else {
						if err := txn.Set(inID, updatedOuts.Serialize()); err != nil {
							log.Panic(err)
						}
					}
				}
			}
			newOutputs := TxOutputs{}
			for _, out := range tx.Outputs {
				newOutputs.Outputs = append(newOutputs.Outputs, out)
			}
			txID := append(utxoPrefix, tx.ID...)
			if err := txn.Set(txID, newOutputs.Serialize()); err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
	bcerror.Handle(err)
}

// Deletes all entries in the database with `prefix`.
func (u *UTXOSet) deleteByPrefix(prefix []byte) {
	// local function to delete keys.
	deleteKeys := func(keysForDelete [][]byte) error {
		fmt.Printf("DeleteByPrefix, key: %v\n", keysForDelete)
		err := u.Blockchain.Database.Update(func(txn *badger.Txn) error {
			for _, key := range keysForDelete {
				fmt.Printf("DeleteByPrefix, key: %v\n", key)
				if err := txn.Delete(key); err != nil {
					return err
				}
			}
			return nil
		})
		return err
	}

	collectSize := 100_000
	u.Blockchain.Database.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		keysForDelete := make([][]byte, 0, collectSize)
		keysCollected := 0
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			keysForDelete = append(keysForDelete, key)
			keysCollected++
			if keysCollected == collectSize {
				if err := deleteKeys(keysForDelete); err != nil {
					log.Panic(err)
				}
				keysForDelete = make([][]byte, 0, collectSize)
				keysCollected = 0
			}
		}
		if keysCollected > 0 {
			if err := deleteKeys(keysForDelete); err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
}
