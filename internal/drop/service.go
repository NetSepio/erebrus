package drop

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/telemetry"
)

const (
	// DefaultKuboRPCURL is internal to the Compose network and is never published.
	DefaultKuboRPCURL = "http://kubo:5001"
	// MaxObjectBytes is the v1 per-object stream limit.
	MaxObjectBytes int64 = 1_000_000_000
)

var (
	ErrDisabled    = errors.New("Drop is disabled")
	ErrStorageFull = errors.New("Drop storage reservation exceeds capacity")
)

// Snapshot is the current gateway-private Drop health and capacity report.
type Snapshot struct {
	State           string `json:"state"`
	KuboVersion     string `json:"kubo_version,omitempty"`
	RepoSizeBytes   int64  `json:"repo_size_bytes"`
	StorageMaxBytes int64  `json:"storage_max_bytes"`
	NumObjects      int64  `json:"num_objects"`
}

// Service owns Drop health, identity initialization, and bounded Kubo operations.
type Service struct {
	cfg     *config.Config
	client  *Client
	metrics *telemetry.Metrics

	mu       sync.RWMutex
	snapshot Snapshot
}

// NewService creates the optional Drop runtime.
func NewService(cfg *config.Config, metrics *telemetry.Metrics) *Service {
	state := "disabled"
	if cfg.DropEnabled {
		state = "starting"
	}
	return &Service{
		cfg: cfg, client: NewClient(DefaultKuboRPCURL), metrics: metrics,
		snapshot: Snapshot{State: state, StorageMaxBytes: cfg.DropStorageMaxBytes},
	}
}

// Start initializes the deterministic Kubo identity and begins health polling.
func (s *Service) Start(ctx context.Context) error {
	if !s.cfg.DropEnabled {
		return nil
	}
	if err := PrepareKuboIdentity(DefaultKuboRepoPath, s.cfg.Mnemonic); err != nil {
		s.setSnapshot(Snapshot{State: "degraded", StorageMaxBytes: s.cfg.DropStorageMaxBytes})
		return err
	}
	go s.poll(ctx)
	return nil
}

// Snapshot returns the latest immutable status.
func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

// Enabled reports the operator's Drop setting.
func (s *Service) Enabled() bool { return s.cfg.DropEnabled }

// AcceptsPublicUploads reports the stable public-storage capability.
func (s *Service) AcceptsPublicUploads() bool { return s.cfg.DropAcceptsPublicUploads() }

// WebUIAvailable reports whether the exact-purpose Kubo proxy may be used.
func (s *Service) WebUIAvailable() bool { return s.cfg.DropWebUIAvailable() }

// Upload streams, pins, and verifies one reserved object.
func (s *Service) Upload(ctx context.Context, in AddRequest) (AddResult, error) {
	if !s.cfg.DropEnabled {
		return AddResult{}, ErrDisabled
	}
	if in.DeclaredSize > MaxObjectBytes {
		return AddResult{}, ErrByteLimit
	}
	snapshot := s.Snapshot()
	if snapshot.RepoSizeBytes >= snapshot.StorageMaxBytes ||
		in.DeclaredSize > snapshot.StorageMaxBytes-snapshot.RepoSizeBytes {
		return AddResult{}, ErrStorageFull
	}
	if in.MaxBytes <= 0 || in.MaxBytes > MaxObjectBytes {
		in.MaxBytes = MaxObjectBytes
	}
	result, err := s.client.AddAndPin(ctx, in)
	s.observeOperation("upload", err)
	if err == nil && s.metrics != nil {
		s.metrics.DropUploads.WithLabelValues("success", "node").Inc()
		s.metrics.DropUploadBytes.WithLabelValues("node").Add(float64(result.Size))
	} else if err != nil && s.metrics != nil {
		s.metrics.DropUploads.WithLabelValues("error", "node").Inc()
	}
	return result, err
}

// Read streams one object with the v1 object byte limit.
func (s *Service) Read(ctx context.Context, value string) (io.ReadCloser, error) {
	if !s.cfg.DropEnabled {
		return nil, ErrDisabled
	}
	body, err := s.client.Cat(ctx, value, MaxObjectBytes)
	s.observeOperation("read", err)
	return body, err
}

// PinStatus checks recursive pin state.
func (s *Service) PinStatus(ctx context.Context, value string) (bool, error) {
	if !s.cfg.DropEnabled {
		return false, ErrDisabled
	}
	pinned, err := s.client.PinStatus(ctx, value)
	s.observeOperation("pin_check", err)
	return pinned, err
}

// Unpin removes a recursive pin.
func (s *Service) Unpin(ctx context.Context, value string) error {
	if !s.cfg.DropEnabled {
		return ErrDisabled
	}
	err := s.client.Unpin(ctx, value)
	s.observeOperation("unpin", err)
	return err
}

// RecordDownload accounts for bytes successfully streamed to a gateway caller.
func (s *Service) RecordDownload(size int64) {
	if s.metrics != nil && size > 0 {
		s.metrics.DropDownloadBytes.WithLabelValues("node").Add(float64(size))
	}
}

func (s *Service) poll(ctx context.Context) {
	s.refresh(ctx)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(ctx)
		}
	}
}

func (s *Service) refresh(ctx context.Context) {
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	version, err := s.client.Version(requestCtx)
	if err != nil {
		s.setSnapshot(Snapshot{State: "unreachable", StorageMaxBytes: s.cfg.DropStorageMaxBytes})
		return
	}
	stats, err := s.client.RepoStats(requestCtx)
	if err != nil {
		s.setSnapshot(Snapshot{
			State: "degraded", KuboVersion: version, StorageMaxBytes: s.cfg.DropStorageMaxBytes,
		})
		return
	}
	state := "active"
	if stats.RepoSize >= s.cfg.DropStorageMaxBytes {
		state = "full"
	}
	s.setSnapshot(Snapshot{
		State: state, KuboVersion: version, RepoSizeBytes: stats.RepoSize,
		StorageMaxBytes: s.cfg.DropStorageMaxBytes, NumObjects: stats.NumObjects,
	})
}

func (s *Service) setSnapshot(snapshot Snapshot) {
	s.mu.Lock()
	s.snapshot = snapshot
	s.mu.Unlock()
}

func (s *Service) observeOperation(operation string, err error) {
	if s.metrics == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	s.metrics.DropNodeOperations.WithLabelValues(operation, result).Inc()
}
