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