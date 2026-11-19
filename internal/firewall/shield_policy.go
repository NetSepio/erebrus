package firewall

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/NetSepio/erebrus/internal/sentinel/policy"
)

const settingPreviousResolvers = "shield_pre_policy_upstreams"

type shieldRewrite struct {
	Domain string `json:"domain"`
	Answer string `json:"answer"`
}

func (c *Client) syncShield(ctx context.Context, p SyncPayload) error {
	c.shieldMu.Lock()
	defer c.shieldMu.Unlock()
	base := strings.TrimRight(c.cfg.ShieldAdminURL, "/")
	if base == "" {
		return fmt.Errorf("Shield admin URL is missing")
	}
	// Validate rule text before it can enter AdGuard's filtering language.
	// AdGuard accepts resolver URLs that the Unbound config renderer cannot use.
	if err := policy.Validate(policy.Policy{Rules: p.Rules}); err != nil {
		return err
	}
	if len(p.Upstreams) > 32 {
		return fmt.Errorf("policy exceeds upstream limit")
	}

	filters := make([]string, 0, len(p.Rules))
	rewrites := make(map[shieldRewrite]bool)
	for _, rule := range p.Rules {
		if !rule.Enabled {
			continue
		}
		target := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rule.Target), "."))
		switch rule.NormalizedAction() {
		case "block":
			filters = append(filters, "||"+strings.TrimPrefix(target, "*.")+"^")
		case "allow":
			filters = append(filters, "@@||"+target+"^")
		case "rewrite":
			addr, err := netip.ParseAddr(strings.TrimSpace(rule.Value))
			if err != nil {
				return err
			}
			rewrites[shieldRewrite{Domain: target, Answer: addr.Unmap().String()}] = true
		default:
			return fmt.Errorf("unsupported Shield rule action %q", rule.Action)
		}
	}
	if err := c.syncShieldResolvers(ctx, base, p.Upstreams); err != nil {
		return err
	}
	if err := c.postAuthJSON(ctx, base+"/control/filtering/set_rules", map[string][]string{"rules": filters}); err != nil {
		return err
	}
	if err := c.syncShieldRewrites(ctx, base, rewrites); err != nil {
		return err
	}
	return c.postAuth(ctx, base+"/control/cache_clear", []byte(`{}`))
}

// Diff against AdGuard so a retry after a partial update also converges.
func (c *Client) syncShieldRewrites(ctx context.Context, base string, wanted map[shieldRewrite]bool) error {
	var current []struct {
		shieldRewrite
		Enabled *bool `json:"enabled"`
	}
	if err := c.getAuthJSON(ctx, base+"/control/rewrite/list", &current); err != nil {
		return err
	}
	// AdGuard deletes every copy of a (domain, answer) pair at once.
	// Replace duplicate or disabled entries before adding missing ones.
	active := make(map[shieldRewrite]bool, len(current))
	for _, entry := range current {
		_, seen := active[entry.shieldRewrite]
		active[entry.shieldRewrite] = !seen && (entry.Enabled == nil || *entry.Enabled)
	}
	for entry, enabled := range active {
		if enabled && wanted[entry] {
			delete(wanted, entry)
			continue
		}
		if err := c.postAuthJSON(ctx, base+"/control/rewrite/delete", entry); err != nil {
			return err
		}
	}
	for entry := range wanted {
		if err := c.postAuthJSON(ctx, base+"/control/rewrite/add", entry); err != nil {
			return err
		}
	}
	return nil
}

// Save the operator's resolvers before the first override. Removing the last
// gateway resolver rule restores those resolvers, even across node restarts.
func (c *Client) syncShieldResolvers(ctx context.Context, base string, wanted []string) error {
	saved, err := c.settings.GetSetting(ctx, settingPreviousResolvers)
	if err != nil {
		return err
	}
	if len(wanted) == 0 && saved == "" {
		return nil
	}

	var current struct {
		Upstreams []string `json:"upstream_dns"`
	}
	if err := c.getAuthJSON(ctx, base+"/control/dns_info", &current); err != nil {
		return err
	}
	restore := len(wanted) == 0
	if restore {
		if err := json.Unmarshal([]byte(saved), &wanted); err != nil {
			return fmt.Errorf("read saved Shield upstreams: %w", err)
		}
	} else if saved == "" {
		previous := current.Upstreams
		if len(previous) == 0 {
			previous = c.shieldUpstreams()
		}
		snapshot, err := json.Marshal(previous)
		if err != nil {
			return err
		}
		if err := c.settings.SetSetting(ctx, settingPreviousResolvers, string(snapshot)); err != nil {
			return err
		}
	}
	if !slices.Equal(current.Upstreams, wanted) {
		if err := c.postAuthJSON(ctx, base+"/control/dns_config", map[string][]string{"upstream_dns": wanted}); err != nil {
			return err
		}
	}
	if restore {
		return c.settings.SetSetting(ctx, settingPreviousResolvers, "")
	}
	return nil
}
