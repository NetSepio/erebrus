package node

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/firewall"
	"github.com/NetSepio/erebrus/internal/gatewayclient"
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
