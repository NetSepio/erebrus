// Package node is the node's core service: it ties the SQLite store and the
// WireGuard manager together to provision peers and build credential bundles.
// Phase 2 extends Service with sing-box (VLESS/Hysteria2) provisioning; the
// api.Provisioner interface it satisfies stays unchanged.
package node

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"github.com/NetSepio/erebrus/internal/api"
	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/stealth"
	"github.com/NetSepio/erebrus/internal/store"
	"github.com/NetSepio/erebrus/internal/telemetry"
	"github.com/NetSepio/erebrus/internal/wg"
	"github.com/google/uuid"
)

// Service provisions peers across all protocols and renders credential bundles.
type Service struct {
	cfg     *config.Config
	st      *store.Store
	wg      *wg.Manager
	stealth *stealth.Manager
	metrics *telemetry.Metrics
}

// New constructs the node service. stealthMgr may be nil when the stealth
// carriers are not in use.
func New(cfg *config.Config, st *store.Store, wgm *wg.Manager, stealthMgr *stealth.Manager, m *telemetry.Metrics) *Service {
	return &Service{cfg: cfg, st: st, wg: wgm, stealth: stealthMgr, metrics: m}
}

// UpsertPeer creates or updates a peer and returns its credential bundle. The
// store allocates the WireGuard IP and persists generated proxy credentials
// atomically; the WireGuard interface is then synced live.
func (s *Service) UpsertPeer(ctx context.Context, id string, req api.PeerRequest) (*api.CredentialBundle, error) {
	if id == "" {
		id = uuid.NewString()
	}
	gen := store.GeneratedCreds{
		ProxyUUID:     uuid.NewString(),
		ProxyPassword: randomToken(24),
	}
	in := &store.Peer{
		ID:             id,
		Name:           req.Name,
		Wallet:         req.Wallet,
		WGPublicKey:    req.WGPublicKey,
		WGPresharedKey: req.WGPresharedKey,
		Enabled:        true,
		ExpiresAt:      req.ExpiresAt,
	}
	peer, err := s.st.UpsertPeer(ctx, in, s.wg.Subnet(), gen)
	if err != nil {
		return nil, err
	}
	if err := s.wg.Apply(ctx); err != nil {
		return nil, err
	}
	if s.metrics != nil {
		s.metrics.PeerProvisioned.Inc()
		s.updatePeerGauge(ctx)
	}
	return s.buildBundle(peer)
}

// DeletePeer removes a peer and re-syncs WireGuard. Idempotent.
func (s *Service) DeletePeer(ctx context.Context, id string) error {
	if err := s.st.DeletePeer(ctx, id); err != nil {
		return err
	}
	if err := s.wg.Apply(ctx); err != nil {
		return err
	}
	if s.metrics != nil {
		s.metrics.PeerDeprovisioned.Inc()
		s.updatePeerGauge(ctx)
	}
	return nil
}

// Credentials re-fetches the bundle for an existing peer.
func (s *Service) Credentials(ctx context.Context, id string) (*api.CredentialBundle, error) {
	peer, err := s.st.GetPeer(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.buildBundle(peer)
}

// ListPeers returns metadata-only peer info.
func (s *Service) ListPeers(ctx context.Context) ([]api.PeerInfo, error) {
	peers, err := s.st.ListPeers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.PeerInfo, 0, len(peers))
	for _, p := range peers {
		out = append(out, api.PeerInfo{
			ID: p.ID, Name: p.Name, WGAllowedIP: p.WGAllowedIP,
			Enabled: p.Enabled, CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt,
		})
	}
	return out, nil
}

func (s *Service) buildBundle(p *store.Peer) (*api.CredentialBundle, error) {
	conf, err := s.wg.ClientConfig(p)
	if err != nil {
		return nil, err
	}
	bundle := &api.CredentialBundle{
		ID: p.ID,
		WireGuard: api.WireGuardBundle{
			ClientConf:      conf,
			ServerPublicKey: s.wg.ServerPublicKey(),
			Endpoint:        s.wg.Endpoint(),
			Address:         p.WGAllowedIP,
			DNS:             s.cfg.WGDNS,
		},
	}
	// Stealth carriers (when enabled): the same WireGuard tunnel, wrapped in a
	// DPI-resistant transport for clients whose UDP is blocked.
	if s.stealth != nil && s.stealth.Enabled() {
		label := p.Name
		if label == "" {
			label = s.cfg.NodeName
		}
		ps := s.stealth.BuildPeer(label, s.wg.ServerPublicKey(), p.WGAllowedIP, p.WGPresharedKey)
		bundle.VLESSURI = ps.VLESSURI
		bundle.Hysteria2URI = ps.Hysteria2URI
		bundle.SingboxProfile = ps.SingboxProfile
	}
	return bundle, nil
}

func (s *Service) updatePeerGauge(ctx context.Context) {
	peers, err := s.st.ListPeers(ctx)
	if err != nil {
		return
	}
	s.metrics.WGPeers.Set(float64(len(peers)))
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
