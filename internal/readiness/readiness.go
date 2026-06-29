// Package readiness evaluates whether a node is correctly configured and operational.
package readiness

import (
	"fmt"
	"strings"

	"github.com/NetSepio/erebrus/internal/config"
)

// Check is one readiness predicate.
type Check struct {
	ID       string `json:"id"`
	OK       bool   `json:"ok"`
	Detail   string `json:"detail,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

// Report is the aggregate readiness result exposed on /api/v2/status.
type Report struct {
	OK       bool     `json:"ok"`
	Checks   []Check  `json:"checks"`
	Warnings []string `json:"warnings,omitempty"`
}

// Input carries live signals the evaluator cannot infer from config alone.
type Input struct {
	Cfg                *config.Config
	IdentityConfigured bool
	GatewayRegistered  bool
	GatewayConnected   bool
	WireGuardOK        bool
	StealthListening   bool
	FirewallOK         bool
	FirewallDetail     string
}

// Evaluate builds a readiness report from config and runtime signals.
func Evaluate(in Input) Report {
	cfg := in.Cfg
	if cfg == nil {
		return Report{Checks: []Check{{ID: "config", OK: false, Detail: "configuration not loaded"}}}
	}

	checks := []Check{
		{
			ID: "identity",
			OK: in.IdentityConfigured && cfg.Mnemonic != "",
			Detail: identityDetail(in.IdentityConfigured, cfg.Mnemonic != ""),
		},
		{
			ID:     "public_address",
			OK:     cfg.WGEndpointHost != "",
			Detail: cfg.WGEndpointHost,
		},
		apiKeyCheck(cfg),
		{
			ID:     "wireguard",
			OK:     in.WireGuardOK,
			Detail: wireguardDetail(in.WireGuardOK),
		},
	}
	checks = append(checks, stealthCheck(cfg, in.StealthListening))
	checks = append(checks, firewallCheck(cfg, in.FirewallOK, in.FirewallDetail))
	checks = append(checks, controlPlaneCheck(cfg, in.GatewayRegistered, in.GatewayConnected))

	warnings := append([]string{}, cfg.Mode.Warnings...)

	ok := true
	for _, c := range checks {
		if c.Optional {
			continue
		}
		if !c.OK {
			ok = false
			break
		}
	}

	return Report{OK: ok, Checks: checks, Warnings: warnings}
}

func identityDetail(configured, hasMnemonic bool) string {
	if configured && hasMnemonic {
		return "node identity configured"
	}
	if !hasMnemonic {
		return "node identity (recovery phrase) not set"
	}
	return "node identity pending"
}

func apiKeyCheck(cfg *config.Config) Check {
	if cfg.RunType == "debug" && cfg.EffectiveNodeKey() == "" {
		return Check{
			ID:       "node_api_key",
			OK:       true,
			Optional: true,
			Detail:   "not set — peer API open in debug mode",
		}
	}
	return Check{
		ID:     "node_api_key",
		OK:     cfg.EffectiveNodeKey() != "",
		Detail: apiKeyDetail(cfg.EffectiveNodeKey() != ""),
	}
}

func apiKeyDetail(ok bool) string {
	if ok {
		return "configured"
	}
	return "required in release mode"
}

func wireguardDetail(ok bool) string {
	if ok {
		return "interface ready"
	}
	return "interface not up — check NET_ADMIN and wireguard-tools"
}

func stealthCheck(cfg *config.Config, listening bool) Check {
	if !cfg.EnableStealth {
		return Check{ID: "stealth", OK: true, Optional: true, Detail: "disabled"}
	}
	return Check{
		ID:     "stealth",
		OK:     listening,
		Detail: stealthDetail(listening),
	}
}

func stealthDetail(listening bool) string {
	if listening {
		return "vless-reality and hysteria2 listening"
	}
	return "carriers enabled but not listening"
}

func firewallCheck(cfg *config.Config, ok bool, detail string) Check {
	if !cfg.HasFirewallService() {
		return Check{ID: "firewall", OK: true, Optional: true, Detail: "not configured"}
	}
	if detail == "" {
		detail = "sidecar healthy"
	}
	if !ok {
		return Check{ID: "firewall", OK: false, Detail: detail}
	}
	return Check{ID: "firewall", OK: true, Detail: detail}
}

func controlPlaneCheck(cfg *config.Config, registered, connected bool) Check {
	if !cfg.GatewayEnabled() {
		return Check{
			ID:       "control_plane",
			OK:       true,
			Optional: true,
			Detail:   "gateway URL not configured",
		}
	}
	if !registered {
		return Check{
			ID:     "control_plane",
			OK:     false,
			Detail: "not registered with gateway",
		}
	}
	if connected {
		return Check{
			ID:     "control_plane",
			OK:     true,
			Detail: "registered, control channel connected",
		}
	}
	return Check{
		ID:     "control_plane",
		OK:     true,
		Detail: "registered, control channel reconnecting",
	}
}

// Preboot evaluates config-only checks before the node process is running.
func Preboot(cfg *config.Config) Report {
	if cfg == nil {
		return Report{Checks: []Check{{ID: "config", OK: false, Detail: "configuration not loaded"}}}
	}
	checks := []Check{
		{ID: "identity", OK: cfg.Mnemonic != "", Detail: identityDetail(cfg.Mnemonic != "", cfg.Mnemonic != "")},
		{ID: "public_address", OK: cfg.WGEndpointHost != "", Detail: cfg.WGEndpointHost},
		apiKeyCheck(cfg),
		{ID: "wireguard", OK: true, Optional: true, Detail: "checked after node start"},
	}
	if cfg.EnableStealth {
		checks = append(checks, Check{ID: "stealth", OK: true, Optional: true, Detail: "checked after node start"})
	}
	checks = append(checks, Check{ID: "control_plane", OK: true, Optional: true, Detail: "checked after node start"})

	ok := true
	for _, c := range checks {
		if !c.Optional && !c.OK {
			ok = false
			break
		}
	}
	return Report{OK: ok, Checks: checks, Warnings: append([]string{}, cfg.Mode.Warnings...)}
}

// AccessModeLabel returns the access mode name for display (Private, Shared, Public).
func AccessModeLabel(mode config.RuntimeMode) string {
	switch mode {
	case config.ModePrivate:
		return "Private"
	case config.ModeShared:
		return "Shared"
	case config.ModePublic:
		return "Public"
	default:
		return string(mode)
	}
}

// AccessModeHint is a one-line explanation shown in docs or expanded UI.
func AccessModeHint(mode config.RuntimeMode) string {
	switch mode {
	case config.ModePrivate:
		return "Only your own devices can use this node."
	case config.ModeShared:
		return "Only wallets you invite can connect."
	case config.ModePublic:
		return "Listed on the network for users to connect."
	default:
		return ""
	}
}

// RegionLabel turns an ISO 3166-1 alpha-2 code (or custom REGION value) into a
// friendly label. Custom values like "EU-WEST" pass through unchanged.
func RegionLabel(code string) string {
	code = strings.TrimSpace(code)
	if code == "" || strings.EqualFold(code, "unknown") {
		return "Not set"
	}
	if name, ok := regionNames[strings.ToUpper(code)]; ok {
		return name
	}
	// Custom operator-defined region (not a 2-letter country code).
	if len(code) > 2 || strings.ContainsAny(code, "-_") {
		return code
	}
	return code + " (country code)"
}

// ZoneLabel turns a ZONE env value into a dashboard-friendly label.
// Common US values: east → US East, west → US West. Unknown values pass through.
func ZoneLabel(zone string) string {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return ""
	}
	switch strings.ToLower(strings.ReplaceAll(zone, "_", "-")) {
	case "east", "us-east", "useast":
		return "US East"
	case "west", "us-west", "uswest":
		return "US West"
	case "central", "us-central", "uscentral":
		return "US Central"
	default:
		return zone
	}
}

// PublicAPIURL returns the URL operators should allow for gateway provisioning.
func PublicAPIURL(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.PublicAPIBaseURL()
}

// SummaryLine returns one line suitable for CLI output.
func SummaryLine(r Report) string {
	if r.OK {
		return "ready"
	}
	for _, c := range r.Checks {
		if !c.Optional && !c.OK {
			return fmt.Sprintf("not ready: %s", c.ID)
		}
	}
	return "not ready"
}