// Package stealth runs the node's DPI-resistant carrier transports via an
// embedded sing-box instance. When a client's WireGuard UDP is throttled or
// blocked, it wraps the same WireGuard tunnel inside one of two carriers that
// look like ordinary internet traffic:
//
//   - VLESS + REALITY on tcp/:443 — indistinguishable from a real TLS session
//     to a borrowed SNI (no fake cert; the handshake is proxied to a real site).
//   - Hysteria2 on udp/:443 — QUIC/HTTP3 with optional Salamander obfuscation.
//
// VPN carrier traffic is pinned to the local WireGuard listener. A dedicated
// VLESS bootstrap credential can also reach the configured HTTPS gateway.
package stealth

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/google/uuid"
	box "github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
)

const (
	vlessFlowVision = "xtls-rprx-vision"
	vlessInboundTag = "vless-reality"
	bootstrapUser   = "bootstrap"
	// The retired bootstrap identity. Routed exactly like bootstrapUser, so a
	// client still holding it reaches the gateway rather than WireGuard.
	bootstrapPrevUser = "bootstrap-previous"
	// How long a descriptor retired by a short-id rotation keeps working. The
	// gateway's rotate_reality command carries no window of its own, and the
	// scheduled carrier rotation defaults to the same 24 hours.
	descriptorGrace = 24 * time.Hour
)

// Manager owns the embedded sing-box instance and the node-wide carrier secrets.
type Manager struct {
	cfg         *config.Config
	st          SettingsStore
	gatewayHost string
	secrets     *Secrets
	certPEM     string
	keyPEM      string

	mu sync.Mutex
	// instance is the live sing-box, nil whenever nothing is listening.
	instance *box.Box
	// want records whether the carrier is supposed to be listening. instance is
	// nil both when the carrier is deliberately down and when a start failed to
	// bind, so restarts keyed on instance alone turned into no-ops after one
	// failure: the carrier stayed offline while rotation reported success.
	want bool
	// servedUUID and servedShortID are the descriptor the running listener last
	// accepted, which is not always the one the settings hold: a rotation whose
	// restart failed has written a credential the network never saw. Retiring
	// that one would drop the descriptor clients are actually holding, inside
	// its own grace window.
	servedUUID    string
	servedShortID string
	// graceExpiry rebuilds the listener when the retired descriptor's window
	// closes. Without it the deadline is advisory only: the retired identity
	// lives on the running listener until something else rebuilds it, which is
	// the next rotation — days after the window the descriptor was retired
	// with, and under an operator's nose who was told it had expired.
	graceExpiry *time.Timer
}

// New constructs a Manager. Call Init before Start or Params.
func New(cfg *config.Config, st SettingsStore) *Manager {
	return &Manager{cfg: cfg, st: st, gatewayHost: bootstrapHost(cfg.GatewayURL)}
}

// Enabled reports whether the stealth carriers are turned on.
func (m *Manager) Enabled() bool { return m.cfg.EnableStealth }

// Init loads (creating on first run) the node-wide carrier secrets and the
// Hysteria2 self-signed certificate. Safe to call even when stealth is disabled
// — it makes Params usable without starting the listeners.
func (m *Manager) Init(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.instance != nil {
		return nil
	}
	return m.init(ctx)
}

func (m *Manager) init(ctx context.Context) error {
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
	// The stored descriptor counts as served: it is what the gateway was told
	// while this process, or the one before it, was running. Only a rotation
	// that fails to bind leaves this pointing at the older descriptor, which is
	// the one still in clients' hands.
	m.servedUUID, m.servedShortID = secrets.VLESSBootstrapUUID, secrets.RealityShortID
	// A node restarted inside a grace window still has to expire it on time.
	m.scheduleGraceExpiry()
	return nil
}

// Start builds and starts the embedded sing-box instance. No-op when stealth is
// disabled. Init must have been called first.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.start(ctx)
}

