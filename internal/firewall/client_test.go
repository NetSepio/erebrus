package firewall

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NetSepio/erebrus/internal/config"
)

func TestShieldUpstreamsDefaults(t *testing.T) {
	c := New(&config.Config{})
	got := c.shieldUpstreams()
	want := []string{"1.1.1.1", "1.0.0.1"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestShieldUpstreamsFromEnv(t *testing.T) {
	c := New(&config.Config{ShieldUpstreamDNS: "9.9.9.9, 149.112.112.112"})
	got := c.shieldUpstreams()
	want := []string{"9.9.9.9", "149.112.112.112"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestIsStockUpstreams(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want bool
	}{
		{"missing", nil, true},
		{"empty", []any{}, true},
		{"stock quad9 doh", []any{"https://dns10.quad9.net/dns-query"}, true},
		{"operator custom", []any{"9.9.9.9"}, false},
		{"mixed", []any{"https://dns10.quad9.net/dns-query", "8.8.8.8"}, false},
	}
	for _, tc := range cases {
		if got := isStockUpstreams(tc.in); got != tc.want {
			t.Errorf("%s: isStockUpstreams(%v) = %v, want %v", tc.name, tc.in, got, tc.want)
		}
	}
}

// fakeAdGuard is a minimal AdGuard admin API requiring admin/secret basic auth.
func fakeAdGuard(t *testing.T, upstreams []string, gotConfig *[]string, calls map[string]*int) *httptest.Server {
	t.Helper()
	requireAuth := func(w http.ResponseWriter, r *http.Request) bool {
		if u, p, ok := r.BasicAuth(); !ok || u != "admin" || p != "secret" {
			http.Error(w, "unauthorized", http.StatusForbidden)
			return false
		}
		return true
	}
	count := func(path string) {
		if n, ok := calls[path]; ok {
			*n++
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/control/install/configure", func(w http.ResponseWriter, r *http.Request) {
		count(r.URL.Path)
		http.Error(w, "already configured", http.StatusForbidden)
	})
	mux.HandleFunc("/control/dns_info", func(w http.ResponseWriter, r *http.Request) {
		count(r.URL.Path)
		if !requireAuth(w, r) {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"upstream_dns":  upstreams,
			"upstream_mode": "load_balance",
		})
	})
	mux.HandleFunc("/control/dns_config", func(w http.ResponseWriter, r *http.Request) {
		count(r.URL.Path)
		if !requireAuth(w, r) {
			return
		}
		var body struct {
			UpstreamDNS  []string `json:"upstream_dns"`
			UpstreamMode string   `json:"upstream_mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.UpstreamMode != "load_balance" {
			t.Errorf("dns_config dropped sibling field upstream_mode: %q", body.UpstreamMode)
		}
		*gotConfig = body.UpstreamDNS
	})
	mux.HandleFunc("/control/cache_clear", func(w http.ResponseWriter, r *http.Request) {
		count(r.URL.Path)
		requireAuth(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func shieldClient(url string) *Client {
	return New(&config.Config{
		FirewallProvider:    config.FirewallAdGuardHome,
		ShieldAdminURL:      url,
		ShieldAdminUser:     "admin",
		ShieldAdminPassword: "secret",
	})
}

func TestConfigureAdminReplacesStockUpstreams(t *testing.T) {
	var got []string
	srv := fakeAdGuard(t, []string{"https://dns10.quad9.net/dns-query"}, &got, nil)

	if err := shieldClient(srv.URL).ConfigureAdmin(context.Background()); err != nil {
		t.Fatalf("ConfigureAdmin: %v", err)
	}
	want := []string{"1.1.1.1", "1.0.0.1"}
	if len(got) != len(want) {
		t.Fatalf("upstreams = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("upstreams = %v, want %v", got, want)
		}
	}
}

func TestConfigureAdminKeepsOperatorUpstreams(t *testing.T) {
	var got []string
	configured := 0
	srv := fakeAdGuard(t, []string{"9.9.9.9"}, &got, map[string]*int{"/control/dns_config": &configured})

	if err := shieldClient(srv.URL).ConfigureAdmin(context.Background()); err != nil {
		t.Fatalf("ConfigureAdmin: %v", err)
	}
	if configured != 0 {
		t.Fatalf("dns_config called %d times for operator-set upstreams, want 0", configured)
	}
}

func TestSyncShieldOnlyClearsCache(t *testing.T) {
	var got []string
	configured, cleared := 0, 0
	srv := fakeAdGuard(t, []string{"https://dns10.quad9.net/dns-query"}, &got,
		map[string]*int{"/control/dns_config": &configured, "/control/cache_clear": &cleared})

	raw := json.RawMessage(`{"licensed":true,"upstreams":["8.8.8.8"]}`)
	if err := shieldClient(srv.URL).Sync(context.Background(), raw); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if cleared != 1 {
		t.Fatalf("cache_clear called %d times, want 1", cleared)
	}
	if configured != 0 {
		t.Fatalf("dns_config called %d times from Sync, want 0", configured)
	}
}
