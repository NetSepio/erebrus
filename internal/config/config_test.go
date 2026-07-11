package config

import (
	"strings"
	"testing"
)

func TestParseModeDefaults(t *testing.T) {
	m, err := ParseModeSettings("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.RuntimeMode != ModePublic || m.Deploy != DeployContainer || m.NetworkProfile != NetworkBridge {
		t.Fatalf("got access=%s deploy=%s profile=%s", m.RuntimeMode, m.Deploy, m.NetworkProfile)
	}
	if m.GatewayAccessMode() != "public" {
		t.Fatalf("gateway access = %q, want public", m.GatewayAccessMode())
	}
}

func TestParseAccessSharedDeprecated(t *testing.T) {
	m, err := ParseModeSettings("shared", "container", "bridge")
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsPrivate() || m.Deploy != DeployContainer || m.NetworkProfile != NetworkBridge {
		t.Fatalf("got access=%s deploy=%s profile=%s", m.RuntimeMode, m.Deploy, m.NetworkProfile)
	}
	if len(m.Warnings) == 0 || !strings.Contains(m.Warnings[0], "deprecated") {
		t.Fatalf("expected shared deprecation warning, got %v", m.Warnings)
	}
	if m.GatewayAccessMode() != "private" {
		t.Fatalf("gateway access = %q, want private", m.GatewayAccessMode())
	}
}

func TestParsePublicHost(t *testing.T) {
	m, err := ParseModeSettings("public", "host", "host-network")
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsPublic() || m.Deploy != DeployHost || m.NetworkProfile != NetworkHostNetwork {
		t.Fatalf("got access=%s deploy=%s profile=%s", m.RuntimeMode, m.Deploy, m.NetworkProfile)
	}
}

func TestPublicContainer(t *testing.T) {
	m, err := ParseModeSettings("public", "container", "")
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsPublic() || m.Deploy != DeployContainer || m.NetworkProfile != NetworkBridge {
		t.Fatalf("got access=%s deploy=%s profile=%s", m.RuntimeMode, m.Deploy, m.NetworkProfile)
	}
}

func TestLegacyAccessInModeEnv(t *testing.T) {
	m, err := ParseModeSettings("", "gateway", "host-network")
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsPublic() || m.Deploy != DeployContainer {
		t.Fatalf("got access=%s deploy=%s profile=%s", m.RuntimeMode, m.Deploy, m.NetworkProfile)
	}
	if len(m.Warnings) == 0 || !strings.Contains(m.Warnings[0], "deprecated") {
		t.Fatalf("expected deprecation warning, got %v", m.Warnings)
	}
}

