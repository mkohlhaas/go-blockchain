package wallet

import (
	"github.com/mkohlhaas/golang-blockchain/bcerror"
	"github.com/mr-tron/base58"
)

func Base58Encode(input []byte) []byte {
	encode := base58.Encode(input)
	return []byte(encode)
}
func Base58Decode(input []byte) []byte {
	decode, err := base58.Decode(string(input[:]))
  bcerror.Handle(err)
	return decode
}

// 0 O l I + /
