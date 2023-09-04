package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"

	"github.com/mkohlhaas/gobc/bcerror"
	"golang.org/x/crypto/ripemd160"
)

// A wallet is just a pair of a private and a public key.
type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	PublicKey  []byte
}

func PKToAddress(pubKey []byte) []byte {
	pubHash := PublicKeyHash(pubKey)
	versionedHash := append([]byte{0}, pubHash...)
	checksum := Checksum(versionedHash)
	hash := append(versionedHash, checksum...)
	return Base58Encode(hash)
}

// https://raw.githubusercontent.com/kallerosenbaum/grokkingbitcoin/master/images/ch03/03-13.svg
// Returns Bitcoin address used by end-users.
// The Bitcoin network deals with PKHs (Public Key Hash).
func (w Wallet) Address() []byte {
	return PKToAddress(w.PublicKey)
}

// Creates a new private key and from it its public key.
// Returns (private key, public key).
func newKeyPair() (ecdsa.PrivateKey, []byte) {
	curve := elliptic.P256()
	sKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	bcerror.Handle(err)
	pKey := append(sKey.PublicKey.X.Bytes(), sKey.PublicKey.Y.Bytes()...)
	return *sKey, pKey
}

// Create a new wallet.
func MakeWallet() *Wallet {
	private, public := newKeyPair()
	return &Wallet{private, public}
}

// Calculates ripemd-160 hash.
func ripeMD(pubHash []byte) []byte {
	hasher := ripemd160.New()
	_, err := hasher.Write(pubHash)
	bcerror.Handle(err)
	return hasher.Sum(nil)
}

// https://raw.githubusercontent.com/kallerosenbaum/grokkingbitcoin/master/images/ch03/03-06.svg
// Returns public key hash of `pubKey`.
// = Hash160 = sha256 then ripemd160
func PublicKeyHash(pubKey []byte) []byte {
	pubHash := sha256.Sum256(pubKey)
	return ripeMD(pubHash[:])
}

// Calculates checksum for Hash256.
// Checksum is the first 4 bytes of double sha256.
func Checksum(payload []byte) []byte {
	fstHash := sha256.Sum256(payload)
	sndHash := sha256.Sum256(fstHash[:])
	return sndHash[:4]
}

// https://raw.githubusercontent.com/kallerosenbaum/grokkingbitcoin/master/images/ch03/03-15.svg
// Checks checksum of address.
func Validate(address string) bool {
	pubKeyHash := Base58Decode([]byte(address))
	actualChecksum := pubKeyHash[len(pubKeyHash)-4:]
	version := pubKeyHash[0]
	pubKeyHash = PKHFrom([]byte(address))
	calculatedChecksum := Checksum(append([]byte{version}, pubKeyHash...))
	return bytes.Compare(actualChecksum, calculatedChecksum) == 0
}
