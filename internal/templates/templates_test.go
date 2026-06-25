package templates

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NetSepio/erebrus/internal/services"
	"github.com/NetSepio/erebrus/internal/store"
)

func TestInstallOllama(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "tpl.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := &services.Registry{St: st}
	svc, err := Install(context.Background(), reg, "ollama")
	if err != nil {
		t.Fatal(err)
	}
	if svc.Port != 11434 || svc.Type != "ai.llm" {
		t.Fatalf("svc = %+v", svc)
	}
}
