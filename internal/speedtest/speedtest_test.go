package speedtest

import (
	"context"
	"testing"
	"time"

	"github.com/NetSepio/erebrus/internal/gatewayclient"
)

func TestCacheGetSet(t *testing.T) {
	c := NewCache()
	want := gatewayclient.Speedtest{DownloadMbps: 12.5, UploadMbps: 8.2, LatencyMs: 42, MeasuredAt: 100}
	c.set(want)
	got := c.Get()
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestMeasureReturnsMeasuredAt(t *testing.T) {
	t.Skip("network integration test")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	st, err := Measure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.MeasuredAt <= 0 {
		t.Fatalf("expected measured_at, got %+v", st)
	}
}