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
	"sync"
	"sync/atomic"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/sentinel/policy"
)

// defaultShieldUpstreams are fast UDP resolvers for AdGuard on Shield nodes.
var defaultShieldUpstreams = []string{"1.1.1.1", "1.0.0.1"}

// SyncPayload mirrors gateway store.FirewallSyncPayload.
type SyncPayload struct {
	PolicyVersion *int64        `json:"policy_version,omitempty"`
	OrgID         string        `json:"org_id"`
	NodeID        string        `json:"node_id"`
	ServiceKind   string        `json:"service_kind"`
	Rules         snapshotRules `json:"rules"`
	Upstreams     []string      `json:"upstreams"`
	Licensed      bool          `json:"licensed"`
	ShieldAdmin   string        `json:"shield_admin_url,omitempty"`
}

// SyncRule is one gateway firewall rule.
type SyncRule = policy.Rule

// Distinguish an omitted "rules" field from the gateway's explicit null or [].
// Only an explicit empty set may clear Shield's custom rules.
type snapshotRules []SyncRule

func (rules *snapshotRules) UnmarshalJSON(raw []byte) error {
	var parsed []SyncRule
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	if parsed == nil {
		parsed = []SyncRule{}
	}
	*rules = parsed
	return nil
}

// SettingsStore holds Shield setup state across node restarts.
type SettingsStore interface {
	GetSetting(context.Context, string) (string, error)
	SetSetting(context.Context, string, string) error
}

// Client applies firewall operations for the active profile.
type Client struct {
	cfg      *config.Config
	settings SettingsStore
	shieldMu sync.Mutex
	http     *http.Client
	licensed atomic.Bool
}

