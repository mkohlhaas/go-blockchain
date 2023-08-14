package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"

	"github.com/mkohlhaas/golang-blockchain/bcerror"
	"golang.org/x/crypto/ripemd160"
)

// https://raw.githubusercontent.com/kallerosenbaum/grokkingbitcoin/master/images/ch03/u03-14.svg
const (
	checksumLength = 4
	version        = byte(0x00)
)

type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	PublicKey  []byte
}

// https://raw.githubusercontent.com/kallerosenbaum/grokkingbitcoin/master/images/ch03/03-13.svg
// Returns Bitcoin address used by end-users.
// The Bitcoin network deals with PKHs (Public Key Hash).
func (w Wallet) Address() []byte {
	pubHash := PublicKeyHash(w.PublicKey)
	versionedHash := append([]byte{version}, pubHash...)
	checksum := Checksum(versionedHash)
	fullHash := append(versionedHash, checksum...)
	address := Base58Encode(fullHash)
	return address
}
func NewKeyPair() (ecdsa.PrivateKey, []byte) {
	curve := elliptic.P256()
	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	bcerror.Handle(err)
	pub := append(private.PublicKey.X.Bytes(), private.PublicKey.Y.Bytes()...)
	return *private, pub
}
func MakeWallet() *Wallet {
	private, public := NewKeyPair()
	wallet := Wallet{private, public}
	return &wallet
}

// https://raw.githubusercontent.com/kallerosenbaum/grokkingbitcoin/master/images/ch03/03-06.svg
func PublicKeyHash(pubKey []byte) []byte {
	pubHash := sha256.Sum256(pubKey)
	hasher := ripemd160.New()
	_, err := hasher.Write(pubHash[:])
	bcerror.Handle(err)
	publicRipMD := hasher.Sum(nil)
	return publicRipMD
}
func Checksum(payload []byte) []byte {
	firstHash := sha256.Sum256(payload)
	secondHash := sha256.Sum256(firstHash[:])
	return secondHash[:checksumLength]
}

// https://raw.githubusercontent.com/kallerosenbaum/grokkingbitcoin/master/images/ch03/03-15.svg
func ValidateAddress(address string) bool {
	pubKeyHash := Base58Decode([]byte(address))
	actualChecksum := pubKeyHash[len(pubKeyHash)-checksumLength:]
	version := pubKeyHash[0]
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-checksumLength]
	targetChecksum := Checksum(append([]byte{version}, pubKeyHash...))
	return bytes.Compare(actualChecksum, targetChecksum) == 0
}
