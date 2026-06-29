// Package serviceagent reports attached firewall service health for Shield/Sentinel profiles.
package serviceagent

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
)

// Agent polls local firewall sidecars and logs health (gateway push deferred).
type Agent struct {
	cfg *config.Config
}

// New constructs an Agent.
func New(cfg *config.Config) *Agent { return &Agent{cfg: cfg} }

// Start runs periodic health checks until ctx is done.
func (a *Agent) Start(ctx context.Context) {
	if !a.cfg.HasFirewallService() {
		return
	}
	go a.loop(ctx)
}

func (a *Agent) loop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	a.check()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.check()
		}
	}
}

func (a *Agent) check() {
	status := map[string]string{"vpn": "active"}
	switch a.cfg.FirewallProvider {
	case config.FirewallAdGuardHome:
		status["shield"] = probeHTTP(a.cfg.ShieldAdminURL + "/")
	case config.FirewallUnboundErebrus:
		status["sentinel"] = probeHTTP(a.cfg.SentinelAPIURL + "/health")
	default:
		return
	}
	slog.Info("service health", "profile", a.cfg.ErebrusProfile, "services", status)
}

func probeHTTP(url string) string {
	url = strings.TrimSpace(url)
	if url == "" {
		return "unknown"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "error"
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "unreachable"
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 == 2 {
		return "active"
	}
	return "degraded"
}

// Snapshot returns the latest coarse service map for API/status extensions.
func (a *Agent) Snapshot() map[string]string {
	if !a.cfg.HasFirewallService() {
		return map[string]string{"vpn": "active"}
	}
	out := map[string]string{"vpn": "active"}
	switch a.cfg.FirewallProvider {
	case config.FirewallAdGuardHome:
		out["community_firewall"] = probeHTTP(a.cfg.ShieldAdminURL + "/")
	case config.FirewallUnboundErebrus:
		out["erebrus_firewall"] = probeHTTP(a.cfg.SentinelAPIURL + "/health")
	}
	return out
}