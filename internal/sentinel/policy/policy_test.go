package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriterApplyWildcardBlock(t *testing.T) {
	dir := t.TempDir()
	w := &Writer{Dir: dir}
	err := w.Apply(Policy{
		Rules: []Rule{{RuleType: "wildcard_domain_block", Target: "*.ads.example.com", Action: "block", Enabled: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dir + "/blocklist.conf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ads.example.com.") {
		t.Fatalf("blocklist = %q", body)
	}
}

func TestWriterApplyBlockRule(t *testing.T) {
	dir := t.TempDir()
	w := &Writer{Dir: dir}
	err := w.Apply(Policy{
		Rules: []Rule{{RuleType: "domain", Target: "ads.example.com", Action: "block", Enabled: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dir + "/blocklist.conf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ads.example.com.") {
		t.Fatalf("blocklist = %q", body)
	}
}

func TestWriterRejectsConfigInjection(t *testing.T) {
	w := &Writer{Dir: t.TempDir()}
	err := w.Apply(Policy{Rules: []Rule{{
		Target: "example.com\nserver:", Action: "block", Enabled: true,
	}}})
	if err == nil {
		t.Fatal("expected injected domain to be rejected")
	}
}

func TestWriterRejectsInvalidRewrite(t *testing.T) {
	w := &Writer{Dir: t.TempDir()}
	err := w.Apply(Policy{Rules: []Rule{{
		Target: "example.com", Action: "rewrite", Value: "not-an-ip", Enabled: true,
	}}})
	if err == nil {
		t.Fatal("expected invalid rewrite to be rejected")
	}
}

func TestWriterRendersIPv6RewriteAsAAAA(t *testing.T) {
	dir := t.TempDir()
	w := &Writer{Dir: dir}
	if err := w.Apply(Policy{Rules: []Rule{{
		Target: "example.com", Action: "rewrite", Value: "2001:db8::1", Enabled: true,
	}}}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "rewrites.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "IN AAAA 2001:db8::1") {
		t.Fatalf("rewrites = %q", body)
	}
}
