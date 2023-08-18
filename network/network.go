package network

// TODO: replace slice functions, e.g. remove, exists with functions from slice extension package
// See e.g. NodeIsKnown(); search for range command

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/mkohlhaas/golang-blockchain/bcerror"
	"github.com/mkohlhaas/golang-blockchain/blockchain"
	"github.com/vrecan/death/v3"
)

const (
	protocol      = "tcp"
	commandLength = 12
)

var (
	nodeAddress string
	mineAddress string
	KnownNodes  = []string{"localhost:3000"} // TODO: replace slice with map (makes insertion and deletion easier)
	// KnownNodes      = map[string]bool{"localhost:3000": true}
	blocksInTransit = [][]byte{}                              // track downloaded block hashes
	memoryPool      = make(map[string]blockchain.Transaction) // map: transaction id -> transaction
)

// For sending/receiving known nodes.
type addr struct {
	addrList []string // list of peers
}

// For sending/receiving blocks.
type block struct {
	addrFrom string // sender
	block    []byte // block
}

// For sending/receiving requesting a list of all block hashes.
type getBlocks struct {
	addrFrom string // sender
}

// For sending/receiving blocks and transactions.
type getData struct {
	addrFrom string // sender
	kind     string // "block" or "tx"
	id       []byte // identifier
}

// For sending/receiving inventary. Can be either blocks or transactions.
// Show me
type inv struct {
	addrFrom string   // sender
	kind     string   // "block" or "tx"
	items    [][]byte // blocks or transactions
}

// For sending/receiving transactions.
type tx struct {
	addrFrom    string // sender
	transaction []byte // one transaction only in our implementation
}

// For sending/receiving version and current blockchain height.
type version struct {
	bestHeight uint64 // current blockchain length
	addrFrom   string // the sender
}

// ------------------------------------------------------------------- //
// ------------------- Sending Requests ------------------------------ //
// ------------------------------------------------------------------- //

// Converts string to a command.
func cmdToBytes(cmd string) []byte {
	var bytes [commandLength]byte
	for i, c := range cmd {
		bytes[i] = byte(c)
	}
	return bytes[:]
}

// Generic send used by every specific send function, e.g. SendAddr(), sendData(),...
// Sends `data` to `addr`.
// Removes `addr` from list of known nodes if it is unreachable..
func sendData(addr string, data []byte) {
	conn, err := net.Dial(protocol, addr)
	if err != nil {
		fmt.Printf("%s is not available\n", addr)
		// Remove `address` from KnownNodes.
		var updatedNodes []string
		for _, node := range KnownNodes {
			if node != addr {
				updatedNodes = append(updatedNodes, node)
			}
		}
		KnownNodes = updatedNodes
		return
	}
	defer conn.Close()
	// Puts data on the wire.
	_, err = io.Copy(conn, bytes.NewReader(data))
	bcerror.Handle(err)
}

func encode(data any) []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	bcerror.Handle(err)
	return buff.Bytes()
}

// Sends a getblock message to every known node.
func requestBlocks() {
	for _, node := range KnownNodes {
		sendGetBlocks(node)
	}
}

// Not used.
func sendAddr(address string) {
	nodes := addr{KnownNodes}
	nodes.addrList = append(nodes.addrList, nodeAddress)
	payload := encode(nodes)
	request := append(cmdToBytes("addr"), payload...)
	sendData(address, request)
}

// We have new blocks or transaction for our peer.
// Send our peer those hashes.
func sendInv(address, kind string, items [][]byte) {
	inventory := inv{nodeAddress, kind, items}
	payload := encode(inventory)
	request := append(cmdToBytes("inv"), payload...)
	sendData(address, request)
}

// Request all block hashes from peer.
func sendGetBlocks(address string) {
	payload := encode(getBlocks{nodeAddress})
	request := append(cmdToBytes("getblocks"), payload...)
	sendData(address, request)
}

// Send a request for a single block or a single transaction.
// `id` is hash of the block or transaction.
func sendGetData(address, kind string, id []byte) {
	payload := encode(getData{nodeAddress, kind, id})
	request := append(cmdToBytes("getdata"), payload...)
	sendData(address, request)
}

// Sends the whole block to our peer.
func sendBlock(addr string, b *blockchain.Block) {
	data := block{nodeAddress, b.SerializeBlock()}
	payload := encode(data)
	request := append(cmdToBytes("block"), payload...)
	sendData(addr, request)
}

// Sends transaction to peer.
func SendTx(addr string, tnx *blockchain.Transaction) {
	data := tx{nodeAddress, tnx.Serialize()}
	payload := encode(data)
	request := append(cmdToBytes("tx"), payload...)
	sendData(addr, request)
}

