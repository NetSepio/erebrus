package wg

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/NetSepio/erebrus/internal/store"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// peerSyncDiff returns the minimum wgctrl peer operations that make the live
// device match desired. Unchanged peers are omitted so handshake state,
// learned endpoints, and transfer counters stay intact.
//
// The returned configs must be applied with ReplacePeers=false.
func peerSyncDiff(current []wgtypes.Peer, desired []*store.Peer) ([]wgtypes.PeerConfig, error) {
	live := make(map[wgtypes.Key]wgtypes.Peer, len(current))
	for _, p := range current {
		live[p.PublicKey] = p
	}

	want := make(map[wgtypes.Key]desiredPeer, len(desired))
	for _, p := range desired {
		if p == nil || !p.Enabled {
			continue
		}
		d, err := parseDesiredPeer(p)
		if err != nil {
			return nil, err
		}
		want[d.pub] = d
	}

	cfgs := make([]wgtypes.PeerConfig, 0, len(want)+len(live))
	for pub, d := range want {
		existing, ok := live[pub]
		if !ok {
			cfgs = append(cfgs, d.toAddConfig())
			continue
		}
		if cfg, changed := d.toUpdateConfig(existing); changed {
			cfgs = append(cfgs, cfg)
		}
	}
	for pub := range live {
		if _, ok := want[pub]; ok {
			continue
		}
		cfgs = append(cfgs, wgtypes.PeerConfig{PublicKey: pub, Remove: true})
	}

	sort.Slice(cfgs, func(i, j int) bool {
		if cfgs[i].Remove != cfgs[j].Remove {
			return cfgs[i].Remove
		}
		return cfgs[i].PublicKey.String() < cfgs[j].PublicKey.String()
	})
	return cfgs, nil
}

type desiredPeer struct {
	id  string
	pub wgtypes.Key
	psk *wgtypes.Key
	ips []net.IPNet
}

func parseDesiredPeer(p *store.Peer) (desiredPeer, error) {
	pub, err := wgtypes.ParseKey(p.WGPublicKey)
	if err != nil {
		return desiredPeer{}, fmt.Errorf("peer %s bad public key: %w", p.ID, err)
	}
	_, ipnet, err := net.ParseCIDR(p.WGAllowedIP)
	if err != nil {
		return desiredPeer{}, fmt.Errorf("peer %s bad allowed ip: %w", p.ID, err)
	}
	d := desiredPeer{id: p.ID, pub: pub, ips: []net.IPNet{*ipnet}}
	if p.WGPresharedKey != "" {
		psk, err := wgtypes.ParseKey(p.WGPresharedKey)
		if err != nil {
			return desiredPeer{}, fmt.Errorf("peer %s bad preshared key: %w", p.ID, err)
		}
		d.psk = &psk
	}
	return d, nil
}

func (d desiredPeer) toAddConfig() wgtypes.PeerConfig {
	cfg := wgtypes.PeerConfig{
		PublicKey:         d.pub,
		ReplaceAllowedIPs: true,
		AllowedIPs:        d.ips,
	}
	if d.psk != nil {
		cfg.PresharedKey = d.psk
	}
	return cfg
}

func (d desiredPeer) toUpdateConfig(existing wgtypes.Peer) (wgtypes.PeerConfig, bool) {
	cfg := wgtypes.PeerConfig{
		PublicKey:  d.pub,
		UpdateOnly: true,
	}
	changed := false
	if allowedIPsKey(existing.AllowedIPs) != allowedIPsKey(d.ips) {
		cfg.ReplaceAllowedIPs = true
		cfg.AllowedIPs = d.ips
		changed = true
	}
	wantPSK := wgtypes.Key{}
	if d.psk != nil {
		wantPSK = *d.psk
	}
	if existing.PresharedKey != wantPSK {
		cfg.PresharedKey = &wantPSK
		changed = true
	}
	return cfg, changed
}

func allowedIPsKey(ips []net.IPNet) string {
	parts := make([]string, len(ips))
	for i, n := range ips {
		parts[i] = n.String()
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
