package wg

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/NetSepio/erebrus/internal/store"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Controller abstracts the host's WireGuard plumbing so the Manager can be
// unit-tested with a fake. The real implementation uses wg-quick for the
// interface lifecycle (addresses + PostUp/Down rules) and wgctrl for live
// peer changes (no interface bounce, sessions survive).
type Controller interface {
	// BringUp (re)creates the interface from the rendered conf file.
	BringUp(iface, confPath string) error
	// SyncPeers replaces the live peer set on iface to match peers.
	SyncPeers(iface string, peers []*store.Peer) error
}

// realController talks to the kernel via wg-quick and wgctrl.
type realController struct{}

// NewController returns the production WireGuard controller.
func NewController() Controller { return &realController{} }

func (r *realController) BringUp(iface, confPath string) error {
	// Idempotent: tear down a stale interface first, ignore its error.
	_ = exec.Command("wg-quick", "down", confPath).Run()
	out, err := exec.Command("wg-quick", "up", confPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick up: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r *realController) SyncPeers(iface string, peers []*store.Peer) error {
	cl, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer cl.Close()

	cfgs := make([]wgtypes.PeerConfig, 0, len(peers))
	for _, p := range peers {
		if !p.Enabled {
			continue
		}
		pub, err := wgtypes.ParseKey(p.WGPublicKey)
		if err != nil {
			return fmt.Errorf("peer %s bad public key: %w", p.ID, err)
		}
		pc := wgtypes.PeerConfig{
			PublicKey:         pub,
			ReplaceAllowedIPs: true,
		}
		if p.WGPresharedKey != "" {
			psk, err := wgtypes.ParseKey(p.WGPresharedKey)
			if err != nil {
				return fmt.Errorf("peer %s bad preshared key: %w", p.ID, err)
			}
			pc.PresharedKey = &psk
		}
		_, ipnet, err := net.ParseCIDR(p.WGAllowedIP)
		if err != nil {
			return fmt.Errorf("peer %s bad allowed ip: %w", p.ID, err)
		}
		pc.AllowedIPs = []net.IPNet{*ipnet}
		cfgs = append(cfgs, pc)
	}

	return cl.ConfigureDevice(iface, wgtypes.Config{
		ReplacePeers: true,
		Peers:        cfgs,
	})
}
