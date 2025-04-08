package core

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/blocto/solana-go-sdk/pkg/hdwallet"
	"github.com/blocto/solana-go-sdk/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/sha3"
)

// These variables will be set at build time
var (
	Version  string
	CodeHash string
)

var (
	NodeName   string
	ChainName  string
	NodeType   string
	NodeConfig string
)
var WalletAddress string

// Function to load the node details from the environment and save it to the global variable
func LoadNodeDetails() {
	// Get the CHAIN_NAME variable from the environment
	NodeName = os.Getenv("NODE_NAME")

	ChainName = os.Getenv("CHAIN_NAME")
	if ChainName == "" {
		log.Fatalf("CHAIN_NAME environment variable is not set")
	} else {
		ChainName = strings.ToLower(ChainName)
		if ChainName == "solana" || ChainName == "eclipse" {
			GenerateWalletAddressSolanaAndEclipse(os.Getenv("MNEMONIC"))
		} else if ChainName == "ethereum" {
			GenerateEthereumWalletAddress(os.Getenv("MNEMONIC"))
		} else if ChainName == "sui" {
			GenerateWalletAddressSui(os.Getenv("MNEMONIC"))
		} else if ChainName == "aptos" {
			GenerateWalletAddressAptos(os.Getenv("MNEMONIC"))
		}
	}
	fmt.Printf("Chain Name: %s\n", ChainName)

	NodeType = os.Getenv("NODE_TYPE")
	if NodeType == "" {
		log.Fatalf("NODE_TYPE environment variable is not set")
	}
	fmt.Printf("Node Type: %s\n", NodeType)

	NodeConfig = os.Getenv("NODE_CONFIG")
	if NodeConfig == "" {
		log.Fatalf("NODE_CONFIG environment variable is not set")
	}
	fmt.Printf("Node Config: %s\n", NodeConfig)
}

// GenerateEthereumWalletAddress generates an Ethereum wallet address from the given mnemonic
func GenerateEthereumWalletAddress(mnemonic string) (string, *ecdsa.PrivateKey, error) {
	// Validate the mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		log.Fatal("Invalid mnemonic")
	}

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

	// log.Println("Private Key:", hex.EncodeToString(crypto.FromECDSA(privateKey)))
	// log.Println("Public Key:", hex.EncodeToString(publicKeyBytes))

	// Generate the Ethereum address
	keccak := sha3.NewLegacyKeccak256()
	keccak.Write(publicKeyBytes[1:])      // Skip the first byte (0x04) of the uncompressed public key
	walletAddress := keccak.Sum(nil)[12:] // Take the last 20 bytes

	// Convert to checksummed address
	WalletAddress = toChecksumAddress(hex.EncodeToString(walletAddress))
	// log.Println("Ethereum Wallet Address:", WalletAddress)
	return WalletAddress, privateKey, nil
}

// toChecksumAddress converts an address to checksummed format
func toChecksumAddress(address string) string {
	address = strings.ToLower(address)
	keccak := sha3.NewLegacyKeccak256()
	keccak.Write([]byte(address))
	hash := keccak.Sum(nil)

	var checksumAddress strings.Builder
	checksumAddress.WriteString("0x")

	for i, c := range address {
		if c >= '0' && c <= '9' {
			checksumAddress.WriteRune(c)
		} else {
			if hash[i/2]>>uint(4*(1-i%2))&0xF >= 8 {
				checksumAddress.WriteRune(c - 'a' + 'A')
			} else {
				checksumAddress.WriteRune(c)
			}
		}
	}

	return checksumAddress.String()
}

// GenerateWalletAddressSolana generates a Solana wallet address from the given mnemonic
func GenerateWalletAddressSolanaAndEclipse(mnemonic string) {
	// Validate the mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		fmt.Println("Invalid mnemonic")
		return
	}
	// mnemonic := "curtain century depth trim slogan stay case human farm ivory case merge"
	seed := bip39.NewSeed(mnemonic, "") // (mnemonic, password)
	path := `m/44'/501'/0'/0'`
	derivedKey, _ := hdwallet.Derived(path, seed)
	account, _ := types.AccountFromSeed(derivedKey.PrivateKey)

	WalletAddress = account.PublicKey.ToBase58()

	fmt.Printf("Solona OR Eclipse Wallet Address: %s\n", WalletAddress)
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

	// Derive a child key (using the Sui derivation path m/44'/784'/0'/0/0)
	childKey, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		log.Fatal(err)
	}
	childKey, err = childKey.NewChildKey(bip32.FirstHardenedChild + 784)
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

	// Generate wallet address (using SHA3-256)
	hash := sha3.New256()
	hash.Write(publicKey)
	walletAddress := hash.Sum(nil)

	WalletAddress = "0x" + hex.EncodeToString(walletAddress)
	log.Println("Sui Wallet Address:", WalletAddress)
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

	// Generate wallet address (using SHA3-256)
	hash := sha3.New256()
	hash.Write(publicKey)
	walletAddress := hash.Sum(nil)

	WalletAddress = "0x" + hex.EncodeToString(walletAddress)
	log.Println("Aptos Wallet Address:", WalletAddress)
}



func GetCodeHashAndVersion() (string, string) {
	CodeHash = "4f5610aae32077a92ac570eeff5f3a404052fd94"
	Version = "1.1.1"
	return CodeHash, Version
}
