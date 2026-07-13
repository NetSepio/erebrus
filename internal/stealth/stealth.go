// Package stealth runs the node's DPI-resistant carrier transports via an
// embedded sing-box instance. When a client's WireGuard UDP is throttled or
// blocked, it wraps the same WireGuard tunnel inside one of two carriers that
// look like ordinary internet traffic:
//
//   - VLESS + REALITY on tcp/:443 — indistinguishable from a real TLS session
//     to a borrowed SNI (no fake cert; the handshake is proxied to a real site).
//   - Hysteria2 on udp/:443 — QUIC/HTTP3 with optional Salamander obfuscation.
//
// Both carriers terminate on a single node-wide credential and route to a
// direct outbound; per-client authentication stays in the inner WireGuard
// tunnel, so the sing-box instance never restarts on peer churn (Topology A —
// "WireGuard as the endpoint").
package stealth

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"sync"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/google/uuid"
	box "github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
)

const vlessFlowVision = "xtls-rprx-vision"

// Manager owns the embedded sing-box instance and the node-wide carrier secrets.
type Manager struct {
	cfg     *config.Config
	st      SettingsStore
	secrets *Secrets
	certPEM string
	keyPEM  string

	mu       sync.Mutex
	instance *box.Box
	running  bool
}

// New constructs a Manager. Call Init before Start or Params.
func New(cfg *config.Config, st SettingsStore) *Manager {
	return &Manager{cfg: cfg, st: st}
}

// Enabled reports whether the stealth carriers are turned on.
func (m *Manager) Enabled() bool { return m.cfg.EnableStealth }

// Init loads (creating on first run) the node-wide carrier secrets and the
// Hysteria2 self-signed certificate. Safe to call even when stealth is disabled
// — it makes Params usable without starting the listeners.
func (m *Manager) Init(ctx context.Context) error {
	secrets, err := loadOrCreateSecrets(ctx, m.st)
	if err != nil {
		return fmt.Errorf("stealth secrets: %w", err)
	}
	certPEM, keyPEM, err := loadOrCreateCert(ctx, m.st, m.cfg.RealitySNI())
	if err != nil {
		return fmt.Errorf("stealth cert: %w", err)
	}
	m.secrets = secrets
	m.certPEM = certPEM
	m.keyPEM = keyPEM
	return nil
}

// Start builds and starts the embedded sing-box instance. No-op when stealth is
// disabled. Init must have been called first.
func (m *Manager) Start(ctx context.Context) error {
	if !m.cfg.EnableStealth {
		return nil
	}
	if m.secrets == nil {
		return fmt.Errorf("stealth: Init not called")
	}

	opts := m.serverOptions()
	boxCtx := box.Context(ctx, inboundRegistry(), outboundRegistry(), endpointRegistry())
	instance, err := box.New(box.Options{Context: boxCtx, Options: opts})
	if err != nil {
		return fmt.Errorf("stealth: build sing-box: %w", err)
	}
	if err := instance.Start(); err != nil {
		_ = instance.Close()
		return fmt.Errorf("stealth: start sing-box: %w", err)
	}

	m.mu.Lock()
	m.instance = instance
	m.running = true
	m.mu.Unlock()
	return nil
}

// RotateAllSecrets regenerates VLESS UUID, REALITY short-id, and Hysteria2
// password, then restarts sing-box if it was running.
func (m *Manager) RotateAllSecrets(ctx context.Context) error {
	if m.st == nil {
		return fmt.Errorf("stealth: not initialized")
	}
	if m.secrets == nil {
		if err := m.Init(ctx); err != nil {
			return err
		}
	}
	if err := m.st.SetSetting(ctx, keyVLESSUUID, uuid.NewString()); err != nil {
		return err
	}
	if err := m.st.SetSetting(ctx, keyRealityShortID, randHex(4)); err != nil {
		return err
	}
	if err := m.st.SetSetting(ctx, keyHysteria2Pass, randToken(24)); err != nil {
		return err
	}
	secrets, err := loadOrCreateSecrets(ctx, m.st)
	if err != nil {
		return err
	}
	m.secrets = secrets
	wasRunning := m.running
	if wasRunning {
		_ = m.Close()
	}
	if wasRunning && m.cfg.EnableStealth {
		return m.Start(ctx)
	}
	return nil
}

// RotateReality regenerates the REALITY short-id, restarts sing-box, and returns
// the new short-id. The keypair is kept per ws-protocol rotate_reality semantics.
func (m *Manager) RotateReality(ctx context.Context) (string, error) {
	if m.st == nil || m.secrets == nil {
		return "", fmt.Errorf("stealth: not initialized")
	}
	shortID := randHex(4)
	if err := m.st.SetSetting(ctx, keyRealityShortID, shortID); err != nil {
		return "", err
	}
	m.secrets.RealityShortID = shortID
	if m.running {
		_ = m.Close()
		if err := m.Start(ctx); err != nil {
			return "", err
		}
	}
	return shortID, nil
}

