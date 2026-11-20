package stealth

import (
	"context"
	"slices"
	"testing"
	"time"
)

// listenerIdentities reports what the listener would be built with right now:
// the accepted REALITY short-ids and VLESS UUIDs.
func (m *Manager) listenerIdentities() (shortIDs, users []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	shortIDs = append(shortIDs, m.realityShortIDs()...)
	for _, u := range m.vlessUsers() {
		users = append(users, u.UUID)
	}
	return shortIDs, users
}

func (m *Manager) liveInstance() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.instance
}

// The grace window is a promise about the running listener, not a note in the
// settings table. Dropping the retired descriptor only when something else
// rebuilds the carrier left it accepted until the next rotation — days past the
// deadline it was retired with, and days after the operator was told it was
// gone.
func TestRetiredDescriptorExpiresOnTheRunningListener(t *testing.T) {
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

	retired, retiredShortID := m.Params().VLESSBootstrapUUID, m.Params().RealityShortID
	// Deadlines are kept to the second, so the window has to be worth seconds.
	if err := m.RotateAllSecrets(ctx, 2*time.Second); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	live := m.liveInstance()

	shortIDs, users := m.listenerIdentities()
	if !slices.Contains(shortIDs, retiredShortID) || !slices.Contains(users, retired) {
		t.Fatalf("inside the window the listener offered short-ids %v and users %v, want the retired %q/%q", shortIDs, users, retired, retiredShortID)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && m.liveInstance() == live {
		time.Sleep(20 * time.Millisecond)
	}
	if m.liveInstance() == live {
		t.Fatal("the carrier was never rebuilt, so the retired descriptor outlived its window")
	}
	shortIDs, users = m.listenerIdentities()
	if slices.Contains(shortIDs, retiredShortID) || slices.Contains(users, retired) {
		t.Fatalf("past the window the listener still offered short-ids %v and users %v", shortIDs, users)
	}
	// Expiry rebuilds the carrier; it must not take it down.
	waitListening(t, vp).Close()
}