func (m *Manager) start(ctx context.Context) error {
	if !m.cfg.EnableStealth {
		return nil
	}
	if m.secrets == nil {
		return fmt.Errorf("stealth: Init not called")
	}
	// Set before anything that can fail, so a start that cannot bind is still
	// remembered as wanted and the next rotation retries it.
	m.want = true

	if m.instance != nil {
		return nil
	}
	opts := m.serverOptions()
	boxCtx := box.Context(
		ctx,
		inboundRegistry(),
		outboundRegistry(),
		endpointRegistry(),
		dnsTransportRegistry(),
		serviceRegistry(),
		certificateProviderRegistry(),
	)
	instance, err := box.New(box.Options{Context: boxCtx, Options: opts})
	if err != nil {
		return fmt.Errorf("stealth: build sing-box: %w", err)
	}
	if err := instance.Start(); err != nil {
		_ = instance.Close()
		return fmt.Errorf("stealth: start sing-box: %w", err)
	}

	m.instance = instance
	m.servedUUID, m.servedShortID = m.secrets.VLESSBootstrapUUID, m.secrets.RealityShortID
	return nil
}

// retire returns the descriptor a rotation should keep working, and the second
// it stops being accepted. It is the one the listener actually served: pass a
// non-positive grace to retire it at once. When nothing has been served — a
// carrier that has never come up, or whose last rotation could not bind — the
// window already open is left exactly as it is.
func (m *Manager) retire(grace time.Duration) (uuid, shortID string, until int64) {
	if grace <= 0 {
		return "", "", 0
	}
	if m.servedUUID == "" && m.servedShortID == "" {
		// Nothing has ever been served, so there is nothing new to retire and
		// any window already open stays exactly as it is.
		return m.secrets.VLESSBootstrapPrevUUID, m.secrets.RealityShortIDPrev, m.secrets.VLESSBootstrapPrevUntil
	}
	return m.servedUUID, m.servedShortID, time.Now().Add(grace).Unix()
}

// RotateAllSecrets regenerates both VLESS UUIDs, the REALITY short-id, and Hysteria2
// password, then restarts sing-box if it was running. The outgoing bootstrap
// credential stays valid for grace so clients that reach the gateway only
// through this carrier can still collect the replacement descriptor; pass a
// non-positive grace to retire it immediately.
func (m *Manager) RotateAllSecrets(ctx context.Context, grace time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.st == nil {
		return fmt.Errorf("stealth: not initialized")
	}
	if m.secrets == nil {
		if err := m.init(ctx); err != nil {
			return err
		}
	}
	next := *m.secrets
	// A descriptor is only usable whole, so the retired bootstrap UUID is kept
	// together with the short-id it was published alongside. Only one
	// generation overlaps: what is retired now replaces whatever the previous
	// rotation left behind.
	next.VLESSBootstrapPrevUUID, next.RealityShortIDPrev, next.VLESSBootstrapPrevUntil = m.retire(grace)
	next.VLESSUUID = uuid.NewString()
	next.VLESSBootstrapUUID = uuid.NewString()
	next.RealityShortID = randHex(4)
	next.Hysteria2Password = randToken(24)
	if err := m.st.SetSettings(ctx, map[string]string{
		keyVLESSUUID:               next.VLESSUUID,
		keyVLESSBootstrapUUID:      next.VLESSBootstrapUUID,
		keyVLESSBootstrapPrev:      next.VLESSBootstrapPrevUUID,
		keyRealityShortIDPrev:      next.RealityShortIDPrev,
		keyVLESSBootstrapPrevUntil: strconv.FormatInt(next.VLESSBootstrapPrevUntil, 10),
		keyRealityShortID:          next.RealityShortID,
		keyHysteria2Pass:           next.Hysteria2Password,
	}); err != nil {
		return err
	}
	m.secrets = &next
	m.scheduleGraceExpiry()
	return m.rotated(ctx)
}

