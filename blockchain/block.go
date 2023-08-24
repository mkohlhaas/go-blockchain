package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"log"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/mkohlhaas/golang-blockchain/bcerror"
)

// Proof of work difficulty.
const (
	Difficulty = 12
)

// Target value for proof of work.
var (
	target *big.Int
)

// Block of the blockchain.
type Block struct {
	Timestamp    int64
	Hash         []byte
	Transactions []*Transaction
	PrevHash     []byte
	Nonce        uint32
	Height       uint64
}

func init() {
	// Updates the requirement target for proof of work.
	target = big.NewInt(1)
	target.Lsh(target, uint(256-Difficulty))
}

// Hashes block using Merkle tree.
// Returns hash of Merkle tree.
func (b *Block) hashTransactions() []byte {
	var transactions [][]byte
	for _, tx := range b.Transactions {
		transactions = append(transactions, tx.Serialize())
	}
	return CalcMerkleHash(transactions)
}

// Creates a valid new block WITH proof of work.
func createBlock(txs []*Transaction, prevHash []byte, height uint64) *Block {
	b := &Block{
		Timestamp:    time.Now().Unix(),
		Hash:         []byte{},
		Transactions: txs,
		PrevHash:     prevHash,
		Nonce:        0,
		Height:       height,
	}
	b.RunProof()
	log.Printf("New Block: %+v\n", b)
	return b
}

// Creates the legenedary Genesis Block with only a coinbase transaction.
func genesis(coinbase *Transaction) *Block {
	prevHash := []byte{} // no previous hash
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

// SerializeBlock block for storing in database.
func (b *Block) SerializeBlock() []byte {
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
	for nonce < math.MaxUint32 { // we expect to find a nonce
		b.Nonce = nonce
		if b.IsValidBlockHeader() {
			break // we found a nonce
		}
		nonce++
	}
	b.calculateBlockHash()
}

// IsValidBlockHeader returns true if we have a valid block header.
// Validates proof of work.
func (b *Block) IsValidBlockHeader() bool {
	var intHash big.Int
	intHash.SetBytes(b.Hash)
	return intHash.Cmp(target) == -1
}

// Sets block hash to double sha256 of block header.
func (b *Block) calculateBlockHash() {
	data := [][]byte{
		[]byte(strconv.FormatInt(b.Timestamp, 10)),
		b.PrevHash,
		b.hashTransactions(),
		toHex(int64(b.Nonce)),
		toHex(int64(Difficulty)),
	}
	sep := []byte{}
	bh := bytes.Join(data, sep)
	b.Hash = doubleHash256(bh)
}

// Convert int64 to hexadecimal.
func toHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

// Returns double sha256 hash.
func doubleHash256(data []byte) []byte {
	hash := sha256.Sum256(data)
	hash1 := sha256.Sum256(hash[:])
	return hash1[:]
}

// Stringer for blocks.
func (b *Block) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Timestamp: %d", b.Timestamp))
	lines = append(lines, fmt.Sprintf("Hash: %s", b.Hash))
	lines = append(lines, fmt.Sprintf("PrevHash: %s", b.PrevHash))
	lines = append(lines, fmt.Sprintf("Nonce: %d", b.Nonce))
	lines = append(lines, fmt.Sprintf("Height: %d", b.Height))
	for i, tx := range b.Transactions {
		lines = append(lines, fmt.Sprintf("%d: %s", i, tx))
	}
	return strings.Join(lines, "\n")
}
