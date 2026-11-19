package firewall

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/NetSepio/erebrus/internal/config"
)

func TestShieldUpstreamsDefaults(t *testing.T) {
	c := New(&config.Config{}, nil)
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
	c := New(&config.Config{ShieldUpstreamDNS: "9.9.9.9, 149.112.112.112"}, nil)
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
	}, testSettings{})
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

type testSettings map[string]string

func (s testSettings) GetSetting(_ context.Context, key string) (string, error) {
	return s[key], nil
}
func (s testSettings) SetSetting(_ context.Context, key, value string) error {
	s[key] = value
	return nil
}

func TestSyncShieldReconcilesSnapshot(t *testing.T) {
	stale := shieldRewrite{"old.example", "192.0.2.1"}
	alias := shieldRewrite{"internal.example", "2001:db8::1"}
	rewrites := []shieldRewrite{stale}
	var rules []string
	resolvers := []string{"192.0.2.53"}
	sets, adds, deletes, clears := 0, 0, 0, 0
	failAdd := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, _ := r.BasicAuth(); user != "admin" || pass != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		read := func(dst any) {
			if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
				t.Error(err)
			}
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /control/dns_info":
			json.NewEncoder(w).Encode(map[string]any{"upstream_dns": resolvers})
		case "POST /control/dns_config":
			var body struct {
				Upstreams []string `json:"upstream_dns"`
			}
			read(&body)
			resolvers = body.Upstreams
		case "POST /control/filtering/set_rules":
			var body struct {
				Rules []string `json:"rules"`
			}
			read(&body)
			if body.Rules == nil {
				t.Error("set_rules requires a JSON array")
			}
			rules = body.Rules
			sets++
		case "GET /control/rewrite/list":
			json.NewEncoder(w).Encode(rewrites)
		case "POST /control/rewrite/delete":
			var entry shieldRewrite
			read(&entry)
			rewrites = slices.DeleteFunc(rewrites, func(r shieldRewrite) bool { return r == entry })
			deletes++
		case "POST /control/rewrite/add":
			if failAdd {
				failAdd = false
				http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			var entry shieldRewrite
			read(&entry)
			rewrites = append(rewrites, entry)
			adds++
		case "POST /control/cache_clear":
			clears++
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	c := shieldClient(srv.URL)
	p := json.RawMessage(`{"policy_version":42,"upstreams":["https://resolver.example/dns-query"],"rules":[
  {"action":"block","target":"bad.example","enabled":true},
  {"action":"domain_block","target":"bad2.example","enabled":true},
  {"action":"block","target":"*.WILD.example.","enabled":true},
  {"action":"wildcard_domain_block","target":"*.wild2.example","enabled":true},
  {"action":"allow","target":"ok.example","enabled":true},
  {"action":"domain_allow","target":"ok2.example","enabled":true},
  {"action":"dns_rewrite","target":"internal.example","value":"2001:db8::1","enabled":true},
  {"action":"rewrite","target":"internal.example","value":"2001:db8::1","enabled":true},
  {"action":"block","target":"disabled.example","enabled":false}
 ]}`)
	if v, err := c.Sync(ctx, p); err == nil || v != nil || clears != 0 {
		t.Fatalf("partial sync returned version %v, error %v, clears %d", v, err, clears)
	}
	// Simulate a restart between attempts. Only the settings and AdGuard persist.
	c = New(c.cfg, c.settings)
	for i := 0; i < 2; i++ {
		if v, err := c.Sync(ctx, p); err != nil || v == nil || *v != 42 {
			t.Fatalf("retry returned version %v, error %v", v, err)
		}
	}
	want := []string{"||bad.example^", "||bad2.example^", "||wild.example^", "||wild2.example^", "@@||ok.example^", "@@||ok2.example^"}
	if !slices.Equal(rules, want) || !slices.Equal(rewrites, []shieldRewrite{alias}) || !slices.Equal(resolvers, []string{"https://resolver.example/dns-query"}) {
		t.Fatalf("got rules %v, rewrites %v, resolvers %v", rules, rewrites, resolvers)
	}
	if sets != 3 || adds != 1 || deletes != 1 || clears != 2 {
		t.Fatalf("sets=%d adds=%d deletes=%d clears=%d", sets, adds, deletes, clears)
	}

	if v, err := c.Sync(ctx, json.RawMessage(`{"rules":null,"upstreams":null}`)); err != nil || v != nil {
		t.Fatalf("legacy empty snapshot returned version %v, error %v", v, err)
	}
	if len(rules) != 0 || len(rewrites) != 0 || !slices.Equal(resolvers, []string{"192.0.2.53"}) || sets != 4 || clears != 3 {
		t.Fatalf("empty snapshot left rules %v, rewrites %v, resolvers %v, sets %d, clears %d", rules, rewrites, resolvers, sets, clears)
	}
}

func TestSyncShieldRejectsInvalidSnapshotsBeforeApplying(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected AdGuard request %s", r.URL.Path)
	}))
	defer srv.Close()
	c := shieldClient(srv.URL)
	for _, input := range []string{"", "null", "{}", `{"policy_version":1}`, `{"rules":"block"}`,
		`{"rules":[{"action":"block","enabled":true,"target":"bad.example^$important"}]}`,
		`{"rules":[{"action":"block","enabled":true,"target":"*.\nexample.com"}]}`,
		`{"rules":[{"action":"rewrite","enabled":true,"target":"host.example","value":"not-an-address"}]}`} {
		if v, err := c.Sync(context.Background(), json.RawMessage(input)); err == nil || v != nil {
			t.Errorf("Sync(%q) returned version %v, error %v", input, v, err)
		}
	}
}

