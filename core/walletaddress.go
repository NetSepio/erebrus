package core

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58/base58"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/sha3"
)

var WalletAddress string

// GenerateWalletAddress generates a wallet address based on the environment variable set
func GenerateWalletAddress() {
	// Check if MNEMONIC_ETH is set
	if mnemonicEth := os.Getenv("MNEMONIC_ETH"); mnemonicEth != "" {
		GenerateEthereumWalletAddress(mnemonicEth)
		return
	}

	// Check if MNEMONIC_SOL is set
	if mnemonicSol := os.Getenv("MNEMONIC_SOL"); mnemonicSol != "" {
		GenerateWalletAddressSolana(mnemonicSol)
		return
	}

	// Check if MNEMONIC_APTOS is set
	if mnemonicAptos := os.Getenv("MNEMONIC_APTOS"); mnemonicAptos != "" {
		GenerateWalletAddressAptos(mnemonicAptos)
		return
	}

	// Check if MNEMONIC is set
	if mnemonic := os.Getenv("MNEMONIC"); mnemonic != "" {
		GenerateWalletAddressSui(mnemonic)
		return
	}

	log.Fatal("No mnemonic environment variable is set")
}

// GenerateEthereumWalletAddress generates an Ethereum wallet address from the given mnemonic
func GenerateEthereumWalletAddress(mnemonic string) {
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

	// Derive a child key (using the Ethereum derivation path m/44'/60'/0'/0/0)
	childKey, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		log.Fatal(err)
	}
	childKey, err = childKey.NewChildKey(bip32.FirstHardenedChild + 60)
	if err != nil {
		log.Fatal(err)
	}
	childKey, err = childKey.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		log.Fatal(err)
	}
	childKey, err = childKey.NewChildKey(0)
	if err != nil {
		log.Fatal(err)
	}
	childKey, err = childKey.NewChildKey(0)
	if err != nil {
		log.Fatal(err)
	}

	// Generate ECDSA private key from the child key
	privateKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		log.Fatal(err)
	}

	// Get the public key in uncompressed format
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	publicKeyBytes := crypto.FromECDSAPub(publicKey)

	log.Println("Private Key:", hex.EncodeToString(crypto.FromECDSA(privateKey)))
	log.Println("Public Key:", hex.EncodeToString(publicKeyBytes))

	// Generate the Ethereum address
	keccak := sha3.NewLegacyKeccak256()
	keccak.Write(publicKeyBytes[1:])      // Skip the first byte (0x04) of the uncompressed public key
	walletAddress := keccak.Sum(nil)[12:] // Take the last 20 bytes

	WalletAddress = hex.EncodeToString(walletAddress)
	log.Println("Ethereum Wallet Address:", WalletAddress)
}

// GenerateWalletAddressSolana generates a Solana wallet address from the given mnemonic
func GenerateWalletAddressSolana(mnemonic string) {
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
	WalletAddress = base58.Encode(publicKey)

	fmt.Printf("Wallet Address: %s\n", WalletAddress)
}

// GenerateWalletAddressSui generates a Sui wallet address from the given mnemonic
func GenerateWalletAddressSui(mnemonic string) {
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
	log.Println("The final wallet address:", WalletAddress)
}

// GenerateWalletAddressAptos generates an Aptos wallet address from the given mnemonic
func GenerateWalletAddressAptos(mnemonic string) {
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

	// Derive a child key (using the Aptos derivation path m/44'/637'/0'/0/0)
	childKey, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		log.Fatal(err)
	}
	childKey, err = childKey.NewChildKey(bip32.FirstHardenedChild + 637)
	if err != nil {
		log.Fatal(err)
	}
	childKey, err = childKey.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		log.Fatal(err)
	}
	childKey, err = childKey.NewChildKey(0)
	if err != nil {
		log.Fatal(err)
	}
	childKey, err = childKey.NewChildKey(0)
	if err != nil {
		log.Fatal(err)
	}

	// Generate ED25519 keys from the child key
	privateKey := ed25519.NewKeyFromSeed(childKey.Key)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	log.Println("Private Key:", hex.EncodeToString(privateKey))
	log.Println("Public Key:", hex.EncodeToString(publicKey))

	// Generate wallet address
	address := sha3.Sum256(publicKey[:])
	walletAddress := hex.EncodeToString(address[:])

	// Assign the wallet address to the global variable
	WalletAddress = "0x" + walletAddress
	log.Println("Aptos Wallet Address:", WalletAddress)
}
