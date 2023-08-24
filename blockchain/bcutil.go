// Package blockchain provides all necessary functions for dealing with the Bitcoin blockchain.
package blockchain

import (
	"fmt"
	"log"

	"github.com/dgraph-io/badger"
	"github.com/mkohlhaas/golang-blockchain/bcerror"
)

func (bc *BlockChain) showLastHashEntry() {
	err := bc.Database.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lhentry"))
		bcerror.Handle(err)
		err = item.Value(func(val []byte) error {
			fmt.Printf("Last hash entry: %v\n", val)
			return nil
		})
		return err
	})
	if err != nil {
		log.Printf("Error in showLastHashEntry: %s", err)
	}
}
