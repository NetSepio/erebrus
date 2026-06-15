// Package transport models the stealth transport ladder: preferred order,
// probing, scoring, and selection. Clients use this to pick the best working
// path when WireGuard UDP is blocked.
package transport

import "sort"

// Kind identifies a transport in the ladder.
type Kind string

const (
	KindDirectWG     Kind = "direct_wireguard_udp"
	KindHysteria2    Kind = "hysteria2_quic_udp"
	KindVLESSReality Kind = "vless_reality_tcp"
	KindWebSocketTLS Kind = "websocket_tls_tcp"
	KindHTTPSConnect Kind = "https_connect_tcp"
)

// DefaultLadder is the preferred transport order for v2.1.
var DefaultLadder = []Kind{
	KindDirectWG,
	KindHysteria2,
	KindVLESSReality,
	KindWebSocketTLS,
	KindHTTPSConnect,
}

// ImplementedLadder returns transports the node can offer today.
func ImplementedLadder(stealthEnabled bool) []Kind {
	if !stealthEnabled {
		return []Kind{KindDirectWG}
	}
	return []Kind{KindDirectWG, KindHysteria2, KindVLESSReality}
}

// ProbeResult is the outcome of probing one transport.
type ProbeResult struct {
	Kind          Kind
	Success       bool
	LatencyMs     int
	PacketLossPct float64
	Error         string
	Score         int
}

// Score computes the transport ranking per the v2 upgrade plan.
func Score(r ProbeResult, survived60s bool, wasLastSuccess bool) int {
	if !r.Success {
		return 0
	}
	s := 100 - r.LatencyMs/10 - int(r.PacketLossPct*2)
	if survived60s {
		s += 20
	}
	if wasLastSuccess {
		s += 10
	}
	if s < 0 {
		s = 0
	}
	r.Score = s
	return s
}

// SelectBest picks the highest-scoring successful probe.
func SelectBest(results []ProbeResult) (ProbeResult, bool) {
	var best *ProbeResult
	var bestScore int
	for i := range results {
		if !results[i].Success {
			continue
		}
		s := Score(results[i], false, false)
		if best == nil || s > bestScore {
			cp := results[i]
			cp.Score = s
			best = &cp
			bestScore = s
		}
	}
	if best == nil {
		return ProbeResult{}, false
	}
	return *best, true
}

// SortByLadder orders kinds according to the preferred ladder.
func SortByLadder(kinds []Kind) []Kind {
	order := map[Kind]int{}
	for i, k := range DefaultLadder {
		order[k] = i
	}
	out := append([]Kind(nil), kinds...)
	sort.Slice(out, func(i, j int) bool {
		oi, oki := order[out[i]]
		oj, okj := order[out[j]]
		if !oki {
			oi = len(DefaultLadder)
		}
		if !okj {
			oj = len(DefaultLadder)
		}
		return oi < oj
	})
	return out
}

// IsImplemented reports whether a kind has a node-side implementation.
func IsImplemented(k Kind, stealthEnabled bool) bool {
	for _, x := range ImplementedLadder(stealthEnabled) {
		if x == k {
			return true
		}
	}
	return false
}
