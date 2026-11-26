package node

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/firewall"
	"github.com/NetSepio/erebrus/internal/gatewayclient"
	"github.com/NetSepio/erebrus/internal/stealth"
	"github.com/NetSepio/erebrus/internal/store"
	"github.com/NetSepio/erebrus/internal/wg"
	"github.com/gorilla/websocket"
)

func TestSyncFirewallCommandReportsAppliedVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	bridge := &GatewayBridge{fw: firewall.New(&config.Config{FirewallProvider: config.FirewallUnboundErebrus, SentinelAPIURL: srv.URL}, nil)}
	for _, tc := range []struct {
		args    string
		ok      bool
		version string
	}{
		{`{"licensed":true,"rules":[],"policy_version":0}`, true, "0"},
		{`{"licensed":true,"rules":null}`, true, ""},
		{`{"licensed":false,"rules":[],"policy_version":9}`, false, ""},
	} {
		res := bridge.HandleCommand(context.Background(), gatewayclient.Command{Action: gatewayclient.ActionSyncFirewall, RequestID: "req", Args: json.RawMessage(tc.args)})
		raw, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		if res.RequestID != "req" || res.OK != tc.ok || string(fields["policy_version"]) != tc.version {
			t.Errorf("args %s produced %s", tc.args, raw)
		}
	}
}

func TestBootstrapHelloAndRotation(t *testing.T) {
	ctx := t.Context()
	st, err := store.Open(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	// Port zero lets the OS allocate test listeners. Only the credential is used here.
	cfg := &config.Config{EnableStealth: true, GatewayURL: "https://gateway.example", WGEndpointPort: "51820"}
	carrier := stealth.New(cfg, st)
	if err := carrier.Init(ctx); err != nil {
		t.Fatal(err)
	}
	bridge := &GatewayBridge{svc: &Service{cfg: cfg, st: st, wg: wg.New(cfg, st, nil), stealth: carrier}, peerID: "node"}
	checkHello := func(h gatewayclient.Hello, wantBootstrap string) {
		t.Helper()
		if h.Endpoints.VLESSReality.BootstrapUUID != wantBootstrap {
			t.Fatal("hello reported an incorrect bootstrap credential")
		}
		b, err := json.Marshal(h)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte(`"bootstrap_uuid"`)) != (wantBootstrap != "") || bytes.Contains(b, []byte(carrier.Params().VLESSUUID)) {
			t.Fatal("hello exposed the wrong credentials")
		}
	}
	checkHello(bridge.BuildHello("node"), "")
	if err := carrier.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer carrier.Close()
	expected := carrier.Params()

	accepted := make(chan *websocket.Conn, 1)
	upgrader := websocket.Upgrader{}
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		accepted <- conn
	}))
	defer gw.Close()
	clientCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	client := gatewayclient.New(gw.URL, "node", "test-token", bridge, bridge, bridge.Status)
	go func() { defer close(done); client.Run(clientCtx) }()
	defer func() { cancel(); <-done }()
	var ws *websocket.Conn
	select {
	case ws = <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("gateway did not receive a connection")
	}
	defer ws.Close()
	read := func(kind string, dst any) {
		t.Helper()
		_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
		var frame gatewayclient.Envelope
		if err := ws.ReadJSON(&frame); err != nil {
			t.Fatal(err)
		}
		if frame.Type != kind {
			t.Fatalf("got %s, want %s", frame.Type, kind)
		}
		if err := json.Unmarshal(frame.Data, dst); err != nil {
			t.Fatal(err)
		}
	}
	var hello gatewayclient.Hello
	read(gatewayclient.TypeHello, &hello)
	checkHello(hello, expected.VLESSBootstrapUUID)
	if err := ws.WriteJSON(gatewayclient.Envelope{Type: gatewayclient.TypeCommand, Data: json.RawMessage(`{"action":"rotate_reality","request_id":"r1"}`)}); err != nil {
		t.Fatal(err)
	}
	var result gatewayclient.CommandResult
	read(gatewayclient.TypeCommandResult, &result)
	if !result.OK || result.RequestID != "r1" {
		t.Fatalf("rotation failed: %s", result.Error)
	}
	var refreshed gatewayclient.Hello
	read(gatewayclient.TypeHello, &refreshed)
	checkHello(refreshed, expected.VLESSBootstrapUUID)
	if len(refreshed.Endpoints.VLESSReality.ShortIDs) != 1 || refreshed.Endpoints.VLESSReality.ShortIDs[0] == expected.RealityShortID {
		t.Fatal("gateway retained a stale REALITY short ID")
	}
}
