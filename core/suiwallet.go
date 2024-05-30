package core

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

var WalletAddress string

// GenerateWalletAddress generates a wallet address from the mnemonic set in the environment
func GenerateWalletAddress() {
	// Read mnemonic from environment variable
	mnemonic := os.Getenv("MNEMONIC")
	if mnemonic == "" {
		log.Fatal("MNEMONIC environment variable is not set")
	}

	// Validate the mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		log.Fatal("Invalid mnemonic")
	}
	log.Println("Mnemonic:", mnemonic)

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

	log.Println("Private Key:", hex.EncodeToString(privateKey))
	log.Println("Public Key:", hex.EncodeToString(publicKey))

	// Generate wallet address
	hash := sha256.Sum256(publicKey)
	walletAddress := hex.EncodeToString(hash[:])
	log.Println("Wallet Address:", walletAddress)

	// Assign the wallet address to the global variable
	WalletAddress = walletAddress
}
