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

	"github.com/mkohlhaas/gobc/bcerror"
)

const (
	noIndex = -1
)

// Transaction contains transaction inputs and outputs.
type Transaction struct {
	ID      Hash
	Inputs  []TxInput
	Outputs []TxOutput
}

// Updates Transaction ID.
func (tx *Transaction) calcTransactionID() []byte {
	oldTX_ID := tx.ID
	tx.ID = make([]byte, 0) // reset transaction ID before calculating the Double SHA256 hash!
	dh := doubleHash256(tx.Serialize())
	tx.ID = oldTX_ID
	return dh
}

// Serialize transaction.
func (tx *Transaction) Serialize() []byte {
	var encoded bytes.Buffer
	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	bcerror.Handle(err)
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
	if len(data) == 0 {
		data[0] = randomString()
	}
	txin := TxInput{
		Out:    noIndex,
		PubKey: []byte(data[0])}
	txout := newTXOutput(20, to)
	tx := &Transaction{
		Inputs:  []TxInput{txin},
		Outputs: []TxOutput{*txout}}
	tx.ID = tx.calcTransactionID()
	return tx
}

// Returns a random string.
func randomString() string {
	randData := make([]byte, 24)
	_, err := rand.Read(randData)
	bcerror.Handle(err)
	return fmt.Sprintf("%x", randData)
}

// NewTransaction returns a transaction which uses all spendable outputs.
// Left over/change will be transferred to payer.
func NewTransaction(w *Wallet, to string, amount int, UTXO *UTXOSet) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput
	pubKeyHash := PublicKeyHash(w.PublicKey)
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
	tx.ID = tx.calcTransactionID()
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

func (tx *Transaction) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Transaction %x:\n", tx.ID)
	for i, input := range tx.Inputs {
		fmt.Fprintf(&b, "     Input %d:\n", i)
		fmt.Fprintf(&b, "       TXID:      %x\n", input.ID)
		fmt.Fprintf(&b, "       Out:       %d\n", input.Out)
		fmt.Fprintf(&b, "       Signature: %x\n", input.Signature)
		fmt.Fprintf(&b, "       PubKey:    %x\n", input.PubKey)
	}
	for i, output := range tx.Outputs {
		fmt.Fprintf(&b, "     Output %d:\n", i)
		fmt.Fprintf(&b, "       Value:  %d\n", output.Value)
		fmt.Fprintf(&b, "       Script: %x\n", output.PubKeyHash)
	}
	return b.String()
}
