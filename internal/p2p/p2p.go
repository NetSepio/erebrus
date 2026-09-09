package p2p

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
)

// rendezvous is the DHT advertisement tag shared by all Erebrus nodes.
const rendezvous = "erebrus"

// Node is the running libp2p host plus its DHT.
type Node struct {
	Host host.Host
	DHT  *dht.IpfsDHT
	did  string
}

// Start brings up the libp2p host with a deterministic identity, connects to
// the gateway bootstrap peer (if configured), and advertises on the DHT. It
// returns a started Node; call Close to stop it.
func Start(ctx context.Context, mnemonic, listenPort, publicHost, gatewayMultiaddr string) (*Node, error) {
	priv, err := DeriveIdentity(mnemonic)
	if err != nil {
		return nil, err
	}

	listen := []string{fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", listenPort)}
	if ipv6Listening() {
		listen = append(listen, fmt.Sprintf("/ip6/::/tcp/%s", listenPort))
	}
	options := []libp2p.Option{
		libp2p.ListenAddrStrings(listen...),
		libp2p.Identity(priv),
		libp2p.DisableRelay(),
	}
	if advertised, err := advertisedAddresses(publicHost, listenPort); err != nil {
		return nil, fmt.Errorf("public p2p address: %w", err)
	} else if len(advertised) > 0 {
		options = append(options, libp2p.AddrsFactory(func(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
			return append(addrs, advertised...)
		}))
	}
	h, err := libp2p.New(options...)
	if err != nil {
		return nil, fmt.Errorf("libp2p host: %w", err)
	}

	kad, err := dht.New(h, dht.Mode(dht.ModeAuto))
	if err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("dht: %w", err)
	}
	if err := kad.Bootstrap(ctx); err != nil {
		slog.Warn("dht bootstrap", "err", err)
	}

	n := &Node{Host: h, DHT: kad, did: DIDPrefix + h.ID().String()}

	if gatewayMultiaddr != "" {
		if err := n.connectBootstrap(ctx, gatewayMultiaddr); err != nil {
			slog.Warn("gateway bootstrap connect failed", "err", err)
		}
	}

	// Advertise ourselves under the shared rendezvous tag.
	rd := drouting.NewRoutingDiscovery(kad)
	dutil.Advertise(ctx, rd, rendezvous)

	slog.Info("libp2p host started",
		"peer_id", h.ID().String(), "did", n.did, "addrs", h.Addrs())
	return n, nil
}

func advertisedAddresses(host, port string) ([]multiaddr.Multiaddr, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, nil
	}
	if ip := net.ParseIP(host); ip != nil {
		ma, err := advertisedAddress(host, port)
		if err != nil || ma == nil {
			return nil, err
		}
		return []multiaddr.Multiaddr{ma}, nil
	}
	a4, err := multiaddr.NewMultiaddr(fmt.Sprintf("/dns4/%s/tcp/%s", host, port))
	if err != nil {
		return nil, err
	}
	a6, err := multiaddr.NewMultiaddr(fmt.Sprintf("/dns6/%s/tcp/%s", host, port))
	if err != nil {
		return []multiaddr.Multiaddr{a4}, nil
	}
	return []multiaddr.Multiaddr{a4, a6}, nil
}

func advertisedAddress(host, port string) (multiaddr.Multiaddr, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, nil
	}
	protocol := "dns4"
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			protocol = "ip4"
			host = ip.String()
		} else {
			protocol = "ip6"
			host = ip.String()
		}
	}
	return multiaddr.NewMultiaddr(fmt.Sprintf("/%s/%s/tcp/%s", protocol, host, port))
}

func ipv6Listening() bool {
	ln, err := net.Listen("tcp6", "[::]:0")
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func (n *Node) connectBootstrap(ctx context.Context, addr string) error {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return err
	}
	pi, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return n.Host.Connect(cctx, *pi)
}

// PeerID returns the node's PeerID string.
func (n *Node) PeerID() string { return n.Host.ID().String() }

// DID returns the node's did:erebrus identifier.
func (n *Node) DID() string { return n.did }

// Close shuts down the DHT and host.
func (n *Node) Close() error {
	if n.DHT != nil {
		_ = n.DHT.Close()
	}
	return n.Host.Close()
}
