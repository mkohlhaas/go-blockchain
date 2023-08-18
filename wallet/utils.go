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

// https://raw.githubusercontent.com/kallerosenbaum/grokkingbitcoin/master/images/ch03/03-13.svg
// Turns address into public key hash.
func PKHFrom(address []byte) []byte {
	pubKeyHash := Base58Decode(address)
	return pubKeyHash[1 : len(pubKeyHash)-4] // remove version and checksum
}
