package config

import (
	"strings"
	"testing"
)

func TestParseModeDefaults(t *testing.T) {
	m, err := ParseModeSettings("", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.RuntimeMode != ModePrivate || m.NetworkProfile != NetworkBridge {
		t.Fatalf("got mode=%s profile=%s", m.RuntimeMode, m.NetworkProfile)
	}
}

func TestParseModeShared(t *testing.T) {
	m, err := ParseModeSettings("shared", "bridge")
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsShared() || m.NetworkProfile != NetworkBridge {
		t.Fatalf("got mode=%s profile=%s", m.RuntimeMode, m.NetworkProfile)
	}
}

func TestParseModePublicExplicit(t *testing.T) {
	m, err := ParseModeSettings("public", "host-network")
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsPublic() || m.NetworkProfile != NetworkHostNetwork {
		t.Fatalf("got mode=%s profile=%s", m.RuntimeMode, m.NetworkProfile)
	}
}

func TestLegacyGatewayAlias(t *testing.T) {
	m, err := ParseModeSettings("gateway", "host-network")
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsPublic() || m.NetworkProfile != NetworkHostNetwork {
		t.Fatalf("got mode=%s profile=%s", m.RuntimeMode, m.NetworkProfile)
	}
	if len(m.Warnings) == 0 || !strings.Contains(m.Warnings[0], "deprecated") {
		t.Fatalf("expected deprecation warning, got %v", m.Warnings)
	}
}

func TestLegacyDockerAlias(t *testing.T) {
	m, err := ParseModeSettings("docker", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.RuntimeMode != ModePrivate || m.NetworkProfile != NetworkBridge {
		t.Fatalf("got mode=%s profile=%s", m.RuntimeMode, m.NetworkProfile)
	}
	if len(m.Warnings) == 0 || !strings.Contains(m.Warnings[0], "deprecated") {
		t.Fatalf("expected deprecation warning, got %v", m.Warnings)
	}
}

func TestLegacyHostAlias(t *testing.T) {
	m, err := ParseModeSettings("host", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.RuntimeMode != ModePublic || m.NetworkProfile != NetworkHostNetwork {
		t.Fatalf("got mode=%s profile=%s", m.RuntimeMode, m.NetworkProfile)
	}
	if len(m.Warnings) == 0 || !strings.Contains(m.Warnings[0], "deprecated") {
		t.Fatalf("expected deprecation warning, got %v", m.Warnings)
	}
}

func TestPublicBridgeWarning(t *testing.T) {
	m, err := ParseModeSettings("public", "bridge")
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

func TestInvalidMode(t *testing.T) {
	if _, err := ParseModeSettings("astro", "bridge"); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestInvalidProfile(t *testing.T) {
	if _, err := ParseModeSettings("private", "container"); err == nil {
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