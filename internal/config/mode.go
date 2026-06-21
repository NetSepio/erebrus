package config

import (
	"fmt"
	"strings"
)

// RuntimeMode is the node's access policy: who can discover and use the VPN.
// It is not the deployment method (see NetworkProfile).
type RuntimeMode string

const (
	ModePrivate RuntimeMode = "private" // host + own devices only
	ModeShared  RuntimeMode = "shared"  // wallet allowlist on gateway (friends)
	ModePublic  RuntimeMode = "public"  // open directory; host earnings (future)
)

// legacyModeGateway is the deprecated v2.0 name for public access mode.
const legacyModeGateway = "gateway"

// NetworkProfile describes how the node is deployed on the network.
type NetworkProfile string

const (
	NetworkBridge      NetworkProfile = "bridge"
	NetworkHostNetwork NetworkProfile = "host-network"
	NetworkNative      NetworkProfile = "native"
)

// Legacy install/deployment aliases (deprecated).
const (
	legacyModeDocker = "docker"
	legacyModeHost   = "host"
)

// ModeSettings holds parsed access mode and network profile.
type ModeSettings struct {
	RuntimeMode    RuntimeMode
	NetworkProfile NetworkProfile
	Warnings       []string
}

// ParseModeSettings reads EREBRUS_MODE and EREBRUS_NETWORK_PROFILE, applying
// defaults and legacy aliases.
func ParseModeSettings(modeRaw, profileRaw string) (ModeSettings, error) {
	modeRaw = strings.ToLower(strings.TrimSpace(modeRaw))
	profileRaw = strings.ToLower(strings.TrimSpace(profileRaw))

	var warnings []string
	mode := RuntimeMode(modeRaw)
	profile := NetworkProfile(profileRaw)

	switch modeRaw {
	case "", legacyModeDocker:
		mode = ModePrivate
		if modeRaw == legacyModeDocker {
			warnings = append(warnings, deprecationLegacyInstallMode(legacyModeDocker, string(ModePrivate), string(NetworkBridge)))
		}
	case legacyModeHost:
		mode = ModePublic
		profile = NetworkHostNetwork
		warnings = append(warnings, deprecationLegacyInstallMode(legacyModeHost, string(ModePublic), string(NetworkHostNetwork)))
	case legacyModeGateway:
		mode = ModePublic
		warnings = append(warnings, fmt.Sprintf(
			"WARNING: EREBRUS_MODE=%q is deprecated. Use EREBRUS_MODE=%s (public access — open to entitled users).",
			legacyModeGateway, ModePublic))
	case string(ModePrivate), string(ModeShared), string(ModePublic):
		mode = RuntimeMode(modeRaw)
	default:
		return ModeSettings{}, fmt.Errorf("EREBRUS_MODE must be private, shared, or public (got %q)", modeRaw)
	}

	switch profileRaw {
	case "":
		if modeRaw == legacyModeHost {
			profile = NetworkHostNetwork
		} else {
			profile = NetworkBridge
		}
	case string(NetworkBridge), string(NetworkHostNetwork), string(NetworkNative):
		profile = NetworkProfile(profileRaw)
	default:
		return ModeSettings{}, fmt.Errorf("EREBRUS_NETWORK_PROFILE must be bridge, host-network, or native (got %q)", profileRaw)
	}

	if mode == ModePublic && profile == NetworkBridge {
		warnings = append(warnings,
			"WARNING: Public access mode with bridge networking may work, but host-network is recommended for production nodes because WireGuard routing, 443 binding, reverse proxying, and debugging are simpler.")
	}
	if profile == NetworkNative {
		warnings = append(warnings,
			"WARNING: EREBRUS_NETWORK_PROFILE=native is experimental; Docker-first deployment is recommended.")
	}

	return ModeSettings{RuntimeMode: mode, NetworkProfile: profile, Warnings: warnings}, nil
}

func deprecationLegacyInstallMode(legacy, mode, profile string) string {
	return fmt.Sprintf(
		"WARNING: legacy install %q is deprecated. Use EREBRUS_MODE=%s with EREBRUS_NETWORK_PROFILE=%s.",
		legacy, mode, profile)
}

// IsPrivate reports whether only the host and their devices may use the node.
func (m ModeSettings) IsPrivate() bool { return m.RuntimeMode == ModePrivate }

// IsShared reports whether access is limited to a gateway wallet allowlist.
func (m ModeSettings) IsShared() bool { return m.RuntimeMode == ModeShared }

// IsPublic reports whether the node is open to entitled network users.
func (m ModeSettings) IsPublic() bool { return m.RuntimeMode == ModePublic }

// IsGateway is deprecated; use IsPublic.
func (m ModeSettings) IsGateway() bool { return m.IsPublic() }