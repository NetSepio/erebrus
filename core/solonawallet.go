package core

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/mr-tron/base58"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/ed25519"
)

var WalletAddressSolana string

// GenerateWalletAddress generates a wallet address from the mnemonic set in the environment
func GenerateWalletAddresssolana() {
	// Read mnemonic from environment variable
	mnemonic := os.Getenv("MNEMONIC_SOL")
	if mnemonic == "" {
		fmt.Println("MNEMONIC environment variable is not set")
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

	// Generate a keypair
	publicKey, _, err := ed25519.GenerateKey(bytes.NewReader(seed))
	if err != nil {
		fmt.Println(err)
		return
	}

	// Generate wallet address
	hash := sha256.Sum256(publicKey)
	WalletAddresssol := base58.Encode(hash[:20])
	fmt.Printf("Wallet Address: %s\n", WalletAddresssol)

	// Assign the wallet address to the variable (consider error handling)
	WalletAddressSolana = WalletAddresssol
	fmt.Printf("The final wallet address: %s\n", WalletAddressSolana)
}