func TestLegacyDockerDeployAlias(t *testing.T) {
	m, err := ParseModeSettings("", "docker", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.RuntimeMode != ModePublic || m.Deploy != DeployContainer || m.NetworkProfile != NetworkBridge {
		t.Fatalf("got access=%s deploy=%s profile=%s", m.RuntimeMode, m.Deploy, m.NetworkProfile)
	}
	if len(m.Warnings) == 0 || !strings.Contains(m.Warnings[0], "deprecated") {
		t.Fatalf("expected deprecation warning, got %v", m.Warnings)
	}
}

func TestLegacyHostDeployDecoupledFromAccess(t *testing.T) {
	m, err := ParseModeSettings("", "host", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.RuntimeMode != ModePublic || m.Deploy != DeployHost || m.NetworkProfile != NetworkHostNetwork {
		t.Fatalf("got access=%s deploy=%s profile=%s", m.RuntimeMode, m.Deploy, m.NetworkProfile)
	}
}

func TestPublicBridgeWarning(t *testing.T) {
	m, err := ParseModeSettings("public", "container", "bridge")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range m.Warnings {
		if strings.Contains(w, "host-network is recommended") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected public+bridge warning, got %v", m.Warnings)
	}
}

func TestInvalidAccess(t *testing.T) {
	if _, err := ParseModeSettings("astro", "container", "bridge"); err == nil {
		t.Fatal("expected error for invalid access")
	}
}

func TestInvalidDeploy(t *testing.T) {
	if _, err := ParseModeSettings("private", "vm", "bridge"); err == nil {
		t.Fatal("expected error for invalid deploy")
	}
}

func TestInvalidProfile(t *testing.T) {
	if _, err := ParseModeSettings("private", "container", "container"); err == nil {
		t.Fatal("expected error for invalid profile")
	}
}

func TestLoadAPIBindDefault(t *testing.T) {
	t.Setenv("SERVER", "")
	t.Setenv("API_BIND_ADDR", "")
	t.Setenv("UNSAFE_PUBLIC_API", "")
	t.Setenv("MNEMONIC", "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	t.Setenv("WG_ENDPOINT_HOST", "203.0.113.1")
	c := Load()
	if c.BindAddr != "0.0.0.0" {
		t.Fatalf("bind addr = %q, want 0.0.0.0 default during testing", c.BindAddr)
	}
}

func TestLoadRegistrationTokenAlias(t *testing.T) {
	t.Setenv("EREBRUS_NODE_REGISTRATION_TOKEN", "")
	t.Setenv("EREBRUS_ORG_ENROLLMENT_SECRET", "ere_reg_legacy")
	c := Load()
	if c.EffectiveRegistrationToken() != "ere_reg_legacy" {
		t.Fatalf("token = %q, want legacy alias", c.EffectiveRegistrationToken())
	}
	t.Setenv("EREBRUS_NODE_REGISTRATION_TOKEN", "ere_reg_new")
	c = Load()
	if c.EffectiveRegistrationToken() != "ere_reg_new" {
		t.Fatalf("token = %q, want ere_reg_new", c.EffectiveRegistrationToken())
	}
}

func TestLoadDropDefaultsAndOverrides(t *testing.T) {
	t.Setenv("DROP_ENABLED", "")
	t.Setenv("DROP_STORAGE_MAX", "")
	t.Setenv("DROP_SWARM_PORT", "")
	t.Setenv("DROP_WEBUI_ENABLED", "")
	c := Load()
	if c.DropEnabled || c.DropStorageMax != "10GB" || c.DropStorageMaxBytes != 10_000_000_000 ||
		c.DropSwarmPortInt() != 4001 || c.DropWebUIEnabled {
		t.Fatalf("Drop defaults = %+v", c)
	}

	t.Setenv("DROP_ENABLED", "true")
	t.Setenv("DROP_STORAGE_MAX", "5GB")
	t.Setenv("DROP_SWARM_PORT", "4101")
	t.Setenv("DROP_WEBUI_ENABLED", "true")
	t.Setenv("EREBRUS_ACCESS", "private")
	t.Setenv("EREBRUS_MODE", "container")
	c = Load()
	if !c.DropEnabled || c.DropStorageMaxBytes != 5_000_000_000 ||
		c.DropSwarmPortInt() != 4101 || !c.DropWebUIAvailable() {
		t.Fatalf("Drop overrides = %+v", c)
	}
}

func TestDropValidation(t *testing.T) {
	t.Setenv("DROP_ENABLED", "true")
	t.Setenv("DROP_STORAGE_MAX", "invalid")
	t.Setenv("EREBRUS_ACCESS", "private")
	t.Setenv("EREBRUS_MODE", "container")
	c := Load()
	c.Mnemonic = "test"
	c.WGEndpointHost = "203.0.113.1"
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "DROP_STORAGE_MAX") {
		t.Fatalf("expected storage validation error, got %v", err)
	}

	t.Setenv("DROP_STORAGE_MAX", "10GB")
	t.Setenv("EREBRUS_MODE", "host")
	c = Load()
	c.Mnemonic = "test"
	c.WGEndpointHost = "203.0.113.1"
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "container deployment") {
		t.Fatalf("expected host deployment error, got %v", err)
	}

	t.Setenv("EREBRUS_MODE", "container")
	t.Setenv("EREBRUS_ACCESS", "public")
	t.Setenv("DROP_WEBUI_ENABLED", "true")
	c = Load()
	c.Mnemonic = "test"
	c.WGEndpointHost = "203.0.113.1"
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "private or shared") {
		t.Fatalf("expected public WebUI error, got %v", err)
	}
}

func TestLoadAPIBindOverride(t *testing.T) {
	t.Setenv("API_BIND_ADDR", "127.0.0.1")
	t.Setenv("SERVER", "0.0.0.0")
	t.Setenv("MNEMONIC", "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	t.Setenv("WG_ENDPOINT_HOST", "203.0.113.1")
	c := Load()
	if c.BindAddr != "127.0.0.1" {
		t.Fatalf("bind addr = %q, want 127.0.0.1 from API_BIND_ADDR", c.BindAddr)
	}
}
