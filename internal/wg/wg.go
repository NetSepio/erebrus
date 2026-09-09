// Package wg manages the node's WireGuard server: the server keypair (stored
// in SQLite node_settings), rendering the interface config and per-client
// configs, and applying peer changes live. It carries forward the v1 template
// and wgctrl logic but sources state from the store instead of JSON files.
package wg

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/netcidr"
	"github.com/NetSepio/erebrus/internal/store"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	settingServerPrivateKey = "wg_server_private_key"
	settingServerPublicKey  = "wg_server_public_key"
)

// Manager owns the node's WireGuard state.
type Manager struct {
	cfg  *config.Config
	st   *store.Store
	ctrl Controller

	applyMu    sync.Mutex
	mu         sync.RWMutex
	privateKey string
	publicKey  string
}

// New constructs a Manager. Call Init before use.
func New(cfg *config.Config, st *store.Store, ctrl Controller) *Manager {
	return &Manager{cfg: cfg, st: st, ctrl: ctrl}
}

// Init loads or generates the server keypair, writes the interface config, and
// brings the interface up. A failure to bring the interface up (e.g. running
// without NET_ADMIN in local dev) is logged by the caller but not fatal — the
// conf file is still written for later activation.
func (m *Manager) Init(ctx context.Context) error {
	if err := m.loadOrCreateKeys(ctx); err != nil {
		return err
	}
	if err := os.MkdirAll(m.cfg.WGConfDir, 0o700); err != nil {
		return err
	}
	if err := m.writeServerConf(ctx); err != nil {
		return err
	}
	if err := m.ctrl.BringUp(m.cfg.WGInterface, m.confPath()); err != nil {
		return err
	}
	go m.expiryLoop(ctx)
	return nil
}

// ServerPublicKey returns the node's WireGuard public key.
func (m *Manager) ServerPublicKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.publicKey
}

// Endpoint returns host:port clients should dial.
func (m *Manager) Endpoint() string {
	return fmt.Sprintf("%s:%s", m.cfg.WGEndpointHost, m.cfg.WGEndpointPort)
}

// Subnet returns the configured IPv4 subnet (server host CIDR).
func (m *Manager) Subnet() string { return m.cfg.WGIPv4Subnet }

// Stats returns a live device snapshot (transfer counters, active peers).
// Returns a zero value when the interface is not up (e.g. dev without NET_ADMIN).
func (m *Manager) Stats() DeviceStats {
	st, err := m.ctrl.Stats(m.cfg.WGInterface)
	if err != nil {
		return DeviceStats{}
	}
	return st
}

// PeerTransfers returns per-peer transfer counters keyed by WG public key.
func (m *Manager) PeerTransfers() []PeerTransfer {
	pt, err := m.ctrl.PeerTransfers(m.cfg.WGInterface)
	if err != nil {
		return nil
	}
	return pt
}

// Apply re-renders the interface config from the current peer set and syncs the
// live peer list. Call after any peer add/update/remove.
func (m *Manager) Apply(ctx context.Context) error {
	m.applyMu.Lock()
	defer m.applyMu.Unlock()
	if err := m.writeServerConf(ctx); err != nil {
		return err
	}
	peers, err := m.st.ListPeers(ctx)
	if err != nil {
		return err
	}
	return m.ctrl.SyncPeers(m.cfg.WGInterface, m.tunnelPeers(activePeers(peers, time.Now())))
}

// ClientConfig renders a wg-quick config for a peer, with the private key left
// as a placeholder for the client to fill in.
func (m *Manager) ClientConfig(p *store.Peer) (string, error) {
	return renderClient(clientTplData{
		Address:         m.ClientAddresses(p),
		DNS:             m.cfg.WGDNS,
		ServerPublicKey: m.ServerPublicKey(),
		PresharedKey:    p.WGPresharedKey,
		Endpoint:        m.Endpoint(),
	})
}

// ClientAddresses is the dual-stack tunnel address list for a peer.
func (m *Manager) ClientAddresses(p *store.Peer) string {
	if p == nil {
		return ""
	}
	addrs := []string{p.WGAllowedIP}
	if v6 := m.peerIPv6CIDR(p); v6 != "" {
		addrs = append(addrs, v6)
	}
	return strings.Join(addrs, ", ")
}

