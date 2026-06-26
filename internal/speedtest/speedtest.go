// Package speedtest measures node uplink throughput for gateway heartbeats.
package speedtest

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/NetSepio/erebrus/internal/gatewayclient"
)

const (
	refreshInterval = 6 * time.Hour
	downloadBytes   = 50_000_000
	uploadBytes     = 20_000_000
)

// Cache holds the latest measurement; heartbeats read without blocking on I/O.
type Cache struct {
	mu   sync.RWMutex
	last gatewayclient.Speedtest
}

func NewCache() *Cache { return &Cache{} }

func (c *Cache) Get() gatewayclient.Speedtest {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.last
}

func (c *Cache) set(st gatewayclient.Speedtest) {
	c.mu.Lock()
	c.last = st
	c.mu.Unlock()
}

// Start warms the cache once, then refreshes every six hours until ctx is done.
func (c *Cache) Start(ctx context.Context) {
	go func() {
		c.refresh(context.Background())
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.refresh(context.Background())
			}
		}
	}()
}

func (c *Cache) refresh(ctx context.Context) {
	st, err := Measure(ctx)
	if err != nil {
		slog.Warn("speedtest refresh failed", "err", err)
		return
	}
	c.set(st)
	slog.Info("speedtest refreshed",
		"download_mbps", st.DownloadMbps,
		"upload_mbps", st.UploadMbps,
		"latency_ms", st.LatencyMs,
	)
}

// Measure runs a Cloudflare HTTP speed test from the node (install.sh parity).
func Measure(ctx context.Context) (gatewayclient.Speedtest, error) {
	client := &http.Client{Timeout: 45 * time.Second}

	latencyMs := 0.0
	latReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://speed.cloudflare.com/__down?bytes=0",
		nil,
	)
	if err == nil {
		latStart := time.Now()
		if resp, err := client.Do(latReq); err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			latencyMs = float64(time.Since(latStart).Milliseconds())
		}
	}

	downloadMbps := 0.0
	downReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://speed.cloudflare.com/__down?bytes="+itoa(downloadBytes),
		nil,
	)
	if err == nil {
		start := time.Now()
		if resp, err := client.Do(downReq); err == nil {
			n, _ := io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			secs := time.Since(start).Seconds()
			if secs > 0 && n > 0 {
				downloadMbps = (float64(n) * 8) / (secs * 1_000_000)
			}
		}
	}

	uploadMbps := 0.0
	body := make([]byte, uploadBytes)
	upReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://speed.cloudflare.com/__up",
		bytes.NewReader(body),
	)
	if err == nil {
		start := time.Now()
		if resp, err := client.Do(upReq); err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			secs := time.Since(start).Seconds()
			if secs > 0 {
				uploadMbps = (float64(uploadBytes) * 8) / (secs * 1_000_000)
			}
		}
	}

	return gatewayclient.Speedtest{
		DownloadMbps: downloadMbps,
		UploadMbps:   uploadMbps,
		LatencyMs:    latencyMs,
		MeasuredAt:   time.Now().Unix(),
	}, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}