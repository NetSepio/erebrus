package wg

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/store"
)

func TestActivePeersExcludesDisabledAndExpired(t *testing.T) {
	now := time.Unix(1_000, 0)
	peers := []*store.Peer{
		{ID: "active", Enabled: true, ExpiresAt: 2_000},
		{ID: "permanent", Enabled: true},
		{ID: "expired", Enabled: true, ExpiresAt: 1_000},
		{ID: "disabled", Enabled: false},
	}
	got := activePeers(peers, now)
	if len(got) != 2 || got[0].ID != "active" || got[1].ID != "permanent" {
		t.Fatalf("active peers = %+v", got)
	}
}

func TestServerTemplateDoesNotRenderPeerMetadata(t *testing.T) {
	body, err := renderServer(serverTplData{Peers: []*store.Peer{{
		ID: "peer\nPostUp = bad", Name: "name\nPostUp = bad", Wallet: "wallet\nPostUp = bad", Enabled: true,
		WGPublicKey: "public", WGAllowedIP: "10.0.0.2/32",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "PostUp = bad") {
		t.Fatalf("peer metadata escaped into config: %s", body)
	}
}

func TestClientTemplateTunnelsIPv6(t *testing.T) {
	body, err := renderClient(clientTplData{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "0.0.0.0/0, ::/0") {
		t.Fatalf("client config should tunnel IPv6: %s", body)
	}
}

func TestClientAddressesDualStack(t *testing.T) {
	m := New(&config.Config{
		WGIPv4Subnet: "100.64.0.1/24",
		WGIPv6Subnet: "fd00:64:64::1/64",
	}, nil, nil)
	got := m.ClientAddresses(&store.Peer{WGAllowedIP: "100.64.0.7/32"})
	if got != "100.64.0.7/32, fd00:64:64::7/128" {
		t.Fatalf("addresses = %q", got)
	}
	if !strings.Contains(m.serverAddress(), "100.64.0.1/24") || !strings.Contains(m.serverAddress(), "fd00:64:64::1/64") {
		t.Fatalf("server address = %q", m.serverAddress())
	}
}

func TestHasExpiredEnabledPeer(t *testing.T) {
	now := time.Unix(1_000, 0)
	if hasExpiredEnabledPeer([]*store.Peer{
		{ID: "active", Enabled: true, ExpiresAt: 2_000},
		{ID: "permanent", Enabled: true},
	}, now) {
		t.Fatal("expected no expired peers")
	}
	if !hasExpiredEnabledPeer([]*store.Peer{
		{ID: "active", Enabled: true, ExpiresAt: 2_000},
		{ID: "expired", Enabled: true, ExpiresAt: 1_000},
	}, now) {
		t.Fatal("expected expired peer")
	}
	if hasExpiredEnabledPeer([]*store.Peer{
		{ID: "disabled-expired", Enabled: false, ExpiresAt: 1},
	}, now) {
		t.Fatal("disabled expired peers should not trigger a sync")
	}
}

func TestExpireOnceSkipsApplyWhenNothingExpired(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	key := mustPeerKey(t)
	if _, err := st.UpsertPeer(ctx, &store.Peer{
		ID: "keep", Name: "keep", WGPublicKey: key.String(), Enabled: true,
	}, "10.0.0.1/16", store.GeneratedCreds{}); err != nil {
		t.Fatal(err)
	}
	ctrl := &recordingController{}
	m := New(testWGConfig(t), st, ctrl)
	if err := m.expireOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if ctrl.syncs != 0 {
		t.Fatalf("expiry tick synced %d times; want 0", ctrl.syncs)
	}
}

func TestExpireOnceRemovesExpiredPeerAndLeavesActive(t *testing.T) {
	ctx := context.Background()
	st := testStore(t)
	keep := mustPeerKey(t)
	drop := mustPeerKey(t)
	if _, err := st.UpsertPeer(ctx, &store.Peer{
		ID: "keep", Name: "keep", WGPublicKey: keep.String(), Enabled: true,
	}, "10.0.0.1/16", store.GeneratedCreds{ProxyUUID: "keep-uuid"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertPeer(ctx, &store.Peer{
		ID: "drop", Name: "drop", WGPublicKey: drop.String(), Enabled: true,
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	}, "10.0.0.1/16", store.GeneratedCreds{ProxyUUID: "drop-uuid"}); err != nil {
		t.Fatal(err)
	}
	ctrl := &recordingController{}
	m := New(testWGConfig(t), st, ctrl)
	if err := m.expireOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if ctrl.syncs != 1 {
		t.Fatalf("expiry tick synced %d times; want 1", ctrl.syncs)
	}
	if len(ctrl.last) != 1 || ctrl.last[0].WGPublicKey != keep.String() {
		t.Fatalf("synced peers = %+v", ctrl.last)
	}
}

type recordingController struct {
	syncs int
	last  []*store.Peer
}

func (r *recordingController) BringUp(string, string) error { return nil }
func (r *recordingController) SyncPeers(_ string, peers []*store.Peer) error {
	r.syncs++
	r.last = peers
	return nil
}
func (r *recordingController) Stats(string) (DeviceStats, error) { return DeviceStats{}, nil }
func (r *recordingController) PeerTransfers(string) ([]PeerTransfer, error) {
	return nil, nil
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "erebrus.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func testWGConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		WGConfDir:      t.TempDir(),
		WGInterface:    "wg0",
		WGEndpointHost: "127.0.0.1",
		WGEndpointPort: "51820",
		WGIPv4Subnet:   "10.0.0.1/16",
	}
}