// rotated brings the listener up on the credentials just written and records
// them as served. A restart that fails leaves the served descriptor where it
// was: those credentials were never published, so the next rotation must retire
// the older ones — the pair clients are actually holding — and not these.
func (m *Manager) rotated(ctx context.Context) error {
	if err := m.restartIfRunning(ctx); err != nil {
		return err
	}
	m.servedUUID, m.servedShortID = m.secrets.VLESSBootstrapUUID, m.secrets.RealityShortID
	return nil
}

// RotateReality regenerates the REALITY short-id, restarts sing-box, and returns
// the new short-id. The keypair is kept per ws-protocol rotate_reality semantics.
//
// The published bootstrap descriptor names the short-id, and a client holding
// one cannot complete the handshake without it, so the outgoing short-id is
// retired exactly as a full rotation retires a credential: a command meant only
// to change an identifier must not lock out the blocked clients the bootstrap
// carrier exists for.
func (m *Manager) RotateReality(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.st == nil || m.secrets == nil {
		return "", fmt.Errorf("stealth: not initialized")
	}
	shortID := randHex(4)
	retiredUUID, retiredShortID, until := m.retire(descriptorGrace)
	if err := m.st.SetSettings(ctx, map[string]string{
		keyRealityShortID:          shortID,
		keyRealityShortIDPrev:      retiredShortID,
		keyVLESSBootstrapPrev:      retiredUUID,
		keyVLESSBootstrapPrevUntil: strconv.FormatInt(until, 10),
	}); err != nil {
		return "", err
	}
	m.secrets.RealityShortID = shortID
	m.secrets.RealityShortIDPrev = retiredShortID
	m.secrets.VLESSBootstrapPrevUUID = retiredUUID
	m.secrets.VLESSBootstrapPrevUntil = until
	m.scheduleGraceExpiry()
	if err := m.rotated(ctx); err != nil {
		return "", err
	}
	return shortID, nil
}

// scheduleGraceExpiry arms the rebuild that drops a retired descriptor at its
// deadline. Call with the lock held, after m.secrets is in its new state.
func (m *Manager) scheduleGraceExpiry() {
	if m.graceExpiry != nil {
		m.graceExpiry.Stop()
		m.graceExpiry = nil
	}
	if _, _, ok := m.secrets.PreviousDescriptor(time.Now()); !ok {
		return
	}
	// Deadlines are stored to the second, so a timer armed from a sub-second
	// remainder can fire while the window still reads as open. A small floor
	// keeps that from turning into a retry spin.
	delay := time.Until(time.Unix(m.secrets.VLESSBootstrapPrevUntil, 0))
	if delay < 50*time.Millisecond {
		delay = 50 * time.Millisecond
	}
	m.graceExpiry = time.AfterFunc(delay, m.expireGrace)
}

func (m *Manager) expireGrace() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.secrets == nil {
		return
	}
	if _, _, ok := m.secrets.PreviousDescriptor(time.Now()); ok {
		// A rotation moved the deadline out while this timer was waiting.
		m.scheduleGraceExpiry()
		return
	}
	if err := m.restartIfRunning(context.Background()); err != nil {
		slog.Warn("stealth: carrier not rebuilt after the grace window closed", "err", err)
	}
}

func (m *Manager) restartIfRunning(ctx context.Context) error {
	if !m.want {
		return nil
	}
	if err := m.close(); err != nil {
		return err
	}
	return m.start(ctx)
}

// Close stops the embedded sing-box instance. Idempotent.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// A deliberate stop, unlike the close inside a restart.
	m.want = false
	if m.graceExpiry != nil {
		m.graceExpiry.Stop()
		m.graceExpiry = nil
	}
	return m.close()
}

