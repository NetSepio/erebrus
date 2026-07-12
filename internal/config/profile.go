package config

import (
	"fmt"
	"net"
	"strings"
)

// Deployment profiles (product/runtime).
const (
	ProfileStandard = "standard"
	ProfileShield   = "shield"
	ProfileSentinel = "sentinel"
)

// Firewall providers for attached services.
const (
	FirewallNone           = "none"
	FirewallAdGuardHome    = "adguard_home"
	FirewallUnboundErebrus = "unbound_erebrus"
)

// HasFirewallService reports whether a sidecar DNS firewall is configured.
func (c *Config) HasFirewallService() bool {
	p := strings.TrimSpace(c.FirewallProvider)
	return p != "" && p != FirewallNone
}

// ApplyProfileDefaults fills profile-specific env defaults after Load().
func (c *Config) ApplyProfileDefaults() {
	profile := strings.ToLower(strings.TrimSpace(c.ErebrusProfile))
	if profile == "" {
		profile = ProfileStandard
	}
	c.ErebrusProfile = profile

	tunnelDNS := tunnelGatewayIP(c.WGIPv4Subnet, c.PrivateDNSAddr)

	switch profile {
	case ProfileShield:
		if c.FirewallProvider == "" {
			c.FirewallProvider = FirewallAdGuardHome
		}
		if c.FirewallDNSAddr == "" {
			c.FirewallDNSAddr = "adguardhome:53"
		}
		if c.ShieldAdminURL == "" {
			c.ShieldAdminURL = "http://adguardhome:3000"
		}
		if c.WGDNS == "" || c.WGDNS == "1.1.1.1" {
			c.WGDNS = tunnelDNS
		}
		if c.PrivateDNSAddr == "" {
			c.PrivateDNSAddr = tunnelDNS
		}
	case ProfileSentinel:
		if c.FirewallProvider == "" {
			c.FirewallProvider = FirewallUnboundErebrus
		}
		if c.FirewallDNSAddr == "" {
			c.FirewallDNSAddr = "erebrus-sentinel:53"
		}
		if c.SentinelAPIURL == "" {
			c.SentinelAPIURL = "http://erebrus-sentinel:8788"
		}
		if c.SentinelImage == "" {
			c.SentinelImage = "ghcr.io/netsepio/erebrus-sentinel:latest"
		}
		if c.WGDNS == "" || c.WGDNS == "1.1.1.1" {
			c.WGDNS = tunnelDNS
		}
		if c.PrivateDNSAddr == "" {
			c.PrivateDNSAddr = tunnelDNS
		}
	case ProfileStandard:
		if c.FirewallProvider == "" {
			c.FirewallProvider = FirewallNone
		}
	}
}

func tunnelGatewayIP(subnet, override string) string {
	if o := strings.TrimSpace(override); o != "" {
		return strings.Split(o, ":")[0]
	}
	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return "10.0.0.1"
	}
	return ip.String()
}

// ValidateProfile returns an error for unknown profile or provider values.
func (c *Config) ValidateProfile() error {
	switch c.ErebrusProfile {
	case ProfileStandard, ProfileShield, ProfileSentinel:
	default:
		return fmt.Errorf("invalid EREBRUS_PROFILE %q (expected standard, shield, or sentinel)", c.ErebrusProfile)
	}
	switch c.FirewallProvider {
	case FirewallNone, FirewallAdGuardHome, FirewallUnboundErebrus:
	default:
		return fmt.Errorf("invalid FIREWALL_PROVIDER %q", c.FirewallProvider)
	}
	if c.HasFirewallService() && strings.TrimSpace(c.FirewallDNSAddr) == "" {
		return fmt.Errorf("FIREWALL_DNS_ADDR is required when FIREWALL_PROVIDER=%s", c.FirewallProvider)
	}
	return nil
}
