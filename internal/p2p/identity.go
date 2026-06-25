// Package p2p provides the node's libp2p identity (a deterministic PeerID
// derived from the mnemonic), its DID, and DHT advertisement so the gateway
// and future IPFS/DHT use cases can discover it. It deliberately carries NO
// status/heartbeat logic — that moved to HTTPS + WebSocket (see
// internal/gatewayclient). The deterministic derivation matches v1 exactly so
// existing node mnemonics keep their PeerIDs.
package p2p

import (
	"crypto/sha256"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
)

// DIDPrefix is the Erebrus DID method prefix.
const DIDPrefix = "did:erebrus:"

// deterministicReader yields the same fixed seed bytes on every Read, used to
// make libp2p key generation deterministic from the mnemonic-derived seed.
type deterministicReader struct{ seed []byte }

func (r *deterministicReader) Read(p []byte) (int, error) {
	copy(p, r.seed)
	return len(r.seed), nil
}

// DeriveIdentity converts a BIP39 mnemonic into a libp2p Ed25519 private key.
// The derivation path (master → first hardened child → sha256) is identical to
// v1 so PeerIDs are stable across the v2 migration.
func DeriveIdentity(mnemonic string) (crypto.PrivKey, error) {
	if mnemonic == "" {
		return nil, fmt.Errorf("mnemonic is empty")
	}
	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("master key: %w", err)
	}
	childKey, err := masterKey.NewChildKey(bip32.FirstHardenedChild)
	if err != nil {
		return nil, fmt.Errorf("child key: %w", err)
	}
	hashed := sha256.Sum256(childKey.Key)
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 256, &deterministicReader{seed: hashed[:]})
	if err != nil {
		return nil, fmt.Errorf("libp2p key: %w", err)
	}
	return priv, nil
}

// GenerateMnemonic returns a fresh 12-word BIP39 mnemonic (128 bits entropy),
// used by the installer to provision a node identity when the operator does not
// supply one.
func GenerateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return "", fmt.Errorf("entropy: %w", err)
	}
	return bip39.NewMnemonic(entropy)
}

// PeerIDFromMnemonic returns the PeerID and DID derived from a mnemonic without
// starting a host. Useful for registration payloads and tests.
func PeerIDFromMnemonic(mnemonic string) (peerID string, did string, err error) {
	priv, err := DeriveIdentity(mnemonic)
	if err != nil {
		return "", "", err
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return "", "", err
	}
	return id.String(), DIDPrefix + id.String(), nil
}
