package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/slice"
	"github.com/mkohlhaas/gobc/bcerror"
)

// Difficulty is the proof of work difficulty.
const Difficulty = 12

// Target value for proof of work.
var target *big.Int

// Block of the blockchain.
type Block struct {
	Timestamp    int64
	Hash         Hash
	Transactions []*Transaction
	PrevHash     Hash
	Nonce        uint32
	Height       uint64
}

// init updates the requirement target for proof of work.
func init() {
	target = big.NewInt(1)
	target.Lsh(target, uint(256-Difficulty))
}

// Returns hash of Merkle tree for block's transactions.
func (b *Block) hashTransactions() Hash {
	var transactions []serializedTransaction
	for _, tx := range b.Transactions {
		transactions = append(transactions, tx.Serialize())
	}
	return CalcMerkleHash(transactions)
}

// Creates a valid new block.
func createBlock(txs []*Transaction, prevHash Hash, height uint64) *Block {
	b := &Block{
		Timestamp:    time.Now().Unix(),
		Hash:         make(Hash, 0),
		Transactions: txs,
		PrevHash:     prevHash,
		Nonce:        0,
		Height:       height,
	}
	b.RunProof()
	log.Printf("New Block: %s\n", b)
	return b
}

// Creates the legendary Genesis Block with only a coinbase transaction.
func genesis(coinbase *Transaction) *Block {
	prevHash := make(Hash, 0) // no previous hash
	return createBlock([]*Transaction{coinbase}, prevHash, 0)
}

// Returns true if block is the legendary Genesis block.
func (b *Block) isGenesisBlock() bool {
	return b.Height == 0
}

// Returns true if block is NOT the legendary Genesis block.
func (b *Block) isNotGenesisBlock() bool {
	return !b.isGenesisBlock()
}

// Serialize block for storing in database.
func (b *Block) Serialize() []byte {
	var res bytes.Buffer
	encoder := gob.NewEncoder(&res)
	err := encoder.Encode(b)
	bcerror.Handle(err)
	return res.Bytes()
}

// DeserializeBlock block for retrieving from database.
func DeserializeBlock(data []byte) *Block {
	var b Block
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&b)
	bcerror.Handle(err)
	return &b
}

// RunProof runs proof of work.
// Updates nonce and hash in the block.
func (b *Block) RunProof() {
	var nonce uint32
	for nonce < math.MaxUint32 { // we expect to find a nonce (if not it takes too long anyways)
		if b.IsValidBlockHeader() {
			break // we found a nonce
		}
		nonce++
	}
	b.Nonce = nonce
	b.Hash = b.calcHash()
}

// IsValidBlockHeader returns true if we have a valid block header.
// Validates proof of work.
func (b *Block) IsValidBlockHeader() bool {
	hash := b.calcHash()
	var intHash big.Int
	intHash.SetBytes(hash)
	return intHash.Cmp(target) == -1 // block's hash < target
}

// Sets block hash to Double SHA256 of block header.
func (b *Block) calcHash() Hash {
	data := slice.Concat(
		[]byte(strconv.FormatInt(b.Timestamp, 10)),
		b.PrevHash,
		b.hashTransactions(),
		toHex(int64(b.Nonce)),
		toHex(int64(Difficulty))) // Difficulty is part of the hash!!!
	return doubleHash256(data)
}

// Stringer for blocks.
func (b *Block) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Timestamp: %d\n", b.Timestamp)
	fmt.Fprintf(&sb, "Hash: %s\n", b.Hash)
	fmt.Fprintf(&sb, "PrevHash: %s\n", b.PrevHash)
	fmt.Fprintf(&sb, "Nonce: %d\n", b.Nonce)
	fmt.Fprintf(&sb, "Height: %d\n", b.Height)
	fmt.Fprintf(&sb, "Transactions:\n")
	for i, tx := range b.Transactions {
		fmt.Fprintf(&sb, "  %d: %s\n", i, tx)
	}
	return sb.String()
}

// Returns Double SHA256 hash.
func doubleHash256(data []byte) Hash {
	hash := sha256.Sum256(data)
	hash1 := sha256.Sum256(hash[:])
	return hash1[:]
}

// Convert int64 to hexadecimal.
func toHex(num int64) []byte {
	return []byte(strconv.FormatInt(num, 16))
}
