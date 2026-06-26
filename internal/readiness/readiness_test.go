package readiness

import (
	"testing"

	"github.com/NetSepio/erebrus/internal/config"
)

func TestEvaluateReadyPrivate(t *testing.T) {
	cfg := config.Load()
	cfg.Mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	cfg.WGEndpointHost = "203.0.113.1"
	cfg.NodeAPIToken = "secret"
	cfg.RunType = "release"

	r := Evaluate(Input{
		Cfg:                cfg,
		IdentityConfigured: true,
		WireGuardOK:        true,
		StealthListening:   true,
		GatewayRegistered:  true,
		GatewayConnected:   true,
	})
	if !r.OK {
		t.Fatalf("expected ready, got %+v", r)
	}
}

func TestEvaluateMissingPublicAddress(t *testing.T) {
	cfg := config.Load()
	cfg.Mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	cfg.NodeAPIToken = "secret"

	r := Evaluate(Input{Cfg: cfg, IdentityConfigured: true, WireGuardOK: true})
	if r.OK {
		t.Fatal("expected not ready")
	}
}

func TestPreboot(t *testing.T) {
	cfg := config.Load()
	cfg.Mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	cfg.WGEndpointHost = "203.0.113.1"
	cfg.NodeAPIToken = "secret"
	r := Preboot(cfg)
	if !r.OK {
		t.Fatalf("preboot should pass config checks: %+v", r)
	}
}

func TestAccessModeLabel(t *testing.T) {
	if AccessModeLabel(config.ModePublic) != "Public" {
		t.Fatalf("public label = %q", AccessModeLabel(config.ModePublic))
	}
}

func TestRegionLabel(t *testing.T) {
	if RegionLabel("NO") != "Norway" {
		t.Fatalf("NO = %q", RegionLabel("NO"))
	}
	if RegionLabel("EU-WEST") != "EU-WEST" {
		t.Fatalf("custom region = %q", RegionLabel("EU-WEST"))
	}
}

func TestZoneLabel(t *testing.T) {
	if ZoneLabel("") != "" {
		t.Fatalf("empty zone")
	}
	if ZoneLabel("east") != "US East" {
		t.Fatalf("east = %q", ZoneLabel("east"))
	}
	if ZoneLabel("us-west") != "US West" {
		t.Fatalf("us-west = %q", ZoneLabel("us-west"))
	}
	if ZoneLabel("nyc-1") != "nyc-1" {
		t.Fatalf("custom zone = %q", ZoneLabel("nyc-1"))
	}
}