// Send the peer our current state of the blockchain.
func sendVersion(addr string, chain *blockchain.BlockChain) {
	bestHeight := chain.BestHeight()
	payload := encode(version{bestHeight, nodeAddress})
	request := append(cmdToBytes("version"), payload...)
	sendData(addr, request)
}

// ------------------------------------------------------------------- //
// ------------------ Receiving Requests ----------------------------- //
// ------------------------------------------------------------------- //

// Handles incoming request.
// Runs in separate goroutines for each TCP connection.
// TCP connection is stateless and will be closed after handling the request.
func HandleConnection(conn net.Conn, chain *blockchain.BlockChain) {
	req, err := ioutil.ReadAll(conn)
	defer conn.Close()
	bcerror.Handle(err)
	command := bytesToCmd(req[:commandLength])
	fmt.Printf("Received %s command\n", command)
	switch command {
	case "addr":
		HandleAddr(req) // not used in our implementation
	case "block":
		HandleBlock(req, chain)
	case "tx":
		HandleTx(req, chain)
	case "inv":
		HandleInv(req, chain)
	case "getblocks":
		HandleGetBlocks(req, chain)
	case "getdata":
		HandleGetData(req, chain)
	case "version":
		HandleVersion(req, chain)
	default:
		fmt.Println("Unknown command")
	}
}

// Converts byte slice to a string.
func bytesToCmd(bytes []byte) string {
	var cmd []byte
	for _, b := range bytes {
		if b != 0 {
			cmd = append(cmd, b)
		}
	}
	return fmt.Sprintf("%s", cmd)
}

// Returns decoded request.
// Used by all handler functions, e.g. HandleAddr, HandleVersion, etc...
func decode(request []byte, payload any) any {
	var buff bytes.Buffer
	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(payload)
	bcerror.Handle(err)
	return payload
}

// Adds addresses to list of known nodes.
func HandleAddr(request []byte) {
	var payload addr
	decode(request, &payload)
	fmt.Printf("HandleAddr: %+v.\n", payload)
	KnownNodes = append(KnownNodes, payload.addrList...)
	fmt.Printf("there are %d known nodes\n", len(KnownNodes))
	requestBlocks()
}

// Adds a received block to the blockchain.
func HandleBlock(request []byte, chain *blockchain.BlockChain) {
	var payload block
	decode(request, &payload)
	fmt.Printf("HandleBlock: %+v.\n", payload)
	block := blockchain.DeserializeBlock(payload.block)
	fmt.Println("Received a new block:")
	fmt.Printf("%s.\n", block)
	chain.AddBlock(block)
	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		sendGetData(payload.addrFrom, "block", blockHash)
		blocksInTransit = blocksInTransit[1:]
	} else {
		UTXOSet := blockchain.UTXOSet{Blockchain: chain}
		UTXOSet.Reindex()
	}
}

// Peer has a new block or transaction.
func HandleInv(request []byte, chain *blockchain.BlockChain) {
	var payload inv
	decode(request, &payload)
	fmt.Printf("HandleInv: %+v.\n", payload)
	if payload.kind == "block" {
		blocksInTransit = payload.items
		// Request first block of block list from sender. The rest will go into blocksInTransit.
		blockHash := payload.items[0]
		sendGetData(payload.addrFrom, "block", blockHash)
		// Removes first block from blocksInTransit.
		newInTransit := [][]byte{}
		for _, b := range blocksInTransit {
			if bytes.Compare(b, blockHash) != 0 {
				newInTransit = append(newInTransit, b)
			}
		}
		blocksInTransit = newInTransit
	}
	// Request transaction from sender.
	if payload.kind == "tx" {
		txID := payload.items[0] // In our implementation there is only one transaction in the list. See sendInv() call in HandleTx().
		if memoryPool[hex.EncodeToString(txID)].ID == nil {
			sendGetData(payload.addrFrom, "tx", txID)
		}
	}
}

// Sends all block hashes to sender.
func HandleGetBlocks(request []byte, chain *blockchain.BlockChain) {
	var payload getBlocks
	decode(request, &payload)
	fmt.Printf("HandleGetBlocks: %+v.\n", payload)
	blockHashes := chain.GetBlockHashes()
	sendInv(payload.addrFrom, "block", blockHashes)
}

// Send requested block or transaction to sender.
// No check if we have block or transaction available.
func HandleGetData(request []byte, chain *blockchain.BlockChain) {
	var payload getData
	decode(request, &payload)
	fmt.Printf("HandleGetData: %+v.\n", payload)
	if payload.kind == "block" {
		block := chain.GetBlock([]byte(payload.id))
		if block == nil {
			return
		}
		sendBlock(payload.addrFrom, block)
	}
	// Transaction must be in the memory pool. (Strange!)
	if payload.kind == "tx" {
		txID := hex.EncodeToString(payload.id)
		tx := memoryPool[txID]
		SendTx(payload.addrFrom, &tx)
	}
}

