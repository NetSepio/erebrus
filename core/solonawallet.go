package core

import (
	"fmt"
	"os"

	"github.com/mr-tron/base58"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/sha3"
)

var WalletAddressSolana string

// GenerateWalletAddressSolana generates a Solana wallet address from the mnemonic set in the environment
func GenerateWalletAddressSolana() {
	// Read mnemonic from environment variable
	mnemonic := os.Getenv("MNEMONIC_SOL")
	if mnemonic == "" {
		fmt.Println("MNEMONIC_SOL environment variable is not set")
		return
	}

	// Validate the mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		fmt.Println("Invalid mnemonic")
		return
	}
	fmt.Printf("Mnemonic: %s\n", mnemonic)

	// Derive a seed from the mnemonic
	seed := bip39.NewSeed(mnemonic, "")

	// Derive the keypair using PBKDF2 with the Solana-specific path
	derivedKey := pbkdf2.Key(seed, []byte("ed25519 seed"), 2048, 64, sha3.New512)

	// The first 32 bytes are the private key, the next 32 bytes are the chain code (unused here)
	privateKey := derivedKey[:32]

	// Generate the public key
	publicKey := ed25519.NewKeyFromSeed(privateKey).Public().(ed25519.PublicKey)

	// Encode the public key to Base58
	WalletAddressSolana = base58.Encode(publicKey)

	fmt.Printf("Wallet Address: %s\n", WalletAddressSolana)
}
