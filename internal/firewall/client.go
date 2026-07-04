// Package firewall applies gateway-managed policy to local Shield/Sentinel sidecars.
package firewall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/sentinel/policy"
)

// SyncPayload mirrors gateway store.FirewallSyncPayload.
type SyncPayload struct {
	OrgID       string       `json:"org_id"`
	NodeID      string       `json:"node_id"`
	ServiceKind string       `json:"service_kind"`
	Rules       []SyncRule   `json:"rules"`
	Upstreams   []string     `json:"upstreams"`
	Licensed    bool         `json:"licensed"`
	ShieldAdmin string       `json:"shield_admin_url,omitempty"`
}

// SyncRule is one gateway firewall rule.
type SyncRule struct {
	RuleType string `json:"rule_type"`
	Target   string `json:"target"`
	Action   string `json:"action"`
	Value    string `json:"value,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// Client applies firewall operations for the active profile.
type Client struct {
	cfg    *config.Config
	http   *http.Client
	licensed bool
}

// New constructs a Client.
func New(cfg *config.Config) *Client {
	return &Client{cfg: cfg, licensed: true, http: &http.Client{Timeout: 10 * time.Second}}
}

// SetLicensed updates whether DNS enforcement is allowed (Sentinel).
func (c *Client) SetLicensed(ok bool) { c.licensed = ok }

// Licensed reports current license state.
func (c *Client) Licensed() bool { return c.licensed }

// Sync applies a gateway policy payload.
func (c *Client) Sync(ctx context.Context, raw json.RawMessage) error {
	if !c.cfg.HasFirewallService() {
		return fmt.Errorf("firewall service not configured")
	}
	var p SyncPayload
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("invalid sync args: %w", err)
		}
	}
	c.SetLicensed(p.Licensed)
	switch c.cfg.FirewallProvider {
	case config.FirewallUnboundErebrus:
		return c.syncSentinel(ctx, p)
	case config.FirewallAdGuardHome:
		return c.syncShield(ctx, p)
	default:
		return fmt.Errorf("unsupported firewall provider %q", c.cfg.FirewallProvider)
	}
}

func (c *Client) syncSentinel(ctx context.Context, p SyncPayload) error {
	if !c.licensed {
		return fmt.Errorf("sentinel unlicensed")
	}
	pol := policy.Policy{
		OrgID: p.OrgID, NodeID: p.NodeID, Upstreams: p.Upstreams,
	}
	for _, r := range p.Rules {
		pol.Rules = append(pol.Rules, policy.Rule{
			RuleType: r.RuleType, Target: r.Target, Action: r.Action, Value: r.Value, Enabled: r.Enabled,
		})
	}
	body, err := json.Marshal(pol)
	if err != nil {
		return err
	}
	licBody, _ := json.Marshal(map[string]bool{"licensed": c.licensed})
	_ = c.post(ctx, c.cfg.SentinelAPIURL+"/license/check", licBody)
	if !c.licensed {
		return fmt.Errorf("sentinel unlicensed")
	}
	if err := c.post(ctx, c.cfg.SentinelAPIURL+"/policy/apply", body); err != nil {
		return err
	}
	return c.post(ctx, c.cfg.SentinelAPIURL+"/reload", nil)
}

func (c *Client) syncShield(ctx context.Context, _ SyncPayload) error {
	base := strings.TrimRight(c.cfg.ShieldAdminURL, "/")
	if base == "" {
		return nil
	}
	return c.post(ctx, base+"/control/cache_clear", []byte(`{}`))
}

// Restart reloads the local firewall sidecar.
func (c *Client) Restart(ctx context.Context) error {
	switch c.cfg.FirewallProvider {
	case config.FirewallUnboundErebrus:
		return c.post(ctx, c.cfg.SentinelAPIURL+"/reload", nil)
	case config.FirewallAdGuardHome:
		base := strings.TrimRight(c.cfg.ShieldAdminURL, "/")
		if base == "" {
			return nil
		}
		return c.post(ctx, base+"/control/restart", []byte(`{}`))
	default:
		return fmt.Errorf("firewall not configured")
	}
}

func (c *Client) adminUser() string {
	if c.cfg.ShieldAdminUser != "" {
		return c.cfg.ShieldAdminUser
	}
	return "admin"
}

// AdminCredentials returns the configured Shield (AdGuard) admin login.
func (c *Client) AdminCredentials() (user, password, url string) {
	return c.adminUser(), c.cfg.ShieldAdminPassword, strings.TrimRight(c.cfg.ShieldAdminURL, "/")
}

// installConfigure body for AdGuard's initial-setup API.
func (c *Client) configureBody(user, password string) []byte {
	body, _ := json.Marshal(map[string]any{
		"web":      map[string]any{"ip": "0.0.0.0", "port": 3000},
		"dns":      map[string]any{"ip": "0.0.0.0", "port": 53},
		"username": user,
		"password": password,
	})
	return body
}

// ConfigureAdmin sets the AdGuard admin credentials on a freshly-installed Shield
// node via the install API. No-op for non-Shield or when no password is set.
// Best-effort: an already-configured AdGuard rejects it, which is ignored.
func (c *Client) ConfigureAdmin(ctx context.Context) error {
	if c.cfg.FirewallProvider != config.FirewallAdGuardHome || c.cfg.ShieldAdminPassword == "" {
		return nil
	}
	base := strings.TrimRight(c.cfg.ShieldAdminURL, "/")
	if base == "" {
		return nil
	}
	_ = c.post(ctx, base+"/control/install/configure", c.configureBody(c.adminUser(), c.cfg.ShieldAdminPassword))
	return nil
}

// SetAdminPassword applies a new admin password to AdGuard. AdGuard has no stable
// password-change API on a configured instance, so this attempts the install API;
// the gateway stays the source of truth for the stored value.
func (c *Client) SetAdminPassword(ctx context.Context, user, password string) error {
	if c.cfg.FirewallProvider != config.FirewallAdGuardHome || password == "" {
		return nil
	}
	base := strings.TrimRight(c.cfg.ShieldAdminURL, "/")
	if base == "" {
		return nil
	}
	if user == "" {
		user = c.adminUser()
	}
	c.cfg.ShieldAdminUser = user
	c.cfg.ShieldAdminPassword = password
	return c.post(ctx, base+"/control/install/configure", c.configureBody(user, password))
}

// ResetCredentials clears Shield admin credentials reference (AdGuard re-setup).
func (c *Client) ResetCredentials(ctx context.Context) error {
	if c.cfg.FirewallProvider != config.FirewallAdGuardHome {
		return nil
	}
	base := strings.TrimRight(c.cfg.ShieldAdminURL, "/")
	if base == "" {
		return nil
	}
	// Best-effort: clear DNS cache; operator re-opens AdGuard setup UI.
	_ = c.post(ctx, base+"/control/cache_clear", []byte(`{}`))
	return nil
}

// CheckLicense queries Sentinel license state.
func (c *Client) CheckLicense(ctx context.Context) (bool, error) {
	if c.cfg.FirewallProvider != config.FirewallUnboundErebrus {
		return true, nil
	}
	base := strings.TrimRight(c.cfg.SentinelAPIURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/license/check", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 != 2 {
		return false, fmt.Errorf("license check: %d", resp.StatusCode)
	}
	var out struct {
		Licensed bool `json:"licensed"`
	}
	_ = json.Unmarshal(raw, &out)
	c.SetLicensed(out.Licensed)
	return out.Licensed, nil
}

func (c *Client) post(ctx context.Context, url string, body []byte) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("empty URL")
	}
	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %d %s", url, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}