package p2p

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p"
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
func Start(ctx context.Context, mnemonic, listenPort, gatewayMultiaddr string) (*Node, error) {
	priv, err := DeriveIdentity(mnemonic)
	if err != nil {
		return nil, err
	}

	h, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", listenPort)),
		libp2p.Identity(priv),
		libp2p.DisableRelay(),
	)
	if err != nil {
		return nil, fmt.Errorf("libp2p host: %w", err)
	}

	kad, err := dht.New(ctx, h, dht.Mode(dht.ModeAuto))
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
