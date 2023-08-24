package blockchain

import (
	"crypto/sha256"
	"log"
)

// Merkle tree.
type merkleTree struct {
	rootNode *merkleNode
}

// Merkle node in a Merkle tree.
type merkleNode struct {
	left  *merkleNode
	right *merkleNode
	data  []byte
}

// Returns hash of Merkle tree.
func (t *merkleTree) hash() []byte {
	return t.rootNode.data
}

// Creates a new merkle node given its left and right branch.
func newMerkleNode(left, right *merkleNode, transaction []byte) *merkleNode {
	node := merkleNode{}
	if left == nil && right == nil {
		hash := sha256.Sum256(transaction)
		node.data = hash[:]
	} else {
		prevHashes := append(left.data, right.data...)
		hash := sha256.Sum256(prevHashes)
		node.data = hash[:]
	}
	node.left = left
	node.right = right
	return &node
}

// Creates a new Merkle tree.
func newMerkleTree(transactions [][]byte) *merkleTree {
	var nodes []merkleNode
	for _, transaction := range transactions {
		node := newMerkleNode(nil, nil, transaction)
		nodes = append(nodes, *node)
	}
	if len(nodes) == 0 {
		log.Panic("No merkel nodes")
	}
	for len(nodes) > 1 {
		if len(nodes)%2 != 0 {
			nodes = append(nodes, nodes[len(nodes)-1])
		}
		var level []merkleNode
		for i := 0; i < len(nodes); i += 2 {
			node := newMerkleNode(&nodes[i], &nodes[i+1], nil)
			level = append(level, *node)
		}
		nodes = level
	}
	// nodes exists only of one node - the root node
	tree := merkleTree{&nodes[0]}
	return &tree
}

// CalcMerkleHash returns Merkel hash value for all transactions.
func CalcMerkleHash(transactions [][]byte) []byte {
	mt := newMerkleTree(transactions)
	return mt.hash()
}
