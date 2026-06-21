package config

import (
	"fmt"
	"os"
	"strings"
)

// RuntimeMode is who may discover and use the node (access policy).
type RuntimeMode string

const (
	ModePrivate RuntimeMode = "private" // operator devices only
	ModeShared  RuntimeMode = "shared"  // wallet allowlist on gateway
	ModePublic  RuntimeMode = "public"  // open directory; host earnings (future)
)

// DeployMode is how the node process is run on the machine.
type DeployMode string

const (
	DeployContainer DeployMode = "container" // Docker / compose (bridge networking)
	DeployHost      DeployMode = "host"      // bare metal / systemd (host-network)
)

// NetworkProfile describes container vs host networking (advanced override).
type NetworkProfile string

const (
	NetworkBridge      NetworkProfile = "bridge"
	NetworkHostNetwork NetworkProfile = "host-network"
	NetworkNative      NetworkProfile = "native"
)

// Legacy env/install aliases (deprecated).
const (
	legacyModeGateway = "gateway"
	legacyModeDocker = "docker"
)

// ModeSettings holds parsed access, deploy, and network profile.
type ModeSettings struct {
	RuntimeMode    RuntimeMode // access: private | shared | public
	Deploy         DeployMode  // container | host
	NetworkProfile NetworkProfile
	Warnings       []string
}

// ParseModeSettings reads EREBRUS_ACCESS, EREBRUS_MODE (deploy), and
// EREBRUS_NETWORK_PROFILE with legacy fallbacks.
func ParseModeSettings(accessRaw, deployRaw, profileRaw string) (ModeSettings, error) {
	accessRaw = strings.ToLower(strings.TrimSpace(accessRaw))
	deployRaw = strings.ToLower(strings.TrimSpace(deployRaw))
	profileRaw = strings.ToLower(strings.TrimSpace(profileRaw))

	var warnings []string

	// Legacy: EREBRUS_MODE used to mean access (private/shared/public/gateway).
	if accessRaw == "" && isAccessToken(deployRaw) {
		warnings = append(warnings, fmt.Sprintf(
			"WARNING: EREBRUS_MODE=%q is deprecated for access. Use EREBRUS_ACCESS=%s and EREBRUS_MODE=container|host for deploy.",
			deployRaw, normalizeAccessToken(deployRaw)))
		accessRaw = deployRaw
		deployRaw = ""
	}

	access, accessWarnings, err := parseAccess(accessRaw)
	if err != nil {
		return ModeSettings{}, err
	}
	warnings = append(warnings, accessWarnings...)

	deploy, deployWarnings, err := parseDeploy(deployRaw)
	if err != nil {
		return ModeSettings{}, err
	}
	warnings = append(warnings, deployWarnings...)

	profile, profileWarnings, err := parseNetworkProfile(profileRaw, deploy)
	if err != nil {
		return ModeSettings{}, err
	}
	warnings = append(warnings, profileWarnings...)

	if access == ModePublic && profile == NetworkBridge {
		warnings = append(warnings,
			"WARNING: Public access with bridge networking may work, but host-network is recommended for production nodes because WireGuard routing, 443 binding, reverse proxying, and debugging are simpler.")
	}
	if profile == NetworkNative {
		warnings = append(warnings,
			"WARNING: EREBRUS_NETWORK_PROFILE=native is experimental; container deployment is recommended.")
	}

	return ModeSettings{
		RuntimeMode:    access,
		Deploy:         deploy,
		NetworkProfile: profile,
		Warnings:       warnings,
	}, nil
}

// ParseModeSettingsFromEnv is the env-backed entry point.
func ParseModeSettingsFromEnv() (ModeSettings, error) {
	return ParseModeSettings(
		os.Getenv("EREBRUS_ACCESS"),
		os.Getenv("EREBRUS_MODE"),
		os.Getenv("EREBRUS_NETWORK_PROFILE"),
	)
}

func isAccessToken(s string) bool {
	switch s {
	case "", string(ModePrivate), string(ModeShared), string(ModePublic), legacyModeGateway:
		return s != ""
	default:
		return false
	}
}

func normalizeAccessToken(s string) string {
	if s == legacyModeGateway {
		return string(ModePublic)
	}
	return s
}

func parseAccess(raw string) (RuntimeMode, []string, error) {
	var warnings []string
	switch raw {
	case "":
		return ModePrivate, warnings, nil
	case string(ModePrivate):
		return ModePrivate, warnings, nil
	case string(ModeShared):
		return ModeShared, warnings, nil
	case string(ModePublic):
		return ModePublic, warnings, nil
	case legacyModeGateway:
		warnings = append(warnings, fmt.Sprintf(
			"WARNING: access %q is deprecated. Use EREBRUS_ACCESS=%s.", legacyModeGateway, ModePublic))
		return ModePublic, warnings, nil
	default:
		return "", nil, fmt.Errorf("EREBRUS_ACCESS must be private, shared, or public (got %q)", raw)
	}
}

func parseDeploy(raw string) (DeployMode, []string, error) {
	var warnings []string
	switch raw {
	case "", legacyModeDocker:
		if raw == legacyModeDocker {
			warnings = append(warnings, fmt.Sprintf(
				"WARNING: EREBRUS_MODE=%q is deprecated. Use EREBRUS_MODE=%s.", legacyModeDocker, DeployContainer))
		}
		return DeployContainer, warnings, nil
	case string(DeployContainer):
		return DeployContainer, warnings, nil
	case string(DeployHost):
		return DeployHost, warnings, nil
	default:
		return "", nil, fmt.Errorf("EREBRUS_MODE must be container or host (got %q)", raw)
	}
}

func parseNetworkProfile(raw string, deploy DeployMode) (NetworkProfile, []string, error) {
	var warnings []string
	switch raw {
	case "":
		if deploy == DeployHost {
			return NetworkHostNetwork, warnings, nil
		}
		return NetworkBridge, warnings, nil
	case string(NetworkBridge), string(NetworkHostNetwork), string(NetworkNative):
		return NetworkProfile(raw), warnings, nil
	default:
		return "", nil, fmt.Errorf("EREBRUS_NETWORK_PROFILE must be bridge, host-network, or native (got %q)", raw)
	}
}

// IsPrivate reports whether only the operator and their devices may use the node.
func (m ModeSettings) IsPrivate() bool { return m.RuntimeMode == ModePrivate }

// IsShared reports whether access is limited to a gateway wallet allowlist.
func (m ModeSettings) IsShared() bool { return m.RuntimeMode == ModeShared }

// IsPublic reports whether the node is open to entitled network users.
func (m ModeSettings) IsPublic() bool { return m.RuntimeMode == ModePublic }

// IsGateway is deprecated; use IsPublic.
func (m ModeSettings) IsGateway() bool { return m.IsPublic() }

// IsContainer reports Docker/compose deployment.
func (m ModeSettings) IsContainer() bool { return m.Deploy == DeployContainer }

// IsHostDeploy reports bare-metal/systemd deployment.
func (m ModeSettings) IsHostDeploy() bool { return m.Deploy == DeployHost }