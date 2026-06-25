package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NetSepio/erebrus/internal/store"
)

func TestRegistryCRUD(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := &Registry{St: st}
	ctx := context.Background()

	svc, err := reg.Publish(ctx, Service{Name: "ollama", Port: 11434, Type: "ai.llm"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := reg.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v err=%v", list, err)
	}
	got, err := reg.Get(ctx, svc.ID)
	if err != nil || got.Name != "ollama" {
		t.Fatalf("get = %+v err=%v", got, err)
	}
	if err := reg.Remove(ctx, svc.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = reg.List(ctx)
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
	_ = os.RemoveAll(dir)
}
