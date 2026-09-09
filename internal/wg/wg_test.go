package wg

import (
	"strings"
	"testing"
	"time"

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

func TestClientTemplateDoesNotAdvertiseUnsupportedIPv6(t *testing.T) {
	body, err := renderClient(clientTplData{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "::/0") {
		t.Fatalf("client config advertises unsupported IPv6: %s", body)
	}
}
