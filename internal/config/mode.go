package config

import (
	"fmt"
	"strings"
)

// RuntimeMode is the node's product mode (not its deployment method).
type RuntimeMode string

const (
	ModePrivate RuntimeMode = "private"
	ModeGateway RuntimeMode = "gateway"
)

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

// ModeSettings holds parsed runtime mode and network profile.
type ModeSettings struct {
	RuntimeMode    RuntimeMode
	NetworkProfile NetworkProfile
	Warnings       []string
}

// ParseModeSettings reads EREBRUS_MODE and EREBRUS_NETWORK_PROFILE, applying
// defaults and legacy docker/host alias mapping.
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
			warnings = append(warnings, deprecationLegacyMode(legacyModeDocker, string(ModePrivate), string(NetworkBridge)))
		}
	case legacyModeHost:
		mode = ModeGateway
		warnings = append(warnings, deprecationLegacyMode(legacyModeHost, string(ModeGateway), string(NetworkHostNetwork)))
	case string(ModePrivate), string(ModeGateway):
		mode = RuntimeMode(modeRaw)
	default:
		return ModeSettings{}, fmt.Errorf("EREBRUS_MODE must be private or gateway (got %q)", modeRaw)
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

	if mode == ModeGateway && profile == NetworkBridge {
		warnings = append(warnings,
			"WARNING: Gateway Mode with bridge networking may work, but host-network is recommended for production gateway nodes because WireGuard routing, 443 binding, reverse proxying, and debugging are simpler.")
	}
	if profile == NetworkNative {
		warnings = append(warnings,
			"WARNING: EREBRUS_NETWORK_PROFILE=native is experimental; Docker-first deployment is recommended.")
	}

	return ModeSettings{RuntimeMode: mode, NetworkProfile: profile, Warnings: warnings}, nil
}

func deprecationLegacyMode(legacy, mode, profile string) string {
	return fmt.Sprintf(
		"WARNING: legacy %q mode is deprecated. Use EREBRUS_MODE=%s with EREBRUS_NETWORK_PROFILE=%s.",
		legacy, mode, profile,
	)
}

// IsGateway reports whether the node runs in Gateway Mode.
func (m ModeSettings) IsGateway() bool { return m.RuntimeMode == ModeGateway }

// IsPrivate reports whether the node runs in Private Mode.
func (m ModeSettings) IsPrivate() bool { return m.RuntimeMode == ModePrivate }
