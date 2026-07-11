// Package initcfg writes the internal operator env file for bare-metal installs.
package initcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
)

// Options is the input for generating an env file.
type Options struct {
	AccessMode           config.RuntimeMode
	NetworkProfile       config.NetworkProfile
	NodeName             string
	Region               string
	Zone                 string
	Mnemonic             string
	NodeAPIToken         string
	GatewayURL           string
	PublicAddress        string
	HTTPPort             string
	EnableStealth        bool
	StealthTCPPort       string
	StealthUDPPort       string
	EnableAppHosting     bool
	AppWildcardDomain    string
	PublicDomain         string
	WildcardDomain       string
	PublicGatewayEnabled bool
	StateDir             string
	DefaultIface         string
}

// DefaultEnvPath is the standard bare-metal env file location.
const DefaultEnvPath = "/etc/erebrus/erebrus.env"

// ApplyModeDefaults sets ports and profiles from access mode when unset.
func ApplyModeDefaults(o *Options) {
	if o.AccessMode == "" {
		o.AccessMode = config.ModePublic
	}
	if o.NetworkProfile == "" {
		if o.AccessMode == config.ModePublic {
			o.NetworkProfile = config.NetworkHostNetwork
		} else {
			o.NetworkProfile = config.NetworkBridge
		}
	}
	if o.StealthTCPPort == "" {
		if o.AccessMode == config.ModePublic {
			o.StealthTCPPort = "443"
		} else {
			o.StealthTCPPort = "8443"
		}
	}
	if o.StealthUDPPort == "" {
		if o.AccessMode == config.ModePublic {
			o.StealthUDPPort = "443"
		} else {
			o.StealthUDPPort = "4443"
		}
	}
	if o.HTTPPort == "" {
		o.HTTPPort = "9080"
	}
	if o.StateDir == "" {
		o.StateDir = "/var/lib/erebrus"
	}
	if o.DefaultIface == "" {
		o.DefaultIface = "eth0"
	}
}

// Render returns the env file contents.
func Render(o Options) string {
	ApplyModeDefaults(&o)
	iface := o.DefaultIface
	return fmt.Sprintf(`# Erebrus v2 node — generated %s
# Internal configuration — use "erebrus status" to verify readiness.
RUNTYPE=release
EREBRUS_ACCESS=%s
EREBRUS_MODE=%s
EREBRUS_NETWORK_PROFILE=%s
SERVER=0.0.0.0
HTTP_PORT=%s
NODE_NAME=%s
REGION=%s
ZONE=%s
MNEMONIC=%s
NODE_API_TOKEN=%s
GATEWAY_URL=%s
GATEWAY_AUTO_REGISTER=true
WALLET_CHAIN=SOLANA
API_PUBLIC_URL=http://%s:%s

WG_CONF_DIR=/etc/wireguard
WG_INTERFACE_NAME=wg0
WG_ENDPOINT_HOST=%s
WG_ENDPOINT_PORT=51820
WG_IPv4_SUBNET=10.0.0.1/16
WG_DNS=1.1.1.1
WG_POST_UP=iptables -A FORWARD -i %%i -j ACCEPT; iptables -A FORWARD -o %%i -j ACCEPT; iptables -t nat -A POSTROUTING -o %s -j MASQUERADE
WG_POST_DOWN=iptables -D FORWARD -i %%i -j ACCEPT; iptables -D FORWARD -o %%i -j ACCEPT; iptables -t nat -D POSTROUTING -o %s -j MASQUERADE

ENABLE_STEALTH=%t
STEALTH_TCP_PORT=%s
STEALTH_UDP_PORT=%s
REALITY_SERVER_NAMES=www.microsoft.com
HYSTERIA2_OBFS_PASSWORD=

ENABLE_APP_HOSTING=%t
APP_WILDCARD_DOMAIN=%s
PUBLIC_DOMAIN=%s
WILDCARD_DOMAIN=%s
PUBLIC_GATEWAY_ENABLED=%t

STATE_DIR=%s
CHAIN_REGISTRATION=off
`,
		time.Now().Format("2006-01-02 15:04:05"),
		o.AccessMode, deployModeFor(o), o.NetworkProfile,
		o.HTTPPort, o.NodeName, o.Region, o.Zone,
		o.Mnemonic, o.NodeAPIToken, o.GatewayURL,
		o.PublicAddress, o.HTTPPort,
		o.PublicAddress, iface, iface,
		o.EnableStealth, o.StealthTCPPort, o.StealthUDPPort,
		o.EnableAppHosting, o.AppWildcardDomain, o.PublicDomain, o.WildcardDomain, o.PublicGatewayEnabled,
		o.StateDir,
	)
}

// WriteFile writes the env file with restrictive permissions.
func WriteFile(path string, o Options) error {
	if path == "" {
		path = DefaultEnvPath
	}
	content := Render(o)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func deployModeFor(o Options) config.DeployMode {
	if o.NetworkProfile == config.NetworkHostNetwork {
		return config.DeployHost
	}
	return config.DeployContainer
}

// ParseAccessMode normalizes user input.
func ParseAccessMode(s string) (config.RuntimeMode, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return config.ModePublic, nil
	case "private":
		return config.ModePrivate, nil
	case "shared":
		return config.ModePrivate, nil
	case "public", "gateway":
		return config.ModePublic, nil
	default:
		return "", fmt.Errorf("access mode must be private or public (got %q)", s)
	}
}
