package meumonicswalletaddr

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

func Meumonicswalletaddr() {
	// Generate a mnemonic
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		log.Fatal(err)
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Mnemonic:", mnemonic)

	// Derive a seed from the mnemonic
	seed := bip39.NewSeed(mnemonic, "")

	// Generate a master key using BIP32
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		log.Fatal(err)
	}

	// Derive a child key
	childKey, err := masterKey.NewChildKey(bip32.FirstHardenedChild)
	if err != nil {
		log.Fatal(err)
	}

	// Generate ED25519 keys from the child key
	privateKey := ed25519.NewKeyFromSeed(childKey.Key)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	fmt.Println("Private Key:", hex.EncodeToString(privateKey))
	fmt.Println("Public Key:", hex.EncodeToString(publicKey))

	// Generate wallet address
	hash := sha256.Sum256(publicKey)
	address := hex.EncodeToString(hash[:])
	fmt.Println("Wallet Address:", address)
}
