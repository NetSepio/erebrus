package wg

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/internal/store"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// DeviceStats is a coarse, live snapshot of the WireGuard interface.
type DeviceStats struct {
	RxBytes   int64 // cumulative bytes received from peers
	TxBytes   int64 // cumulative bytes sent to peers
	Connected int   // peers with a handshake in the last 3 minutes
}

// Controller abstracts the host's WireGuard plumbing so the Manager can be
// unit-tested with a fake. The real implementation uses wg-quick for the
// interface lifecycle (addresses + PostUp/Down rules) and wgctrl for live
// peer changes (no interface bounce, sessions survive).
type Controller interface {
	// BringUp (re)creates the interface from the rendered conf file.
	BringUp(iface, confPath string) error
	// SyncPeers replaces the live peer set on iface to match peers.
	SyncPeers(iface string, peers []*store.Peer) error
	// Stats reads live transfer counters and active-peer count from the device.
	Stats(iface string) (DeviceStats, error)
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

func (r *realController) Stats(iface string) (DeviceStats, error) {
	cl, err := wgctrl.New()
	if err != nil {
		return DeviceStats{}, err
	}
	defer cl.Close()
	d, err := cl.Device(iface)
	if err != nil {
		return DeviceStats{}, err
	}
	var st DeviceStats
	cutoff := time.Now().Add(-3 * time.Minute)
	for _, p := range d.Peers {
		st.RxBytes += p.ReceiveBytes
		st.TxBytes += p.TransmitBytes
		if !p.LastHandshakeTime.IsZero() && p.LastHandshakeTime.After(cutoff) {
			st.Connected++
		}
	}
	return st, nil
}
