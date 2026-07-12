package gatewayclient

import (
	"encoding/json"
	"strings"
	"testing"
)

const canonicalHello = `{
  "type": "hello",
  "data": {
    "node_id": "12D3KooWQYhTNQdmr3ArTeo5gCtJ8m1bbb73Bb4Q4xxK9zMrf1nK",
    "version": "2.0.0",
    "identity": {
      "peer_id": "12D3KooWQYhTNQdmr3ArTeo5gCtJ8m1bbb73Bb4Q4xxK9zMrf1nK",
      "did": "did:erebrus:12D3KooWQYhTNQdmr3ArTeo5gCtJ8m1bbb73Bb4Q4xxK9zMrf1nK",
      "ip_hash": "f1820f54e0e51b8a1a47b0ec96265d6021b3a0b6c6c61563b1d62fa4a4b0d3c2"
    },
    "spec": { "cpu": "4 vCPU", "mem_mb": 8192, "region": "SG", "ip": "203.0.113.10" },
    "capabilities": { "app_hosting": false, "wildcard_domain": "" },
    "endpoints": {
      "wireguard":     { "port": 51820, "public_key": "wOLuwnTGzkkCC1WiV2t5HpJ56FftZyXTK0WnWxSDFkI=" },
      "vless_reality": { "port": 8443,  "public_key": "SRYxyiZ1Tr3w0aV3PXAhd1NSjpvm8wOCnnlLWWBd7Vc", "short_ids": ["6ba85179e30d4fc2"], "sni": "www.microsoft.com" },
      "hysteria2":     { "port": 4443,  "obfs": "" }
    }
  }
}`

func TestParseCanonicalHello(t *testing.T) {
	var env Envelope
	if err := json.Unmarshal([]byte(canonicalHello), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.Type != TypeHello {
		t.Fatalf("type = %q, want %q", env.Type, TypeHello)
	}
	var h Hello
	if err := json.Unmarshal(env.Data, &h); err != nil {
		t.Fatalf("unmarshal hello: %v", err)
	}
	if h.NodeID != "12D3KooWQYhTNQdmr3ArTeo5gCtJ8m1bbb73Bb4Q4xxK9zMrf1nK" {
		t.Errorf("node_id = %q", h.NodeID)
	}
	if h.NodeID != h.Identity.PeerID {
		t.Errorf("node_id %q should equal peer_id %q", h.NodeID, h.Identity.PeerID)
	}
	if h.Identity.DID != "did:erebrus:"+"12D3KooWQYhTNQdmr3ArTeo5gCtJ8m1bbb73Bb4Q4xxK9zMrf1nK" {
		t.Errorf("did = %q", h.Identity.DID)
	}
	if h.Spec.MemMB != 8192 || h.Spec.Region != "SG" {
		t.Errorf("spec = %+v", h.Spec)
	}
	if h.Endpoints.WireGuard.Port != 51820 || h.Endpoints.Hysteria2.Port != 4443 {
		t.Errorf("endpoints ports wrong: %+v", h.Endpoints)
	}
	if len(h.Endpoints.VLESSReality.ShortIDs) != 1 || h.Endpoints.VLESSReality.SNI != "www.microsoft.com" {
		t.Errorf("vless endpoint = %+v", h.Endpoints.VLESSReality)
	}
}

func TestHeartbeatAndUsageRoundTrip(t *testing.T) {
	hb := Heartbeat{
		TS: 1765584000, Status: "online",
		Load:      Load{WGPeersRegistered: 42, WGPeersConnected: 40, ProxySessions: 7, CPUPct: 23.5, MemPct: 41.2, RxBytes: 123456789, TxBytes: 987654321},
		Speedtest: Speedtest{DownloadMbps: 940.2, UploadMbps: 870.1, LatencyMs: 3.2, MeasuredAt: 1765580400},
		Versions:  map[string]string{"node": "2.0.0", "singbox": "1.11.4", "kubo": "0.42.0"},
		Services:  map[string]string{"vpn": "active", "drop": "active"},
		Drop: &DropStatus{
			State: "active", KuboVersion: "0.42.0", RepoSizeBytes: 1048576,
			StorageMaxBytes: 10000000000, NumObjects: 12,
		},
	}
	frame, err := wrap(TypeHeartbeat, hb)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(frame, &env); err != nil || env.Type != TypeHeartbeat {
		t.Fatalf("envelope: %v type=%s", err, env.Type)
	}
	var got Heartbeat
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("unmarshal heartbeat: %v", err)
	}
	if got.Load.RxBytes != 123456789 || got.Load.TxBytes != 987654321 {
		t.Errorf("byte counters lost: %+v", got.Load)
	}
	if got.Speedtest.DownloadMbps != 940.2 || got.Speedtest.UploadMbps != 870.1 ||
		got.Speedtest.LatencyMs != 3.2 || got.Speedtest.MeasuredAt != 1765580400 {
		t.Errorf("speedtest fields lost: %+v", got.Speedtest)
	}
	if got.Drop == nil || got.Drop.State != "active" || got.Drop.StorageMaxBytes != 10000000000 {
		t.Errorf("drop status lost: %+v", got.Drop)
	}
	if got.Services["drop"] != "active" || got.Versions["kubo"] != "0.42.0" {
		t.Errorf("drop service metadata lost: services=%v versions=%v", got.Services, got.Versions)
	}

	ur := UsageReport{TS: 1765584000, Peers: []PeerUsage{{PeerID: "c0a4f1de", RxBytesDelta: 1048576, TxBytesDelta: 8388608, LastHandshake: 1765583970}}}
	frame, _ = wrap(TypeUsageReport, ur)
	_ = json.Unmarshal(frame, &env)
	var gotUR UsageReport
	if err := json.Unmarshal(env.Data, &gotUR); err != nil {
		t.Fatalf("unmarshal usage: %v", err)
	}
	if len(gotUR.Peers) != 1 || gotUR.Peers[0].TxBytesDelta != 8388608 {
		t.Errorf("usage peers wrong: %+v", gotUR.Peers)
	}
}

func TestDropCapabilityRoundTrip(t *testing.T) {
	h := Hello{
		Capabilities: Capabilities{
			Drop: &DropCapability{
				Enabled: true, AcceptsPublicUploads: true, PublicGatewayURL: "https://drop.example.com",
				WebUIAvailable: false,
			},
		},
	}
	frame, err := wrap(TypeHello, h)
	if err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(frame, &env); err != nil {
		t.Fatal(err)
	}
	var got Hello
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Capabilities.Drop == nil || !got.Capabilities.Drop.Enabled ||
		!got.Capabilities.Drop.AcceptsPublicUploads ||
		got.Capabilities.Drop.PublicGatewayURL != "https://drop.example.com" ||
		got.Capabilities.Drop.WebUIAvailable {
		t.Fatalf("drop capability = %+v", got.Capabilities.Drop)
	}
}

func TestDropCapabilityPublicGatewayURLOmitEmpty(t *testing.T) {
	h := Hello{
		Capabilities: Capabilities{
			Drop: &DropCapability{
				Enabled: true, AcceptsPublicUploads: true, WebUIAvailable: false,
			},
		},
	}
	frame, err := wrap(TypeHello, h)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(frame), "public_gateway_url") {
		t.Fatalf("public_gateway_url should be omitted when empty: %s", string(frame))
	}
}
