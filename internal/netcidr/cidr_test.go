package netcidr

import (
	"net"
	"strings"
	"testing"
)

func TestParseIPv4Subnet(t *testing.T) {
	n, err := ParseIPv4Subnet("100.64.0.1/24")
	if err != nil {
		t.Fatal(err)
	}
	if n.String() != "100.64.0.0/24" {
		t.Fatalf("network = %s", n)
	}
	if _, err := ParseIPv4Subnet("fd00::1/64"); err == nil {
		t.Fatal("expected IPv4 error")
	}
	if _, err := ParseIPv4Subnet("10.0.0.1/8"); err == nil {
		t.Fatal("expected prefix error")
	}
}

func TestHostOffsetAndIPv6(t *testing.T) {
	_, v4, _ := net.ParseCIDR("100.64.0.1/24")
	off, err := HostOffset(net.ParseIP("100.64.0.7"), v4)
	if err != nil || off != 7 {
		t.Fatalf("offset = %d, %v", off, err)
	}
	_, v6, _ := net.ParseCIDR("fd00:64:64::1/64")
	got := AddOffset(v6.IP, off)
	if got.String() != "fd00:64:64::7" {
		t.Fatalf("ipv6 = %s", got)
	}
}

func TestDeriveHooksFollowsSubnet(t *testing.T) {
	v4, err := ParseIPv4Subnet("10.42.0.1/24")
	if err != nil {
		t.Fatal(err)
	}
	v6, err := ParseIPv6Subnet("fd00:64:64::1/64")
	if err != nil {
		t.Fatal(err)
	}
	up, down := DeriveHooks(v4, v6, "eth0")
	for _, s := range []string{up, down} {
		if !strings.Contains(s, "10.42.0.0/24") || strings.Contains(s, "10.0.0.0/16") {
			t.Fatalf("hooks must use operator subnet: %s", s)
		}
		if !strings.Contains(s, "fd00:64:64::/64") || !strings.Contains(s, "ip6tables") {
			t.Fatalf("hooks must include IPv6 NAT: %s", s)
		}
		if !strings.Contains(s, "conntrack") {
			t.Fatalf("hooks must restrict return traffic: %s", s)
		}
	}
}

func TestSkipIPv4Host(t *testing.T) {
	if !SkipIPv4Host(net.ParseIP("10.0.1.0")) || !SkipIPv4Host(net.ParseIP("10.0.1.255")) {
		t.Fatal("expected .0 and .255 skipped")
	}
	if SkipIPv4Host(net.ParseIP("10.0.1.2")) {
		t.Fatal("usable host skipped")
	}
}
