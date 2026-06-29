package config

import "testing"

func TestApplyProfileDefaultsShield(t *testing.T) {
	c := &Config{ErebrusProfile: ProfileShield, WGIPv4Subnet: "10.0.0.1/16", WGDNS: "1.1.1.1"}
	c.ApplyProfileDefaults()
	if c.FirewallProvider != FirewallAdGuardHome {
		t.Fatalf("provider = %q", c.FirewallProvider)
	}
	if c.FirewallDNSAddr != "adguardhome:53" {
		t.Fatalf("dns addr = %q", c.FirewallDNSAddr)
	}
	if c.WGDNS != "10.0.0.1" {
		t.Fatalf("wg dns = %q", c.WGDNS)
	}
}

func TestApplyProfileDefaultsSentinel(t *testing.T) {
	c := &Config{ErebrusProfile: ProfileSentinel, WGIPv4Subnet: "10.0.0.1/16"}
	c.ApplyProfileDefaults()
	if c.FirewallProvider != FirewallUnboundErebrus {
		t.Fatalf("provider = %q", c.FirewallProvider)
	}
	if c.SentinelAPIURL != "http://erebrus-sentinel:8788" {
		t.Fatalf("sentinel api = %q", c.SentinelAPIURL)
	}
}