// Close stops the embedded sing-box instance. Idempotent.
func (m *Manager) Close() error {
	m.mu.Lock()
	inst := m.instance
	m.instance = nil
	m.running = false
	m.mu.Unlock()
	if inst == nil {
		return nil
	}
	return inst.Close()
}

// serverOptions renders the sing-box configuration the node runs: the two
// carrier inbounds plus a single direct outbound.
func (m *Manager) serverOptions() option.Options {
	logLevel := "warn"
	if m.cfg.RunType == "debug" {
		logLevel = "info"
	}

	inbounds := []option.Inbound{m.vlessInbound()}
	if h2, ok := m.hysteria2Inbound(); ok {
		inbounds = append(inbounds, h2)
	}

	// The direct outbound is pinned to the node's local WireGuard listener:
	// every connection the carriers accept is forced to 127.0.0.1:<wg-port>,
	// regardless of the inner destination. This keeps the node from acting as
	// an open proxy for anyone holding the (shared) carrier secret — the
	// carriers can only ever deliver packets to WireGuard, where the real
	// per-client authentication happens.
	return option.Options{
		Log:      &option.LogOptions{Level: logLevel, Timestamp: true},
		Inbounds: inbounds,
		Outbounds: []option.Outbound{{
			Type: C.TypeDirect,
			Tag:  "direct",
			Options: &option.DirectOutboundOptions{
				OverrideAddress: "127.0.0.1",
				OverridePort:    uint16(m.cfg.WGEndpointPortInt()),
			},
		}},
		Route: &option.RouteOptions{Final: "direct"},
	}
}

func (m *Manager) vlessInbound() option.Inbound {
	host, port := splitHostPort(m.cfg.RealityHandshakeTarget(), 443)
	return option.Inbound{
		Type: C.TypeVLESS,
		Tag:  "vless-reality",
		Options: &option.VLESSInboundOptions{
			ListenOptions: listenOn(m.cfg.VLESSPortInt()),
			Users: []option.VLESSUser{{
				Name: "erebrus",
				UUID: m.secrets.VLESSUUID,
				Flow: vlessFlowVision,
			}},
			InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
				TLS: &option.InboundTLSOptions{
					Enabled:    true,
					ServerName: m.cfg.RealitySNI(),
					Reality: &option.InboundRealityOptions{
						Enabled: true,
						Handshake: option.InboundRealityHandshakeOptions{
							ServerOptions: option.ServerOptions{Server: host, ServerPort: port},
						},
						PrivateKey: m.secrets.RealityPrivateKey,
						ShortID:    badoption.Listable[string]{m.secrets.RealityShortID},
					},
				},
			},
		},
	}
}

func (m *Manager) hysteria2Inbound() (option.Inbound, bool) {
	tls := &option.InboundTLSOptions{
		Enabled:     true,
		ServerName:  m.cfg.RealitySNI(),
		ALPN:        badoption.Listable[string]{"h3"},
		Certificate: badoption.Listable[string]{m.certPEM},
		Key:         badoption.Listable[string]{m.keyPEM},
	}
	in := &option.Hysteria2InboundOptions{
		ListenOptions:         listenOn(m.cfg.Hysteria2PortInt()),
		IgnoreClientBandwidth: true,
		Users: []option.Hysteria2User{{
			Name:     "erebrus",
			Password: m.secrets.Hysteria2Password,
		}},
		InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: tls},
	}
	if m.cfg.Hysteria2ObfsPassword != "" {
		in.Obfs = &option.Hysteria2Obfs{Type: "salamander", Password: m.cfg.Hysteria2ObfsPassword}
	}
	return option.Inbound{Type: C.TypeHysteria2, Tag: "hysteria2", Options: in}, true
}

// listenOn builds ListenOptions bound to all interfaces on the given port.
func listenOn(port int) option.ListenOptions {
	addr := badoption.Addr(netip.IPv4Unspecified())
	return option.ListenOptions{Listen: &addr, ListenPort: uint16(port)}
}

// splitHostPort splits "host:port"; missing/invalid port falls back to def.
func splitHostPort(s string, def uint16) (string, uint16) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, def
	}
	host := s[:i]
	p, err := strconv.Atoi(s[i+1:])
	if err != nil || p <= 0 || p > 65535 {
		return host, def
	}
	return host, uint16(p)
}
