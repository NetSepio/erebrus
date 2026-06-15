// Package probe defines the transport probing interface. Concrete network
// probes are added incrementally; the node ships with a local evaluator that
// scores configured listeners.
package probe

import (
	"context"

	"github.com/NetSepio/erebrus/internal/transport"
)

// Prober checks transport reachability from the node's perspective.
type Prober interface {
	Probe(ctx context.Context, kinds []transport.Kind) []transport.ProbeResult
}

// LocalProber assumes transports are available when the node has them configured.
type LocalProber struct {
	StealthEnabled bool
	WGPort         int
	VLESSPort      int
	Hysteria2Port  int
}

// Probe returns synthetic success for implemented local listeners.
func (p *LocalProber) Probe(_ context.Context, kinds []transport.Kind) []transport.ProbeResult {
	out := make([]transport.ProbeResult, 0, len(kinds))
	for _, k := range kinds {
		r := transport.ProbeResult{Kind: k, LatencyMs: 1}
		switch k {
		case transport.KindDirectWG:
			if p.WGPort > 0 {
				r.Success = true
			} else {
				r.Error = "wireguard port not configured"
			}
		case transport.KindHysteria2:
			if p.StealthEnabled && p.Hysteria2Port > 0 {
				r.Success = true
			} else {
				r.Error = "hysteria2 not enabled"
			}
		case transport.KindVLESSReality:
			if p.StealthEnabled && p.VLESSPort > 0 {
				r.Success = true
			} else {
				r.Error = "vless+reality not enabled"
			}
		default:
			r.Error = "not implemented"
		}
		transport.Score(r, false, false)
		out = append(out, r)
	}
	return out
}

// Select runs the ladder and returns the best transport.
func Select(ctx context.Context, prober Prober, stealthEnabled bool) (transport.ProbeResult, bool) {
	kinds := transport.ImplementedLadder(stealthEnabled)
	results := prober.Probe(ctx, kinds)
	return transport.SelectBest(results)
}
