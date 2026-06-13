// Package telemetry sets up structured logging (slog JSON) and Prometheus
// metrics for the node.
package telemetry

import (
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// InitLogger installs a JSON slog logger as the default. debug=true lowers the
// level to Debug.
func InitLogger(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

// Metrics holds the node's Prometheus collectors.
type Metrics struct {
	WGPeers           prometheus.Gauge
	ProxySessions     prometheus.Gauge
	SingboxRebuilds   prometheus.Counter
	PeerProvisioned   prometheus.Counter
	PeerDeprovisioned prometheus.Counter
}

// NewMetrics registers and returns the node metrics on the default registry.
func NewMetrics() *Metrics {
	return &Metrics{
		WGPeers: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "erebrus_wg_peers", Help: "Number of configured WireGuard peers.",
		}),
		ProxySessions: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "erebrus_proxy_sessions", Help: "Active sing-box proxy sessions.",
		}),
		SingboxRebuilds: promauto.NewCounter(prometheus.CounterOpts{
			Name: "erebrus_singbox_rebuilds_total", Help: "sing-box configuration rebuilds.",
		}),
		PeerProvisioned: promauto.NewCounter(prometheus.CounterOpts{
			Name: "erebrus_peer_provisioned_total", Help: "Peers provisioned.",
		}),
		PeerDeprovisioned: promauto.NewCounter(prometheus.CounterOpts{
			Name: "erebrus_peer_deprovisioned_total", Help: "Peers removed.",
		}),
	}
}
