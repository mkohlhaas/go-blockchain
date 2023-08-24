package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/mkohlhaas/golang-blockchain/bcerror"
	"github.com/mkohlhaas/golang-blockchain/wallet"
)

const (
	noIndex = -1
)

// Transaction contains transaction inputs and outputs.
type Transaction struct {
	ID      []byte
	Inputs  []TxInput
	Outputs []TxOutput
}

// Updates Transaction ID.
func (tx *Transaction) updateTransactionID() {
	tx.ID = []byte{} // reset transaction ID
	tx.ID = doubleHash256(tx.Serialize())
}

// Serialize transaction.
func (tx *Transaction) Serialize() []byte {
	var encoded bytes.Buffer
	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	return encoded.Bytes()
}

// DeserializeTransaction deserializes transaction.
func DeserializeTransaction(data []byte) Transaction {
	var transaction Transaction
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&transaction)
	bcerror.Handle(err)
	return transaction
}

// CoinbaseTx is the first transcaction in a block.
// Mining rewards and fees go to `to`.
// `data` can be any string. Typically used for vanity blocks.
// `data` is defined as a variadic argument to make it optional.
func CoinbaseTx(to string, data ...string) *Transaction {
	if data[0] == "" {
		data[0] = randomString()
	}
	txin := TxInput{
		Out:    noIndex,
		PubKey: []byte(data[0])}
	txout := newTXOutput(20, to)
	tx := &Transaction{
		Inputs:  []TxInput{txin},
		Outputs: []TxOutput{*txout}}
	tx.updateTransactionID()
	return tx
}

// Returns a random string.
func randomString() string {
	randData := make([]byte, 24)
	_, err := rand.Read(randData)
	bcerror.Handle(err)
	return fmt.Sprintf("%x", randData)
}

// NewTransaction returns a new transaction.
// Uses all spendable outputs in blockchain.
// Left over change will be transferred to oneself.
func NewTransaction(w *wallet.Wallet, to string, amount int, UTXO *UTXOSet) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput
	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)
	acc, validOutputs := UTXO.FindSpendableOutputs(pubKeyHash, amount)
	fmt.Printf("Spendable output: %d\n", acc)
	if acc < amount {
		log.Panic("Error: not enough funds")
	}
	// Spendable outputs become inputs.
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		bcerror.Handle(err)
		for _, out := range outs {
			input := TxInput{
				ID:  txID,
				Out: out,
				// TODO: Signature is left empty.
				PubKey: w.PublicKey}
			inputs = append(inputs, input)
		}
	}
	from := fmt.Sprintf("%s", w.Address())
	outputs = append(outputs, *newTXOutput(amount, to))
	if acc > amount {
		// Create separate output to oneself (`from`) for change/odd money.
		outputs = append(outputs, *newTXOutput(acc-amount, from))
	}
	tx := &Transaction{
		Inputs:  inputs,
		Outputs: outputs}
	tx.updateTransactionID()
	UTXO.Blockchain.signTransaction(tx, w.PrivateKey)
	return tx
}

// Returns true if transaction is a coinbase transaction.
func (tx *Transaction) isCoinbase() bool {
	return tx.Inputs[0].Out == noIndex
}

// Returns true if transaction is NOT a coinbase transaction.
func (tx *Transaction) isNotCoinbase() bool {
	return !tx.isCoinbase()
}

// Sign transaction.
// `prevTXs` is a map: Transaction ID -> transaction.
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	if tx.isCoinbase() {
		return // nothing to do for the coinbase transaction
	}
	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}
	txCleansed := tx.cleanTransaction()
	for inID, in := range txCleansed.Inputs {
		prevTX := prevTXs[hex.EncodeToString(in.ID)]
		// has already been removed in cleanTransaction()
		// txCleansed.Inputs[inId].Signature = nil
		txCleansed.Inputs[inID].PubKey = prevTX.Outputs[in.Out].PubKeyHash
		dataToSign := fmt.Sprintf("%x\n", txCleansed)
		// Signing uses the entire cleaned transaction!
		r, s, err := ecdsa.Sign(rand.Reader, &privKey, []byte(dataToSign))
		bcerror.Handle(err)
		signature := append(r.Bytes(), s.Bytes()...)
		tx.Inputs[inID].Signature = signature
		txCleansed.Inputs[inID].PubKey = nil
	}
}

// Verifies transaction.
func (tx *Transaction) verify(prevTXs map[string]Transaction) bool {
	if tx.isCoinbase() {
		return true // nothing to verify for coinbase transaction
	}
	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("Previous transaction not correct")
		}
	}
	txCleansed := tx.cleanTransaction()
	curve := elliptic.P256()
	for inID, in := range tx.Inputs {
		prevTx := prevTXs[hex.EncodeToString(in.ID)]
		// has already been removed in cleanTransaction()
		// txCleansed.Inputs[inId].Signature = nil
		txCleansed.Inputs[inID].PubKey = prevTx.Outputs[in.Out].PubKeyHash
		// Verify ecdsa signature.
		r := big.Int{}
		s := big.Int{}
		sigLen := len(in.Signature)
		r.SetBytes(in.Signature[:(sigLen / 2)])
		s.SetBytes(in.Signature[(sigLen / 2):])
		x := big.Int{}
		y := big.Int{}
		keyLen := len(in.PubKey)
		x.SetBytes(in.PubKey[:(keyLen / 2)])
		y.SetBytes(in.PubKey[(keyLen / 2):])
		dataToVerify := fmt.Sprintf("%x\n", txCleansed)
		rawPubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}
		if !ecdsa.Verify(&rawPubKey, []byte(dataToVerify), &r, &s) {
			return false
		}
		txCleansed.Inputs[inID].PubKey = nil
	}
	return true
}

// Remove Signature and PubKey from transaction inputs.
func (tx *Transaction) cleanTransaction() Transaction {
	var inputs []TxInput
	var outputs []TxOutput
	for _, in := range tx.Inputs {
		inputs = append(inputs, TxInput{
			ID:  in.ID,
			Out: in.Out})
	}
	// Copy transaction outputs
	for _, out := range tx.Outputs {
		// outputs = append(outputs, TxOutput{out.Value, out.PubKeyHash})
		outputs = append(outputs, out) // TODO: Does this work ?
	}
	return Transaction{tx.ID, inputs, outputs}
}

// Stringer interface for transaction.
func (tx *Transaction) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Transaction %x:", tx.ID))
	for i, input := range tx.Inputs {
		lines = append(lines, fmt.Sprintf("     Input %d:", i))
		lines = append(lines, fmt.Sprintf("       TXID:     %x", input.ID))
		lines = append(lines, fmt.Sprintf("       Out:       %d", input.Out))
		lines = append(lines, fmt.Sprintf("       Signature: %x", input.Signature))
		lines = append(lines, fmt.Sprintf("       PubKey:    %x", input.PubKey))
	}
	for i, output := range tx.Outputs {
		lines = append(lines, fmt.Sprintf("     Output %d:", i))
		lines = append(lines, fmt.Sprintf("       Value:  %d", output.Value))
		lines = append(lines, fmt.Sprintf("       Script: %x", output.PubKeyHash))
	}
	return strings.Join(lines, "\n")
}
