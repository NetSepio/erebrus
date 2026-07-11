package drop

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/NetSepio/erebrus/internal/config"
)

func TestServiceRejectsOversizeAndFullReservationsBeforeRPC(t *testing.T) {
	cfg := config.Load()
	cfg.DropEnabled = true
	cfg.DropStorageMaxBytes = 2_000_000_000
	service := NewService(cfg, nil)

	_, err := service.Upload(context.Background(), AddRequest{
		Body: strings.NewReader("x"), DeclaredSize: MaxObjectBytes + 1,
	})
	if !errors.Is(err, ErrByteLimit) {
		t.Fatalf("oversize error = %v", err)
	}

	service.setSnapshot(Snapshot{
		State: "active", RepoSizeBytes: 1_500_000_000, StorageMaxBytes: 2_000_000_000,
	})
	_, err = service.Upload(context.Background(), AddRequest{
		Body: strings.NewReader("x"), DeclaredSize: 600_000_000,
	})
	if !errors.Is(err, ErrStorageFull) {
		t.Fatalf("capacity error = %v", err)
	}
}