func TestConfigureAdminSeedsBlocklistsOnFirstInstall(t *testing.T) {
	// Exercise both an empty installation and AdGuard's bundled ads filter.
	for _, test := range []struct {
		name    string
		bundled bool
	}{{"empty", false}, {"bundled", true}} {
		t.Run(test.name, func(t *testing.T) {
			const ads = "https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt"
			const malware = "https://adguardteam.github.io/HostlistsRegistry/assets/filter_11.txt"
			subscriptions := map[string]bool{}
			if test.bundled {
				subscriptions[ads] = true
			}
			installs, statusReads, additions := 0, 0, 0
			failMalware := true
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/control/install/") {
					if u, p, ok := r.BasicAuth(); !ok || u != "admin" || p != "secret" {
						t.Error("missing admin credentials")
						http.Error(w, "forbidden", 403)
						return
					}
				}
				switch r.Method + " " + r.URL.Path {
				case "GET /control/install/get_addresses":
					if installs > 0 {
						http.NotFound(w, r)
						return
					}
					fmt.Fprint(w, `{"web_port":3000}`)
				case "POST /control/install/configure":
					installs++
				case "GET /control/dns_info":
					fmt.Fprint(w, `{"upstream_dns":["192.0.2.53"]}`)
				case "GET /control/filtering/status":
					statusReads++
					var filters []map[string]string
					for url := range subscriptions {
						filters = append(filters, map[string]string{"url": url})
					}
					json.NewEncoder(w).Encode(map[string]any{"filters": filters})
				case "POST /control/filtering/add_url":
					var req struct {
						Name      string
						URL       string
						Whitelist bool
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
						http.Error(w, "invalid", 400)
						return
					}
					if subscriptions[req.URL] || req.Name == "" || req.Whitelist || (req.URL != ads && req.URL != malware) {
						t.Errorf("unexpected subscription %+v", req)
					}
					if req.URL == malware && failMalware {
						failMalware = false
						http.Error(w, "download unavailable", 502)
						return
					}
					additions++
					subscriptions[req.URL] = true
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			c := shieldClient(server.URL)
			if err := c.ConfigureAdmin(context.Background()); err == nil {
				t.Fatal("expected first download to fail")
			}
			c = New(c.cfg, c.settings)
			if err := c.ConfigureAdmin(context.Background()); err != nil {
				t.Fatal(err)
			}
			wantAdds := 2
			if test.bundled {
				wantAdds = 1
			}
			if !subscriptions[ads] || !subscriptions[malware] || additions != wantAdds {
				t.Fatalf("subscriptions %v, additions %d", subscriptions, additions)
			}
			// An operator can unsubscribe after setup; a restart must honor that.
			delete(subscriptions, malware)
			if err := c.ConfigureAdmin(context.Background()); err != nil {
				t.Fatal(err)
			}
			if subscriptions[malware] || installs != 1 || statusReads != 2 {
				t.Fatalf("installs=%d reads=%d subscriptions=%v", installs, statusReads, subscriptions)
			}
		})
	}
}
