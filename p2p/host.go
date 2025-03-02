package p2p

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/NetSepio/erebrus/types"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	bip39 "github.com/tyler-smith/go-bip39"
	bip32 "github.com/tyler-smith/go-bip32"
	log "github.com/sirupsen/logrus"
)

// Custom reader for deterministic key generation
type reader struct {
	seed []byte
	pos  int
}

func (r *reader) Read(p []byte) (n int, err error) {
	copy(p, r.seed)
	return len(r.seed), nil
}

func bytesReader(seed []byte) *reader {
	return &reader{seed: seed}
}

// Add this variable to store the host instance
var Host host.Host

// makeBasicHost creates a LibP2P host with a deterministic peer ID using mnemonics
func makeBasicHost() (host.Host, error) {
	// Get mnemonic from environment variable or use default
	mnemonic := os.Getenv("MNEMONIC")
	if mnemonic == "" {
		log.Warn("MNEMONIC not set, using default mnemonic")
		mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	}

	// Convert mnemonic to a BIP-32 seed
	seed := bip39.NewSeed(mnemonic, "")

	// Derive a master key from the seed
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %v", err)
	}

	// Derive a child key (hardened path example: m/44'/60'/0'/0)
	childKey, err := masterKey.NewChildKey(bip32.FirstHardenedChild)
	if err != nil {
		return nil, fmt.Errorf("failed to derive child key: %v", err)
	}

	// Convert the private key to an Ed25519 key (libp2p format)
	hashedKey := sha256.Sum256(childKey.Key) // Hashing to get a fixed-length key
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 256, bytesReader(hashedKey[:]))
	if err != nil {
		return nil, fmt.Errorf("failed to generate libp2p key: %v", err)
	}

	// Log the peer ID being generated (for debugging)
	peerID, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		log.Warnf("Failed to generate peer ID from private key: %v", err)
	} else {
		log.WithFields(log.Fields{
			"peerID": peerID.String(),
		}).Info("Generated deterministic peer ID")
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/9002"),
		libp2p.Identity(priv),
		libp2p.DisableRelay(),
	}

	host, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %v", err)
	}

	// Set the host in types package
	types.SetHost(host)

	// Log the host addresses
	log.WithFields(log.Fields{
		"addresses": host.Addrs(),
	}).Info("LibP2P host created with addresses")

	return host, nil
}

func getHostAddress(ha host.Host) string {
	// Build host multiaddress
	hostAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", ha.ID().String()))

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addr := ha.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr).String()

	log.WithFields(log.Fields{
		"address": fullAddr,
	}).Info("Generated host address")

	return fullAddr
}

// Add a function to get the Host
func GetHost() host.Host {
	return Host
}

// InitHost initializes the LibP2P host
func InitHost() error {
	_, err := makeBasicHost()
	if err != nil {
		return fmt.Errorf("failed to initialize LibP2P host: %v", err)
	}
	
	if host := types.GetHost(); host != nil {
		log.WithFields(log.Fields{
			"peerID": host.ID().String(),
			"addresses": host.Addrs(),
		}).Info("LibP2P host initialized successfully")
	}
	
	return nil
}
