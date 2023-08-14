package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
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

type Transaction struct {
	ID      []byte
	Inputs  []TxInput
	Outputs []TxOutput
}

func (tx Transaction) Hash() []byte {
	var hash [32]byte
	tx.ID = []byte{}
	hash = sha256.Sum256(tx.Serialize())
	hash = sha256.Sum256(hash[:])
	return hash[:]
}
func (tx Transaction) Serialize() []byte {
	var encoded bytes.Buffer
	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	return encoded.Bytes()
}
func DeserializeTransaction(data []byte) Transaction {
	var transaction Transaction
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&transaction)
	bcerror.Handle(err)
	return transaction
}

// First transcaction in a block is always a coinbase transaction.
func CoinbaseTx(to string, data ...string) *Transaction {
	dat := data[0]
	if dat == "" {
		randData := make([]byte, 24)
		_, err := rand.Read(randData)
		bcerror.Handle(err)
		dat = fmt.Sprintf("%x", randData)
	}
	txin := TxInput{
		Out:    noIndex,
		PubKey: []byte(dat)}
	txout := NewTXOutput(20, to)
	tx := &Transaction{
		Inputs:  []TxInput{txin},
		Outputs: []TxOutput{*txout}}
	tx.ID = tx.Hash()
	return tx
}
func NewTransaction(w *wallet.Wallet, to string, amount int, UTXO *UTXOSet) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput
	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)
	acc, validOutputs := UTXO.FindSpendableOutputs(pubKeyHash, amount)
	if acc < amount {
		log.Panic("Error: not enough funds")
	}
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		bcerror.Handle(err)
		for _, out := range outs {
			input := TxInput{txID, out, nil, w.PublicKey}
			inputs = append(inputs, input)
		}
	}
	from := fmt.Sprintf("%s", w.Address())
	outputs = append(outputs, *NewTXOutput(amount, to))
	if acc > amount {
		outputs = append(outputs, *NewTXOutput(acc-amount, from))
	}
	tx := Transaction{nil, inputs, outputs}
	tx.ID = tx.Hash()
	UTXO.Blockchain.SignTransaction(&tx, w.PrivateKey)
	return &tx
}
func (tx *Transaction) IsCoinbase() bool {
	return tx.Inputs[0].Out == noIndex
}
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}
	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}
	txCleansed := tx.CleanseTransaction()
	for inId, in := range txCleansed.Inputs {
		prevTX := prevTXs[hex.EncodeToString(in.ID)]
		txCleansed.Inputs[inId].Signature = nil
		txCleansed.Inputs[inId].PubKey = prevTX.Outputs[in.Out].PubKeyHash
		dataToSign := fmt.Sprintf("%x\n", txCleansed)
		// Signing uses the entire cleansed transaction!
		r, s, err := ecdsa.Sign(rand.Reader, &privKey, []byte(dataToSign))
		bcerror.Handle(err)
		signature := append(r.Bytes(), s.Bytes()...)
		tx.Inputs[inId].Signature = signature
		txCleansed.Inputs[inId].PubKey = nil
	}
}
func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}
	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("Previous transaction not correct")
		}
	}
	txCleansed := tx.CleanseTransaction()
	curve := elliptic.P256()
	for inId, in := range tx.Inputs {
		prevTx := prevTXs[hex.EncodeToString(in.ID)]
		txCleansed.Inputs[inId].Signature = nil
		txCleansed.Inputs[inId].PubKey = prevTx.Outputs[in.Out].PubKeyHash
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
		if ecdsa.Verify(&rawPubKey, []byte(dataToVerify), &r, &s) == false {
			return false
		}
		txCleansed.Inputs[inId].PubKey = nil
	}
	return true
}
func (tx *Transaction) CleanseTransaction() Transaction {
	var inputs []TxInput
	var outputs []TxOutput
	for _, in := range tx.Inputs {
		inputs = append(inputs, TxInput{in.ID, in.Out, nil, nil})
	}
	for _, out := range tx.Outputs {
		// Would this work ?
		// outputs = append(outputs, out)
		outputs = append(outputs, TxOutput{out.Value, out.PubKeyHash})
	}
	return Transaction{tx.ID, inputs, outputs}
}
func (tx Transaction) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))
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
