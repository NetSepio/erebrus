package drop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NetSepio/erebrus/internal/config"
)

func TestProbePublicGatewayURLReachableOnMissingCID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/ipfs/"+probeCID {
			t.Errorf("expected probe path /ipfs/%s, got %s", probeCID, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	if !ProbePublicGatewayURL(context.Background(), server.URL) {
		t.Fatal("expected 404 to be considered reachable")
	}
}

func TestProbePublicGatewayURLNotReachableOnProxyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	if ProbePublicGatewayURL(context.Background(), server.URL) {
		t.Fatal("expected 502 to be considered unreachable")
	}
}

func TestProbePublicGatewayURLInvalidBaseURL(t *testing.T) {
	if ProbePublicGatewayURL(context.Background(), "http://[::1:invalid") {
		t.Fatal("expected invalid URL to be unreachable")
	}
}

func TestPublicGatewayURLGatedByOperationalState(t *testing.T) {
	cfg := config.Load()
	cfg.DropEnabled = true
	cfg.DropPublicGatewayDomain = "drop.example.com"
	service := NewService(cfg, nil)
	service.identityReady = true
	service.setSnapshot(Snapshot{State: "active", StorageMaxBytes: cfg.DropStorageMaxBytes})

	service.setPublicGatewayURL("https://drop.example.com")
	if got := service.PublicGatewayURL(); got != "https://drop.example.com" {
		t.Fatalf("expected URL when operational, got %q", got)
	}

	service.setSnapshot(Snapshot{State: "unreachable", StorageMaxBytes: cfg.DropStorageMaxBytes})
	if got := service.PublicGatewayURL(); got != "" {
		t.Fatalf("expected empty URL when not operational, got %q", got)
	}
}