// New constructs a Client.
func New(cfg *config.Config, settings SettingsStore) *Client {
	c := &Client{
		cfg:      cfg,
		settings: settings,
		http: &http.Client{
			Timeout: 10 * time.Second,
			// A redirect to the AdGuard login or setup page is not an API success.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	c.SetLicensed(true)
	return c
}

// SetLicensed updates whether DNS enforcement is allowed (Sentinel).
func (c *Client) SetLicensed(ok bool) { c.licensed.Store(ok) }

// Licensed reports current license state.
func (c *Client) Licensed() bool { return c.licensed.Load() }

// Sync applies a gateway snapshot and returns its version only after it succeeds.
func (c *Client) Sync(ctx context.Context, raw json.RawMessage) (*int64, error) {
	if !c.cfg.HasFirewallService() {
		return nil, fmt.Errorf("firewall service not configured")
	}
	var p SyncPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid sync args: %w", err)
	}
	if c.cfg.FirewallProvider == config.FirewallAdGuardHome && p.Rules == nil {
		return nil, fmt.Errorf("firewall snapshot is missing rules")
	}
	c.SetLicensed(p.Licensed)
	var err error
	switch c.cfg.FirewallProvider {
	case config.FirewallUnboundErebrus:
		err = c.syncSentinel(ctx, p)
	case config.FirewallAdGuardHome:
		err = c.syncShield(ctx, p)
	default:
		err = fmt.Errorf("unsupported firewall provider %q", c.cfg.FirewallProvider)
	}
	if err != nil {
		return nil, err
	}
	return p.PolicyVersion, nil
}

func (c *Client) syncSentinel(ctx context.Context, p SyncPayload) error {
	if !c.Licensed() {
		return fmt.Errorf("sentinel unlicensed")
	}
	pol := policy.Policy{
		OrgID: p.OrgID, NodeID: p.NodeID, Upstreams: p.Upstreams, Rules: p.Rules,
	}
	body, err := json.Marshal(pol)
	if err != nil {
		return err
	}
	licBody, _ := json.Marshal(map[string]bool{"licensed": c.Licensed()})
	_ = c.post(ctx, c.cfg.SentinelAPIURL+"/license/check", licBody)
	if !c.Licensed() {
		return fmt.Errorf("sentinel unlicensed")
	}
	if err := c.post(ctx, c.cfg.SentinelAPIURL+"/policy/apply", body); err != nil {
		return err
	}
	return c.post(ctx, c.cfg.SentinelAPIURL+"/reload", nil)
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
	c.shieldMu.Lock()
	defer c.shieldMu.Unlock()
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
// node via the install API, then swaps AdGuard's stock upstream for fast UDP
// resolvers. No-op for non-Shield or when no password is set. Best-effort: an
// already-configured AdGuard rejects the install call, which is ignored.
func (c *Client) ConfigureAdmin(ctx context.Context) error {
	c.shieldMu.Lock()
	defer c.shieldMu.Unlock()
	if c.cfg.FirewallProvider != config.FirewallAdGuardHome || c.cfg.ShieldAdminPassword == "" {
		return nil
	}
	base := strings.TrimRight(c.cfg.ShieldAdminURL, "/")
	if base == "" {
		return nil
	}
	_ = c.post(ctx, base+"/control/install/configure", c.configureBody(c.adminUser(), c.cfg.ShieldAdminPassword))
	return c.ensureShieldUpstreams(ctx)
}

func (c *Client) shieldUpstreams() []string {
	raw := strings.TrimSpace(c.cfg.ShieldUpstreamDNS)
	if raw == "" {
		return append([]string(nil), defaultShieldUpstreams...)
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if u := strings.TrimSpace(part); u != "" {
			out = append(out, u)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), defaultShieldUpstreams...)
	}
	return out
}

// ensureShieldUpstreams replaces AdGuard's stock upstream (Quad9 DoH) with fast
// UDP resolvers. DoH cache misses on long CNAME chains resolve slower than VPN
// clients wait for tunnel DNS, so the stock default degrades every peer on the
// node. Upstreams an operator changed by hand are left untouched.
func (c *Client) ensureShieldUpstreams(ctx context.Context) error {
	base := strings.TrimRight(c.cfg.ShieldAdminURL, "/")
	if base == "" {
		return nil
	}
	// A gateway policy can explicitly choose even the stock resolver.
	saved, err := c.settings.GetSetting(ctx, settingPreviousResolvers)
	if err != nil || saved != "" {
		return err
	}
	var dnsCfg map[string]any
	if err := c.getAuthJSON(ctx, base+"/control/dns_info", &dnsCfg); err != nil {
		return err
	}
	if dnsCfg == nil {
		return fmt.Errorf("empty AdGuard dns_info")
	}
	if !isStockUpstreams(dnsCfg["upstream_dns"]) {
		return nil
	}
	dnsCfg["upstream_dns"] = c.shieldUpstreams()
	return c.postAuthJSON(ctx, base+"/control/dns_config", dnsCfg)
}

// isStockUpstreams reports whether AdGuard still runs its shipped default
// upstream (Quad9 DoH) — i.e. nobody has picked resolvers for this node yet.
func isStockUpstreams(v any) bool {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return true
	}
	for _, item := range list {
		s, _ := item.(string)
		if !strings.Contains(s, "dns10.quad9.net") {
			return false
		}
	}
	return true
}

// SetAdminPassword applies a new admin password to AdGuard. AdGuard has no stable
// password-change API on a configured instance, so this attempts the install API;
// the gateway stays the source of truth for the stored value.
func (c *Client) SetAdminPassword(ctx context.Context, user, password string) error {
	c.shieldMu.Lock()
	defer c.shieldMu.Unlock()
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

func (c *Client) getAuthJSON(ctx context.Context, url string, result any) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("empty URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.adminUser(), c.cfg.ShieldAdminPassword)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %d %s", url, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("parse %s: %w", url, err)
	}
	return nil
}

func (c *Client) postAuthJSON(ctx context.Context, url string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.postAuth(ctx, url, body)
}

func (c *Client) postAuth(ctx context.Context, url string, body []byte) error {
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
	req.SetBasicAuth(c.adminUser(), c.cfg.ShieldAdminPassword)
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
