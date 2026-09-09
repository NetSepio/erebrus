// Package netcidr holds shared IPv4/IPv6 subnet helpers for WireGuard addressing
// and derived NAT rules. Operator-configured prefixes are followed as-is.
package netcidr

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

const (
	minIPv4Prefix = 16
	maxIPv4Prefix = 28
	minIPv6Prefix = 64
	maxIPv6Prefix = 112
	defaultNATIF  = "eth0"
)

// ParseIPv4Subnet accepts a host CIDR such as "100.64.0.1/24" and returns the
// network (100.64.0.0/24). Prefix must be /16–/28.
func ParseIPv4Subnet(s string) (*net.IPNet, error) {
	ip, n, err := net.ParseCIDR(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("WG_IPv4_SUBNET: %w", err)
	}
	if ip.To4() == nil {
		return nil, fmt.Errorf("WG_IPv4_SUBNET must be an IPv4 CIDR")
	}
	ones, bits := n.Mask.Size()
	if bits != 32 || ones < minIPv4Prefix || ones > maxIPv4Prefix {
		return nil, fmt.Errorf("WG_IPv4_SUBNET prefix must be /%d to /%d", minIPv4Prefix, maxIPv4Prefix)
	}
	return n, nil
}

// ParseIPv6Subnet accepts a host CIDR such as "fd00:64:64::1/64" and returns
// the network. Prefix must be /64–/112.
func ParseIPv6Subnet(s string) (*net.IPNet, error) {
	ip, n, err := net.ParseCIDR(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("WG_IPv6_SUBNET: %w", err)
	}
	if ip.To4() != nil || ip.To16() == nil {
		return nil, fmt.Errorf("WG_IPv6_SUBNET must be an IPv6 CIDR")
	}
	ones, bits := n.Mask.Size()
	if bits != 128 || ones < minIPv6Prefix || ones > maxIPv6Prefix {
		return nil, fmt.Errorf("WG_IPv6_SUBNET prefix must be /%d to /%d", minIPv6Prefix, maxIPv6Prefix)
	}
	return n, nil
}

// HostCIDR formats the host address with the prefix length, e.g. 100.64.0.1/24.
func HostCIDR(s string) string {
	ip, n, err := net.ParseCIDR(strings.TrimSpace(s))
	if err != nil {
		return s
	}
	ones, _ := n.Mask.Size()
	return fmt.Sprintf("%s/%d", ip.String(), ones)
}

// HostOffset is the integer distance from the network address to ip.
func HostOffset(ip net.IP, n *net.IPNet) (uint64, error) {
	ip4 := ip.To4()
	if ip4 == nil || n == nil {
		return 0, fmt.Errorf("not IPv4")
	}
	net4 := n.IP.To4()
	if net4 == nil {
		net4 = n.IP.Mask(n.Mask).To4()
	}
	if net4 == nil {
		return 0, fmt.Errorf("not IPv4 subnet")
	}
	a := binary.BigEndian.Uint32(ip4)
	b := binary.BigEndian.Uint32(net4)
	if a < b {
		return 0, fmt.Errorf("address outside subnet")
	}
	return uint64(a - b), nil
}

// AddOffset returns network+offset as a 16-byte IP (IPv4-mapped or IPv6).
func AddOffset(network net.IP, offset uint64) net.IP {
	out := make(net.IP, 16)
	src := network.To16()
	if src == nil {
		return network
	}
	copy(out, src)
	carry := offset
	for i := 15; i >= 0 && carry > 0; i-- {
		sum := uint64(out[i]) + (carry & 0xff)
		out[i] = byte(sum)
		carry = (carry >> 8) + (sum >> 8)
	}
	return out
}

// SkipIPv4Host reports addresses that confuse some clients (.0 and .255).
func SkipIPv4Host(ip net.IP) bool {
	ip4 := ip.To4()
	return ip4 != nil && (ip4[3] == 0 || ip4[3] == 255)
}

// DeriveHooks builds wg-quick PostUp/PostDown from the configured subnets.
// outIface is the container uplink (eth0 on Compose). %i is the WG interface.
func DeriveHooks(v4Net, v6Net *net.IPNet, outIface string) (up, down string) {
	if outIface == "" {
		outIface = defaultNATIF
	}
	upParts, downParts := v4Hooks(v4Net, outIface)
	if v6Net != nil {
		u, d := v6Hooks(v6Net, outIface)
		upParts = append(upParts, u...)
		downParts = append(downParts, d...)
	}
	return strings.Join(upParts, "; "), strings.Join(downParts, "; ")
}

func v4Hooks(n *net.IPNet, outIface string) (up, down []string) {
	if n == nil {
		return nil, nil
	}
	s := n.String()
	return []string{
		fmt.Sprintf("iptables -A FORWARD -i %%i -s %s -j ACCEPT", s),
		"iptables -A FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT",
		fmt.Sprintf("iptables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE", s, outIface),
	}, []string{
		fmt.Sprintf("iptables -D FORWARD -i %%i -s %s -j ACCEPT", s),
		"iptables -D FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT",
		fmt.Sprintf("iptables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE", s, outIface),
	}
}

func v6Hooks(n *net.IPNet, outIface string) (up, down []string) {
	s := n.String()
	return []string{
		fmt.Sprintf("ip6tables -A FORWARD -i %%i -s %s -j ACCEPT", s),
		"ip6tables -A FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT",
		fmt.Sprintf("ip6tables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE", s, outIface),
	}, []string{
		fmt.Sprintf("ip6tables -D FORWARD -i %%i -s %s -j ACCEPT", s),
		"ip6tables -D FORWARD -o %i -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT",
		fmt.Sprintf("ip6tables -t nat -D POSTROUTING -s %s -o %s -j MASQUERADE", s, outIface),
	}
}
