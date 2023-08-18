package blockchain

import (
	"bytes"
	"encoding/gob"

	"github.com/mkohlhaas/golang-blockchain/bcerror"
	"github.com/mkohlhaas/golang-blockchain/wallet"
)

// Script stack (input on top of output):
// Signature Script (from input)
// Pubkey    Script (from output)

// transaction input
type TxInput struct {
	ID        []byte
	Out       int
	Signature []byte // = Signature Script in real Bitcoin
	PubKey    []byte
}

// transaction output
type TxOutput struct {
	Value      int
	PubKeyHash []byte // = Pubkey Script in real Bitcoin
}

// transaction outputs
type TxOutputs struct {
	Outputs []TxOutput
}

// Sets PubKeyHash in transaction output.
func (out *TxOutput) lock(address []byte) {
	pubKeyHash := wallet.PKHFrom(address)
	out.PubKeyHash = pubKeyHash
}

// Returns true if transaction output is locked with pubKeyHash.
func (out *TxOutput) IsLockedWith(pubKeyHash []byte) bool {
	return bytes.Compare(out.PubKeyHash, pubKeyHash) == 0
}

// Creates new transaction output.
// 'address' will be converted to a public key hash (PKH).
func NewTXOutput(value int, address string) *TxOutput {
	txo := &TxOutput{value, nil}
	txo.lock([]byte(address))
	return txo
}

// Serialize transaction outputs for storing in DB.
func (outs *TxOutputs) Serialize() []byte {
	var buffer bytes.Buffer
	encode := gob.NewEncoder(&buffer)
	err := encode.Encode(*outs)
	bcerror.Handle(err)
	return buffer.Bytes()
}

// Deserialize transaction outputs for retrieving from DB.
func DeserializeOutputs(data []byte) TxOutputs {
	var outputs TxOutputs
	decode := gob.NewDecoder(bytes.NewReader(data))
	err := decode.Decode(&outputs)
	bcerror.Handle(err)
	return outputs
}
