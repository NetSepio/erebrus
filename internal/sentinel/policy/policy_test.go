package policy

import (
	"os"
	"strings"
	"testing"
)

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