// Package serviceagent reports attached firewall service health for Shield/Sentinel profiles.
package serviceagent

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
)

// Agent polls local firewall sidecars and exposes health for status/heartbeats.
type Agent struct {
	cfg *config.Config

	mu       sync.RWMutex
	snapshot map[string]string
}

// New constructs an Agent.
func New(cfg *config.Config) *Agent { return &Agent{cfg: cfg, snapshot: map[string]string{"vpn": "active"}} }

// Start runs periodic health checks until ctx is done.
func (a *Agent) Start(ctx context.Context) {
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
	status := a.probeAll()
	a.mu.Lock()
	a.snapshot = status
	a.mu.Unlock()
	slog.Info("service health", "profile", a.cfg.ErebrusProfile, "services", status)
}

func (a *Agent) probeAll() map[string]string {
	out := map[string]string{"vpn": "active"}
	if !a.cfg.HasFirewallService() {
		return out
	}
	switch a.cfg.FirewallProvider {
	case config.FirewallAdGuardHome:
		out["community_firewall"] = probeHTTP(a.cfg.ShieldAdminURL + "/")
	case config.FirewallUnboundErebrus:
		state := probeHTTP(a.cfg.SentinelAPIURL + "/health")
		if !a.cfg.SentinelLicensed {
			out["erebrus_firewall"] = "unlicensed"
		} else {
			out["erebrus_firewall"] = state
		}
	}
	return out
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
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make(map[string]string, len(a.snapshot))
	for k, v := range a.snapshot {
		out[k] = v
	}
	return out
}

// FirewallOK reports whether the configured firewall sidecar is healthy enough for readiness.
func (a *Agent) FirewallOK() (bool, string) {
	if !a.cfg.HasFirewallService() {
		return true, "not configured"
	}
	snap := a.Snapshot()
	switch a.cfg.FirewallProvider {
	case config.FirewallAdGuardHome:
		st := snap["community_firewall"]
		return st == "active", st
	case config.FirewallUnboundErebrus:
		if !a.cfg.SentinelLicensed {
			return false, "unlicensed"
		}
		st := snap["erebrus_firewall"]
		return st == "active", st
	default:
		return true, "unknown provider"
	}
}