func (m *Manager) loadOrCreateKeys(ctx context.Context) error {
	priv, err := m.st.GetSetting(ctx, settingServerPrivateKey)
	if err != nil {
		return err
	}
	if priv == "" {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		priv = key.String()
		pub := key.PublicKey().String()
		if err := m.st.SetSetting(ctx, settingServerPrivateKey, priv); err != nil {
			return err
		}
		if err := m.st.SetSetting(ctx, settingServerPublicKey, pub); err != nil {
			return err
		}
	}
	pub, err := m.st.GetSetting(ctx, settingServerPublicKey)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.privateKey = priv
	m.publicKey = pub
	m.mu.Unlock()
	return nil
}

func (m *Manager) writeServerConf(ctx context.Context) error {
	peers, err := m.st.ListPeers(ctx)
	if err != nil {
		return err
	}
	m.mu.RLock()
	priv := m.privateKey
	m.mu.RUnlock()

	data, err := renderServer(serverTplData{
		Address:    m.serverAddress(),
		ListenPort: m.cfg.WGEndpointPortInt(),
		PrivateKey: priv,
		PreUp:      m.cfg.WGPreUp,
		PostUp:     m.cfg.WGPostUp,
		PreDown:    m.cfg.WGPreDown,
		PostDown:   m.cfg.WGPostDown,
		Peers:      m.tunnelPeers(activePeers(peers, time.Now())),
	})
	if err != nil {
		return err
	}
	return writeFileAtomic(m.confPath(), data, 0o600)
}

func activePeers(peers []*store.Peer, now time.Time) []*store.Peer {
	cutoff := now.Unix()
	out := make([]*store.Peer, 0, len(peers))
	for _, peer := range peers {
		if peer.Enabled && (peer.ExpiresAt == 0 || peer.ExpiresAt > cutoff) {
			out = append(out, peer)
		}
	}
	return out
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".wg-conf-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	if err = tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (m *Manager) expiryLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.expireOnce(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("wireguard expiry sync failed", "err", err)
			}
		}
	}
}

func (m *Manager) expireOnce(ctx context.Context) error {
	peers, err := m.st.ListPeers(ctx)
	if err != nil {
		return err
	}
	if !hasExpiredEnabledPeer(peers, time.Now()) {
		return nil
	}
	return m.Apply(ctx)
}

func hasExpiredEnabledPeer(peers []*store.Peer, now time.Time) bool {
	cutoff := now.Unix()
	for _, peer := range peers {
		if peer.Enabled && peer.ExpiresAt > 0 && peer.ExpiresAt <= cutoff {
			return true
		}
	}
	return false
}

func (m *Manager) tunnelPeers(peers []*store.Peer) []*store.Peer {
	out := make([]*store.Peer, len(peers))
	for i, p := range peers {
		cp := *p
		cp.WGAllowedIP = m.ClientAddresses(p)
		out[i] = &cp
	}
	return out
}

func (m *Manager) peerIPv6CIDR(p *store.Peer) string {
	if m.cfg == nil || m.cfg.WGIPv6Subnet == "" || p == nil {
		return ""
	}
	ip, _, err := net.ParseCIDR(p.WGAllowedIP)
	if err != nil {
		return ""
	}
	v4net, err := netcidr.ParseIPv4Subnet(m.cfg.WGIPv4Subnet)
	if err != nil {
		return ""
	}
	v6net, err := netcidr.ParseIPv6Subnet(m.cfg.WGIPv6Subnet)
	if err != nil {
		return ""
	}
	off, err := netcidr.HostOffset(ip, v4net)
	if err != nil {
		return ""
	}
	return netcidr.AddOffset(v6net.IP, off).String() + "/128"
}

// serverAddress returns the server's dual-stack interface addresses.
func (m *Manager) serverAddress() string {
	v4 := netcidr.HostCIDR(m.cfg.WGIPv4Subnet)
	if m.cfg.WGIPv6Subnet == "" {
		return v4
	}
	return v4 + ", " + netcidr.HostCIDR(m.cfg.WGIPv6Subnet)
}

func (m *Manager) confPath() string {
	name := m.cfg.WGInterface
	if !strings.HasSuffix(name, ".conf") {
		name += ".conf"
	}
	return filepath.Join(m.cfg.WGConfDir, name)
}
