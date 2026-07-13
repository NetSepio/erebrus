package config

import (
	"fmt"
	"os"
	"strings"
)

// RuntimeMode is who may discover and use the node (access policy).
type RuntimeMode string

const (
	ModePrivate RuntimeMode = "private" // operator + org members; not in public directory
	ModePublic  RuntimeMode = "public"  // listed in public directory
)

// NetworkProfile describes Docker container networking.
type NetworkProfile string

const (
	NetworkBridge      NetworkProfile = "bridge"
	NetworkHostNetwork NetworkProfile = "host-network"
	NetworkNative      NetworkProfile = "native"
)

// Legacy env/install aliases (deprecated).
const (
	legacyModeGateway = "gateway"
	legacyModeDocker  = "docker"
)

// ModeSettings holds parsed access and network profile.
type ModeSettings struct {
	RuntimeMode    RuntimeMode // access: private | public
	NetworkProfile NetworkProfile
	Warnings       []string
}

// ParseModeSettings reads EREBRUS_ACCESS and EREBRUS_NETWORK_PROFILE with legacy fallbacks.
func ParseModeSettings(accessRaw, profileRaw string) (ModeSettings, error) {
	accessRaw = strings.ToLower(strings.TrimSpace(accessRaw))
	profileRaw = strings.ToLower(strings.TrimSpace(profileRaw))

	var warnings []string

	// Legacy: EREBRUS_MODE used to mean access (private/public/gateway).
	if accessRaw == "" && isAccessToken(profileRaw) {
		warnings = append(warnings, fmt.Sprintf(
			"WARNING: EREBRUS_MODE=%q is deprecated for access. Use EREBRUS_ACCESS=%s.",
			profileRaw, normalizeAccessToken(profileRaw)))
		accessRaw = profileRaw
		profileRaw = ""
	}

	access, accessWarnings, err := parseAccess(accessRaw)
	if err != nil {
		return ModeSettings{}, err
	}
	warnings = append(warnings, accessWarnings...)

	profile, profileWarnings, err := parseNetworkProfile(profileRaw)
	if err != nil {
		return ModeSettings{}, err
	}
	warnings = append(warnings, profileWarnings...)

	if profile == NetworkNative {
		warnings = append(warnings,
			"WARNING: EREBRUS_NETWORK_PROFILE=native is experimental; container deployment is recommended.")
	}

	return ModeSettings{
		RuntimeMode:    access,
		NetworkProfile: profile,
		Warnings:       warnings,
	}, nil
}

// ParseModeSettingsFromEnv is the env-backed entry point.
func ParseModeSettingsFromEnv() (ModeSettings, error) {
	return ParseModeSettings(
		os.Getenv("EREBRUS_ACCESS"),
		os.Getenv("EREBRUS_NETWORK_PROFILE"),
	)
}

func isAccessToken(s string) bool {
	switch s {
	case "", string(ModePrivate), string(ModePublic), legacyModeGateway:
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
		return ModePublic, warnings, nil
	case string(ModePrivate):
		return ModePrivate, warnings, nil
	case string(ModePublic):
		return ModePublic, warnings, nil
	case legacyModeGateway:
		warnings = append(warnings, fmt.Sprintf(
			"WARNING: access %q is deprecated. Use EREBRUS_ACCESS=%s.", legacyModeGateway, ModePublic))
		return ModePublic, warnings, nil
	default:
		return "", nil, fmt.Errorf("EREBRUS_ACCESS must be private or public (got %q)", raw)
	}
}

func parseNetworkProfile(raw string) (NetworkProfile, []string, error) {
	var warnings []string
	switch raw {
	case "":
		return NetworkBridge, warnings, nil
	case string(NetworkBridge), string(NetworkHostNetwork), string(NetworkNative):
		return NetworkProfile(raw), warnings, nil
	default:
		return "", nil, fmt.Errorf("EREBRUS_NETWORK_PROFILE must be bridge, host-network, or native (got %q)", raw)
	}
}

// IsPrivate reports whether only the operator and their devices may use the node.
func (m ModeSettings) IsPrivate() bool { return m.RuntimeMode == ModePrivate }

// IsPublic reports whether the node is open to entitled network users.
func (m ModeSettings) IsPublic() bool { return m.RuntimeMode == ModePublic }

// IsGateway is deprecated; use IsPublic.
func (m ModeSettings) IsGateway() bool { return m.IsPublic() }


// GatewayAccessMode maps local access policy to gateway public|private.
func (m ModeSettings) GatewayAccessMode() string {
	if m.RuntimeMode == ModePublic {
		return "public"
	}
	return "private"
}
