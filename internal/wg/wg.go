// Package wg manages the node's WireGuard server: the server keypair (stored
// in SQLite node_settings), rendering the interface config and per-client
// configs, and applying peer changes live. It carries forward the v1 template
// and wgctrl logic but sources state from the store instead of JSON files.
package wg

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/NetSepio/erebrus/internal/config"
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
	return m.ctrl.BringUp(m.cfg.WGInterface, m.confPath())
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

// Apply re-renders the interface config from the current peer set and syncs the
// live peer list. Call after any peer add/update/remove.
func (m *Manager) Apply(ctx context.Context) error {
	if err := m.writeServerConf(ctx); err != nil {
		return err
	}
	peers, err := m.st.ListPeers(ctx)
	if err != nil {
		return err
	}
	return m.ctrl.SyncPeers(m.cfg.WGInterface, peers)
}

// ClientConfig renders a wg-quick config for a peer, with the private key left
// as a placeholder for the client to fill in.
func (m *Manager) ClientConfig(p *store.Peer) (string, error) {
	return renderClient(clientTplData{
		Address:         p.WGAllowedIP,
		DNS:             m.cfg.WGDNS,
		ServerPublicKey: m.ServerPublicKey(),
		PresharedKey:    p.WGPresharedKey,
		Endpoint:        m.Endpoint(),
	})
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
		Peers:      peers,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(m.confPath(), data, 0o600)
}

// serverAddress returns the server's own address inside the subnet as a CIDR,
// e.g. "10.0.0.1/16".
func (m *Manager) serverAddress() string {
	ip, ipnet, err := net.ParseCIDR(m.cfg.WGIPv4Subnet)
	if err != nil {
		return m.cfg.WGIPv4Subnet
	}
	ones, _ := ipnet.Mask.Size()
	return fmt.Sprintf("%s/%d", ip.String(), ones)
}

func (m *Manager) confPath() string {
	name := m.cfg.WGInterface
	if !strings.HasSuffix(name, ".conf") {
		name += ".conf"
	}
	return filepath.Join(m.cfg.WGConfDir, name)
}
