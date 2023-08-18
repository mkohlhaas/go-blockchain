package wallet

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"fmt"
	"os"

	"github.com/mkohlhaas/golang-blockchain/bcerror"
)

const walletFile = "./tmp/wallets_%s.data"

type Wallets struct {
	// map: Bitcoin Address → Wallet
	Wallets map[string]*Wallet
}

// Opens wallets from existing wallets file.
// If wallets file does not exists returns an error and an empty wallets map - but no file..
func OpenWallets(nodeId string) (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)
	err := wallets.LoadFile(nodeId)
	return &wallets, err
}

// Creates a new wallet and adds it to wallets.
// Used from the command line with the `createwallet` command.
func (ws *Wallets) AddWallet() string {
	wallet := MakeWallet()
	address := fmt.Sprintf("%s", wallet.Address())
	ws.Wallets[address] = wallet
	return address
}

// Returns all Bitcoin addresses in wallets.
func (ws *Wallets) GetAllAddresses() []string {
	var addresses []string
	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}
	return addresses
}

// Returns a specific wallet from wallets.
func (ws Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}

// Saves wallets into a file.
func (ws *Wallets) SaveFile(nodeId string) {
	var content bytes.Buffer
	walletFile := fmt.Sprintf(walletFile, nodeId)
	// NOTE: Works only with Go version 1.18.x!
	gob.Register(elliptic.P256())
	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(ws)
	bcerror.Handle(err)
	err = os.WriteFile(walletFile, content.Bytes(), 0644)
	bcerror.Handle(err)
}

// Loads wallet file for `nodeId`.
// Returns an error If file does not exist.
func (ws *Wallets) LoadFile(nodeId string) error {
	walletFile := fmt.Sprintf(walletFile, nodeId)
	if _, err := os.Stat(walletFile); os.IsNotExist(err) {
		return err
	}
	var wallets Wallets
	fileContent, err := os.ReadFile(walletFile)
	bcerror.Handle(err)
	// NOTE: Works only with Go version 1.18.x!
	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&wallets)
	bcerror.Handle(err)
	ws.Wallets = wallets.Wallets
	return nil
}
