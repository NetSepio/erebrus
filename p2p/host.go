package p2p

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/NetSepio/erebrus/types"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	tls "github.com/libp2p/go-libp2p/p2p/security/tls"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	tcp "github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
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

// QUIC configuration constants
const (
	DefaultQUICPort       = 9002
	DefaultTCPPort        = 9003
	MaxIdleTimeout        = 30 * time.Second
	KeepAlive             = 15 * time.Second
	MaxIncomingStreams    = 1000
	MaxIncomingUniStreams = 1000
)

// getListenAddresses returns optimized listen addresses for both QUIC and TCP fallback
func getListenAddresses() []string {
	// Get ports from environment or use defaults
	quicPort := DefaultQUICPort
	tcpPort := DefaultTCPPort

	if envPort := os.Getenv("QUIC_PORT"); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil {
			quicPort = port
		}
	}

	if envPort := os.Getenv("TCP_PORT"); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil {
			tcpPort = port
		}
	}

	return []string{
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", quicPort), // QUIC v1 (preferred)
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", quicPort),    // QUIC legacy
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", tcpPort),          // TCP fallback
		fmt.Sprintf("/ip6/::/udp/%d/quic-v1", quicPort),      // IPv6 QUIC v1
		fmt.Sprintf("/ip6/::/udp/%d/quic", quicPort),         // IPv6 QUIC legacy
		fmt.Sprintf("/ip6/::/tcp/%d", tcpPort),               // IPv6 TCP fallback
	}
}

// makeBasicHost creates a LibP2P host with optimized QUIC transport and TCP fallback
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

	// Enhanced libp2p options with QUIC optimizations
	opts := []libp2p.Option{
		// Transport configuration - QUIC first, TCP fallback
		libp2p.Transport(quic.NewTransport),
		libp2p.Transport(tcp.NewTCPTransport),

		// Security protocols - Noise preferred, TLS fallback
		libp2p.Security(noise.ID, noise.New),
		libp2p.Security(tls.ID, tls.New),

		// Listen on multiple addresses for better connectivity
		libp2p.ListenAddrStrings(getListenAddresses()...),

		// Identity and basic configuration
		libp2p.Identity(priv),
		libp2p.DisableRelay(), // Disable relay for direct connections

		// Connection management optimizations
		libp2p.ConnectionGater(&QuicConnectionGater{}),

		// Enable connection manager for better resource management
		libp2p.EnableRelay(), // Allow relay as fallback if direct fails
	}

	host, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %v", err)
	}

	// Set the host in types package
	types.SetHost(host)
	Host = host

	// Log comprehensive host information
	log.WithFields(log.Fields{
		"peerID":    host.ID().String(),
		"addresses": host.Addrs(),
		"protocols": host.Mux().Protocols(),
	}).Info("Enhanced LibP2P host created with QUIC optimizations")

	// Log transport-specific information
	for _, addr := range host.Addrs() {
		if addr.String() != "" {
			transportType := "unknown"
			if _, err := addr.ValueForProtocol(multiaddr.P_QUIC); err == nil {
				transportType = "QUIC"
			} else if _, err := addr.ValueForProtocol(multiaddr.P_QUIC_V1); err == nil {
				transportType = "QUIC-v1"
			} else if _, err := addr.ValueForProtocol(multiaddr.P_TCP); err == nil {
				transportType = "TCP"
			}

			log.WithFields(log.Fields{
				"address":   addr.String(),
				"transport": transportType,
			}).Debug("Listening on address")
		}
	}

	return host, nil
}

// QuicConnectionGater implements connection gating for QUIC connections
type QuicConnectionGater struct{}

func (g *QuicConnectionGater) InterceptPeerDial(p peer.ID) (allow bool) {
	// Allow all peer dials by default
	// Add custom logic here for peer filtering if needed
	return true
}

func (g *QuicConnectionGater) InterceptAddrDial(p peer.ID, m multiaddr.Multiaddr) (allow bool) {
	// Allow all address dials by default
	// Add custom logic here for address filtering if needed
	return true
}

func (g *QuicConnectionGater) InterceptAccept(cm network.ConnMultiaddrs) (allow bool) {
	// Allow all connections by default
	// Add custom logic here for connection filtering if needed
	return true
}

func (g *QuicConnectionGater) InterceptSecured(network.Direction, peer.ID, network.ConnMultiaddrs) (allow bool) {
	// Allow all secured connections by default
	return true
}

func (g *QuicConnectionGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	// Allow all upgraded connections by default
	return true, 0
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
			"peerID":    host.ID().String(),
			"addresses": host.Addrs(),
		}).Info("LibP2P host initialized successfully")
	}

	return nil
}
