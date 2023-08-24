package blockchain

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO: Just use already tested cases from other Bitcoin projects.
// This test is really crappy.
func TestNewMerkleNode(t *testing.T) {
	data := [][]byte{
		[]byte("node1"),
		[]byte("node2"),
		[]byte("node3"),
		[]byte("node4"),
		[]byte("node5"),
		[]byte("node6"),
		[]byte("node7"),
	}
	// level 1
	mn1 := newMerkleNode(nil, nil, data[0])
	mn2 := newMerkleNode(nil, nil, data[1])
	mn3 := newMerkleNode(nil, nil, data[2])
	mn4 := newMerkleNode(nil, nil, data[3])
	mn5 := newMerkleNode(nil, nil, data[4])
	mn6 := newMerkleNode(nil, nil, data[5])
	mn7 := newMerkleNode(nil, nil, data[6])
	mn8 := newMerkleNode(nil, nil, data[6])
	// level 2
	mn9 := newMerkleNode(mn1, mn2, nil)
	mn10 := newMerkleNode(mn3, mn4, nil)
	mn11 := newMerkleNode(mn5, mn6, nil)
	mn12 := newMerkleNode(mn7, mn8, nil)
	//level 3
	mn13 := newMerkleNode(mn9, mn10, nil)
	mn14 := newMerkleNode(mn11, mn12, nil)
	//level 4
	mn15 := newMerkleNode(mn13, mn14, nil)
	root := fmt.Sprintf("%x", mn15.data)
	tree := newMerkleTree(data)
	assert.Equal(t, root, fmt.Sprintf("%x", tree.hash()), "Merkle node root has is equal")
}
