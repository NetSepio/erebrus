package stealth

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

// A rotation that cannot bind must stay failed and stay retryable. The sing-box
// handle is nil both when the carrier is deliberately down and when a start
// failed, so a restart gated on the handle alone reported success and skipped
// every later attempt: the node then advertised a rotated short-id for a
// carrier that was not listening at all, and only a process restart fixed it.
func TestRotateRealityRecoversAfterAFailedRestart(t *testing.T) {
	ctx := context.Background()
	vp, hp := freePort(t), freePort(t)
	m := New(testConfig(vp, hp), newMemStore())
	if err := m.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer m.Close()
	waitListening(t, vp).Close()

	// Take the carrier's own address while it is down mid-rotation, so the
	// restart cannot bind. It has to be the wildcard address sing-box listens
	// on: a loopback-only listener does not conflict with it.
	m.mu.Lock()
	err := m.close()
	m.mu.Unlock()
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	squatter, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(vp)))
	if err != nil {
		t.Fatalf("could not occupy the carrier port: %v", err)
	}
	if _, err = m.RotateReality(ctx); err == nil {
		squatter.Close()
		t.Fatal("rotation reported success while the carrier port was taken")
	}

	// With the port free again, a retry must actually bring the carrier back.
	if err = squatter.Close(); err != nil {
		t.Fatalf("releasing the port: %v", err)
	}
	shortID, err := m.RotateReality(ctx)
	if err != nil {
		t.Fatalf("rotation did not recover once the port was free: %v", err)
	}
	if shortID == "" {
		t.Fatal("recovered rotation returned no short-id")
	}
	waitListening(t, vp).Close()

	// And the credentials the gateway is told about describe the live listener.
	if got := m.ActiveParams(); got.RealityShortID != shortID {
		t.Fatalf("advertised short-id %q, listener was rotated to %q", got.RealityShortID, shortID)
	}
}

// A deliberate Close must not leave the carrier marked as wanted, or a later
// rotation would quietly start a listener the operator had stopped.
func TestRotateAfterCloseStaysDown(t *testing.T) {
	ctx := context.Background()
	vp, hp := freePort(t), freePort(t)
	m := New(testConfig(vp, hp), newMemStore())
	if err := m.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitListening(t, vp).Close()
	if err := m.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := m.RotateReality(ctx); err != nil {
		t.Fatalf("rotating a stopped carrier: %v", err)
	}
	if conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(vp)), 200*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("rotation restarted a carrier that had been deliberately stopped")
	}
}

// waitListening dials the carrier until it answers, so assertions are about the
// listener rather than about start-up timing.
func waitListening(t *testing.T, port int) net.Conn {
	t.Helper()
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	var conn net.Conn
	var err error
	for range 50 {
		if conn, err = net.DialTimeout("tcp", addr, 500*time.Millisecond); err == nil {
			return conn
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("carrier not listening on %s: %v", addr, err)
	return nil
}