func (m *Manager) close() error {
	inst := m.instance
	m.instance = nil
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

	var rules []option.Rule
	if m.gatewayHost != "" {
		rules = append(rules, bootstrapRule(m.gatewayHost))
	}
	// Every other request, including misses from the bootstrap user, goes to WireGuard.
	rules = append(rules, option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{RuleAction: option.RuleAction{
			Action: C.RuleActionTypeRouteOptions,
			RouteOptionsOptions: option.RouteOptionsActionOptions{
				OverrideAddress: "127.0.0.1",
				OverridePort:    uint16(m.cfg.WGEndpointPortInt()),
			},
		}},
	})
	return option.Options{
		Log:      &option.LogOptions{Level: logLevel, Timestamp: true},
		Inbounds: inbounds,
		Outbounds: []option.Outbound{{
			Type:    C.TypeDirect,
			Tag:     "direct",
			Options: &option.DirectOutboundOptions{},
		}},
		Route: &option.RouteOptions{
			Rules: rules,
			Final: "direct",
		},
	}
}

func bootstrapRule(gateway string) option.Rule {
	match := option.RawDefaultRule{
		Inbound:  []string{vlessInboundTag},
		Network:  []string{"tcp"},
		AuthUser: []string{bootstrapUser, bootstrapPrevUser},
		Port:     []uint16{443},
	}
	if ip, err := netip.ParseAddr(gateway); err == nil {
		match.IPCIDR = []string{netip.PrefixFrom(ip, ip.BitLen()).String()}
	} else {
		match.Domain = []string{gateway}
	}
	return option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{
			RawDefaultRule: match,
			RuleAction: option.RuleAction{
				Action: C.RuleActionTypeRoute,
				RouteOptions: option.RouteActionOptions{
					Outbound: "direct",
					// Always dial the configured authority, including when metadata carries a sniffed name.
					RawRouteOptionsActionOptions: option.RawRouteOptionsActionOptions{OverrideAddress: gateway, OverridePort: 443},
				},
			},
		},
	}
}

// bootstrapHost returns the exact control-plane host for the HTTPS/443 carrier.
func bootstrapHost(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if ip, err := netip.ParseAddr(host); err == nil && ip.Zone() == "" {
		return ip.Unmap().String()
	}
	if host == "" || !M.IsDomainName(host) {
		return ""
	}
	return host
}

// vlessUsers lists the carrier identities the listener accepts. A retired
// bootstrap credential is offered only while its window is open; the rebuild
// armed by scheduleGraceExpiry drops it at the deadline.
func (m *Manager) vlessUsers() []option.VLESSUser {
	users := []option.VLESSUser{
		{Name: "erebrus", UUID: m.secrets.VLESSUUID, Flow: vlessFlowVision},
		{Name: bootstrapUser, UUID: m.secrets.VLESSBootstrapUUID, Flow: vlessFlowVision},
	}
	if prev, _, ok := m.secrets.PreviousDescriptor(time.Now()); ok && prev != "" && prev != m.secrets.VLESSBootstrapUUID {
		users = append(users, option.VLESSUser{Name: bootstrapPrevUser, UUID: prev, Flow: vlessFlowVision})
	}
	return users
}

// realityShortIDs lists the short-ids the handshake accepts. The retired one
// belongs here for as long as the descriptor that names it is honoured: it is
// checked before VLESS authentication, so leaving it out makes the retired
// credential unusable however long it is kept.
func (m *Manager) realityShortIDs() badoption.Listable[string] {
	ids := badoption.Listable[string]{m.secrets.RealityShortID}
	if _, prev, ok := m.secrets.PreviousDescriptor(time.Now()); ok && prev != "" && prev != m.secrets.RealityShortID {
		ids = append(ids, prev)
	}
	return ids
}

func (m *Manager) vlessInbound() option.Inbound {
	host, port := splitHostPort(m.cfg.RealityHandshakeTarget(), 443)
	return option.Inbound{
		Type: C.TypeVLESS,
		Tag:  vlessInboundTag,
		Options: &option.VLESSInboundOptions{
			ListenOptions: listenOn(m.cfg.VLESSPortInt()),
			Users:         m.vlessUsers(),
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
						ShortID:    m.realityShortIDs(),
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