// Adds received transaction to the memory pool and starts mining
// if we are a miner and memoryPool is big enough (> 2 entries).
func HandleTx(request []byte, chain *blockchain.BlockChain) {
	var payload tx
	decode(request, &payload)
	fmt.Printf("HandleTx: %+v.\n", payload)
	txData := payload.transaction
	tx := blockchain.DeserializeTransaction(txData)
	memoryPool[hex.EncodeToString(tx.ID)] = tx
	// Central node sends transaction ID to all other nodes.
	if nodeAddress == KnownNodes[0] {
		for _, node := range KnownNodes {
			if node != nodeAddress && node != payload.addrFrom {
				sendInv(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		// Other nodes can mine transaction if memory pool is big enough.
		if len(memoryPool) >= 2 && len(mineAddress) > 0 {
			MineTx(chain)
		}
	}
}

// Upon receiving a version request request all block hashes from requester
// if we are behind or send our version if we are ahead.
func HandleVersion(request []byte, chain *blockchain.BlockChain) {
	var payload version
	decode(request, &payload)
	fmt.Printf("HandleVersion: %+v.\n", payload)
	bestHeight := chain.BestHeight()
	otherHeight := payload.bestHeight
	if bestHeight < otherHeight {
		// We are behind. Send our blocks to sender.
		sendGetBlocks(payload.addrFrom)
	} else if bestHeight > otherHeight {
		// Let sender know we are ahead of him.
		sendVersion(payload.addrFrom, chain)
	} // If both nodes are on the same level, we don't do anything.

	// If sender is unknown add it to known hosts.
	if senderIsNotKnown(payload.addrFrom) {
		KnownNodes = append(KnownNodes, payload.addrFrom)
	}
}

func senderIsKnown(addr string) bool {
	for _, node := range KnownNodes {
		if node == addr {
			return true
		}
	}
	return false
}

func senderIsNotKnown(addr string) bool {
	return !senderIsKnown(addr)
}

// ------------------------------------------------------------------- //
// ----------------------- Mining ------------------------------------ //
// ------------------------------------------------------------------- //

// Mines a new block and sends it to all nodes.
func MineTx(chain *blockchain.BlockChain) {
	// Remove all invalid transactions from memory pool.
	var txs []*blockchain.Transaction
	for id := range memoryPool {
		fmt.Printf("tx: %s\n", memoryPool[id].ID)
		tx := memoryPool[id]
		if chain.VerifyTransaction(&tx) {
			txs = append(txs, &tx)
		}
	}
	if len(txs) == 0 {
		fmt.Println("All Transactions are invalid")
		return
	}
	cbTx := blockchain.CoinbaseTx(mineAddress)
	txs = append(txs, cbTx)
	// TODO: Make coinbase the first transaction in the block.
	// txs = append([]*blockchain.Transaction{cbTx}, txs...)
	newBlock := chain.MineBlock(txs)
	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	UTXOSet.Reindex()
	fmt.Printf("New block mined: %s\n", newBlock)
	// Delete transactions from memoryPool.
	for _, tx := range txs {
		txID := hex.EncodeToString(tx.ID)
		delete(memoryPool, txID)
	}
	// Tell our peers we have a new block.
	for _, node := range KnownNodes {
		if node != nodeAddress {
			sendInv(node, "block", [][]byte{newBlock.Hash})
		}
	}
	// Mine again in case we are not finished.
  // Transactions come in via HandleTx() which calls MineTx(). Problematic.
	if len(memoryPool) > 0 {
		MineTx(chain)
	}
}

// ------------------------------------------------------------------- //
// ----------------------- Server ------------------------------------ //
// ------------------------------------------------------------------- //

// Starts server and waits for TCP connections.
// `minerAddress` will get the mining reward.
func StartServer(nodeID, minerAddress string) {
	// set global variables
	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	mineAddress = minerAddress
	// start TCP listen
	ln, err := net.Listen(protocol, nodeAddress)
	bcerror.Handle(err)
	defer ln.Close()
	// Open blockchain.
	chain := blockchain.OpenBlockChain(nodeID)
	defer chain.Database.Close()
	go closeDB(chain)
	// Non-central nodes send version package to central node.
	if nodeAddress != KnownNodes[0] {
		sendVersion(KnownNodes[0], chain)
	}
	// TCP accept loop. Start separate goroutine for handling request.
	for {
		conn, err := ln.Accept()
		bcerror.Handle(err)
		go HandleConnection(conn, chain)
	}
}

// Runs in a goroutine and waits for Ctrl-C to shutt down the server.
func closeDB(chain *blockchain.BlockChain) {
	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM)
	d.WaitForDeathWithFunc(func() {
		defer os.Exit(0)
		defer runtime.Goexit()
		chain.Database.Close()
	})
}
