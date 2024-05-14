package account

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

func GenerateMnemonic() (string, error) {
	// Generate 32 bytes of random entropy for 24 word phrase
	entropy := make([]byte, 32)
	_, err := rand.Read(entropy)
	if err != nil {
		fmt.Println("Error generating entropy:", err)
		return "", err
	}
	// Generate a mnemonic phrase
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		fmt.Println("Error generating mnemonic:", err)
		return "", err
	}
	return mnemonic, nil
}

func AddressFromMnemonic(mnemonic string) {
	// Generate seed from mnemonic
	seed := bip39.NewSeed(mnemonic, "")

	// Derive master key
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		fmt.Println("Error deriving master key:", err)
		return
	}

	// bip32 has hardenedKeyValue as indefined, use 0x80000000 instead
	accountExtendedKey, err := masterKey.NewChildKey(44 | 0x80000000)
	if err != nil {
		fmt.Println("Error deriving account extended key:", err)
		return
	}

	accountExtendedKey, err = accountExtendedKey.NewChildKey(0 | 0x80000000)
	if err != nil {
		fmt.Println("Error deriving account extended key:", err)
		return
	}

	accountExtendedKey, err = accountExtendedKey.NewChildKey(0 | 0x80000000)
	if err != nil {
		fmt.Println("Error deriving account extended key:", err)
		return
	}

	// Derive the first address from the account (index 0)
	address, err := getAddress(accountExtendedKey, 0)
	if err != nil {
		fmt.Println("Error deriving address:", err)
		return
	}
	fmt.Println("Derived Address:", address.Hex())
}

// getAddress derives the address from the given extended key and index
func getAddress(extendedKey *bip32.Key, index uint32) (common.Address, error) {
	derivedKey, err := extendedKey.NewChildKey(index)
	if err != nil {
		return common.Address{}, err
	}

	publicKey := derivedKey.PublicKey().Key
	xBytes := publicKey[1:33]
	yBytes := publicKey[33:]

	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)
	publicKeyECDSA := ecdsa.PublicKey{Curve: crypto.S256(), X: x, Y: y}

	address := crypto.PubkeyToAddress(publicKeyECDSA)
	return address, nil
}
