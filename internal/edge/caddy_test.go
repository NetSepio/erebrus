package edge

import (
	"strings"
	"testing"

	"github.com/NetSepio/erebrus/internal/services"
)

func TestGenerateCaddyfile(t *testing.T) {
	out := GenerateCaddyfile([]services.Service{
		{Name: "dashboard", Public: true, PublicHost: "dashboard.apps.example.com", InternalAddr: "127.0.0.1:3000"},
	}, CaddyOptions{AutoTLS: true})
	if !strings.Contains(out, "dashboard.apps.example.com") {
		t.Fatalf("missing host: %s", out)
	}
	if !strings.Contains(out, "reverse_proxy") {
		t.Fatal("missing reverse_proxy")
	}
}
