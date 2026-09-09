package wg

import (
	"net"
	"testing"

	"github.com/NetSepio/erebrus/internal/store"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestPeerSyncDiffNoChange(t *testing.T) {
	a := mustPeerKey(t)
	ip := mustCIDR(t, "10.0.0.2/32")
	current := []wgtypes.Peer{{PublicKey: a, AllowedIPs: []net.IPNet{ip}}}
	desired := []*store.Peer{{ID: "a", Enabled: true, WGPublicKey: a.String(), WGAllowedIP: "10.0.0.2/32"}}
	cfgs, err := peerSyncDiff(current, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 0 {
		t.Fatalf("expected no kernel updates, got %+v", cfgs)
	}
}

func TestPeerSyncDiffAddsOnlyNewPeer(t *testing.T) {
	keep := mustPeerKey(t)
	add := mustPeerKey(t)
	keepIP := mustCIDR(t, "10.0.0.2/32")
	current := []wgtypes.Peer{{PublicKey: keep, AllowedIPs: []net.IPNet{keepIP}}}
	desired := []*store.Peer{
		{ID: "keep", Enabled: true, WGPublicKey: keep.String(), WGAllowedIP: "10.0.0.2/32"},
		{ID: "add", Enabled: true, WGPublicKey: add.String(), WGAllowedIP: "10.0.0.3/32"},
	}
	cfgs, err := peerSyncDiff(current, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 || cfgs[0].PublicKey != add || cfgs[0].Remove || cfgs[0].UpdateOnly {
		t.Fatalf("expected single add of new peer, got %+v", cfgs)
	}
	if !cfgs[0].ReplaceAllowedIPs || allowedIPsKey(cfgs[0].AllowedIPs) != "10.0.0.3/32" {
		t.Fatalf("add config = %+v", cfgs[0])
	}
}

func TestPeerSyncDiffRemovesOnlyExpiredPeer(t *testing.T) {
	keep := mustPeerKey(t)
	drop := mustPeerKey(t)
	current := []wgtypes.Peer{
		{PublicKey: keep, AllowedIPs: []net.IPNet{mustCIDR(t, "10.0.0.2/32")}},
		{PublicKey: drop, AllowedIPs: []net.IPNet{mustCIDR(t, "10.0.0.3/32")}},
	}
	desired := []*store.Peer{
		{ID: "keep", Enabled: true, WGPublicKey: keep.String(), WGAllowedIP: "10.0.0.2/32"},
	}
	cfgs, err := peerSyncDiff(current, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 || cfgs[0].PublicKey != drop || !cfgs[0].Remove {
		t.Fatalf("expected single remove of dropped peer, got %+v", cfgs)
	}
}

func TestPeerSyncDiffUpdatesChangedAllowedIP(t *testing.T) {
	key := mustPeerKey(t)
	current := []wgtypes.Peer{{PublicKey: key, AllowedIPs: []net.IPNet{mustCIDR(t, "10.0.0.2/32")}}}
	desired := []*store.Peer{{ID: "a", Enabled: true, WGPublicKey: key.String(), WGAllowedIP: "10.0.0.9/32"}}
	cfgs, err := peerSyncDiff(current, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 || !cfgs[0].UpdateOnly || !cfgs[0].ReplaceAllowedIPs {
		t.Fatalf("expected update-only allowed-ip change, got %+v", cfgs)
	}
	if allowedIPsKey(cfgs[0].AllowedIPs) != "10.0.0.9/32" {
		t.Fatalf("allowed ips = %v", cfgs[0].AllowedIPs)
	}
	if cfgs[0].PresharedKey != nil {
		t.Fatalf("psk should be omitted when unchanged: %+v", cfgs[0])
	}
}

func TestPeerSyncDiffIgnoresDisabledDesiredPeers(t *testing.T) {
	key := mustPeerKey(t)
	current := []wgtypes.Peer{{PublicKey: key, AllowedIPs: []net.IPNet{mustCIDR(t, "10.0.0.2/32")}}}
	desired := []*store.Peer{{ID: "a", Enabled: false, WGPublicKey: key.String(), WGAllowedIP: "10.0.0.2/32"}}
	cfgs, err := peerSyncDiff(current, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 || !cfgs[0].Remove {
		t.Fatalf("disabled desired peer should be removed, got %+v", cfgs)
	}
}

func mustPeerKey(t *testing.T) wgtypes.Key {
	t.Helper()
	k, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k.PublicKey()
}

func mustCIDR(t *testing.T, cidr string) net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	return *n
}
