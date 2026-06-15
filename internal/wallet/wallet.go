// Package wallet derives node wallet keys from the BIP39 mnemonic and signs
// gateway registration challenges. Solana (chain "sol") is the default; EVM
// (chain "evm") is also supported to match erebrus-gateway/internal/gw/wallet.
package wallet

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/blocto/solana-go-sdk/pkg/hdwallet"
	"github.com/blocto/solana-go-sdk/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/sha3"
)

const (
	ChainEVM = "evm"
	ChainSOL = "sol"
)

// Identity is a mnemonic-derived wallet used for gateway registration.
type Identity struct {
	Chain   string
	Address string
	PubKey  string // base58 (sol) or hex pubkey (evm/apt/sui)
}

// AddressFromMnemonic returns the wallet address for the given chain.
func AddressFromMnemonic(mnemonic, chain string) (string, error) {
	id, err := Derive(mnemonic, chain)
	if err != nil {
		return "", err
	}
	return id.Address, nil
}

// PublicKeyFromMnemonic returns the signing public key for gateway registration.
func PublicKeyFromMnemonic(mnemonic, chain string) (string, error) {
	chain = strings.ToLower(strings.TrimSpace(chain))
	if chain == "" {
		chain = ChainSOL
	}
	switch chain {
	case ChainSOL:
		seed := bip39.NewSeed(mnemonic, "")
		derived, err := hdwallet.Derived(`m/44'/501'/0'/0'`, seed)
		if err != nil {
			return "", err
		}
		account, err := types.AccountFromSeed(derived.PrivateKey)
		if err != nil {
			return "", err
		}
		return account.PublicKey.ToBase58(), nil
	case ChainEVM:
		_, key, err := evmKeypair(mnemonic)
		if err != nil {
			return "", err
		}
		pub := key.Public().(*ecdsa.PublicKey)
		return hex.EncodeToString(ethcrypto.FromECDSAPub(pub)), nil
	default:
		return "", fmt.Errorf("unsupported chain %q", chain)
	}
}

// Derive returns the wallet identity for the given chain.
func Derive(mnemonic, chain string) (*Identity, error) {
	chain = strings.ToLower(strings.TrimSpace(chain))
	if chain == "" {
		chain = ChainSOL
	}
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}
	switch chain {
	case ChainSOL:
		return deriveSolana(mnemonic)
	case ChainEVM:
		return deriveEVM(mnemonic)
	default:
		return nil, fmt.Errorf("unsupported wallet chain %q (use sol or evm)", chain)
	}
}

func deriveSolana(mnemonic string) (*Identity, error) {
	seed := bip39.NewSeed(mnemonic, "")
	derived, err := hdwallet.Derived(`m/44'/501'/0'/0'`, seed)
	if err != nil {
		return nil, fmt.Errorf("derive solana key: %w", err)
	}
	account, err := types.AccountFromSeed(derived.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("solana account: %w", err)
	}
	return &Identity{
		Chain:   ChainSOL,
		Address: account.PublicKey.ToBase58(),
		PubKey:  account.PublicKey.ToBase58(),
	}, nil
}

// SignChallengeWithMnemonic signs a challenge using the mnemonic directly.
func SignChallengeWithMnemonic(mnemonic, chain, message string) (address, publicKey, signature string, err error) {
	chain = strings.ToLower(strings.TrimSpace(chain))
	if chain == "" {
		chain = ChainSOL
	}
	switch chain {
	case ChainSOL:
		seed := bip39.NewSeed(mnemonic, "")
		derived, err := hdwallet.Derived(`m/44'/501'/0'/0'`, seed)
		if err != nil {
			return "", "", "", err
		}
		account, err := types.AccountFromSeed(derived.PrivateKey)
		if err != nil {
			return "", "", "", err
		}
		sig := ed25519.Sign(account.PrivateKey, []byte(message))
		return account.PublicKey.ToBase58(), account.PublicKey.ToBase58(), base58.Encode(sig), nil
	case ChainEVM:
		addr, key, err := evmKeypair(mnemonic)
		if err != nil {
			return "", "", "", err
		}
		prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
		hash := ethcrypto.Keccak256Hash([]byte(prefixed))
		sig, err := ethcrypto.Sign(hash.Bytes(), key)
		if err != nil {
			return "", "", "", err
		}
		if sig[64] < 27 {
			sig[64] += 27
		}
		pub := key.Public().(*ecdsa.PublicKey)
		return addr, hex.EncodeToString(ethcrypto.FromECDSAPub(pub)), "0x" + hex.EncodeToString(sig), nil
	default:
		return "", "", "", fmt.Errorf("unsupported chain %q", chain)
	}
}

func deriveEVM(mnemonic string) (*Identity, error) {
	addr, _, err := evmKeypair(mnemonic)
	if err != nil {
		return nil, err
	}
	return &Identity{Chain: ChainEVM, Address: addr, PubKey: ""}, nil
}

func evmKeypair(mnemonic string) (address string, key *ecdsa.PrivateKey, err error) {
	seed := bip39.NewSeed(mnemonic, "")
	master, err := bip32.NewMasterKey(seed)
	if err != nil {
		return "", nil, err
	}
	child := master
	for _, idx := range []uint32{bip32.FirstHardenedChild + 44, bip32.FirstHardenedChild + 60, bip32.FirstHardenedChild + 0, 0, 0} {
		child, err = child.NewChildKey(idx)
		if err != nil {
			return "", nil, err
		}
	}
	key, err = ethcrypto.ToECDSA(child.Key)
	if err != nil {
		return "", nil, err
	}
	pub := key.Public().(*ecdsa.PublicKey)
	pubBytes := ethcrypto.FromECDSAPub(pub)
	keccak := sha3.NewLegacyKeccak256()
	keccak.Write(pubBytes[1:])
	addrBytes := keccak.Sum(nil)[12:]
	return toChecksumAddress(hex.EncodeToString(addrBytes)), key, nil
}

func toChecksumAddress(address string) string {
	address = strings.ToLower(address)
	keccak := sha3.NewLegacyKeccak256()
	keccak.Write([]byte(address))
	hash := keccak.Sum(nil)
	var b strings.Builder
	b.WriteString("0x")
	for i, c := range address {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		} else if hash[i/2]>>(4*(1-i%2))&0xF >= 8 {
			b.WriteRune(c - 'a' + 'A')
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}