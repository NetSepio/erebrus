package node

import (
	"path/filepath"
	"testing"

	"github.com/NetSepio/erebrus/internal/config"
	"github.com/NetSepio/erebrus/internal/store"
)

// A throwaway BIP39 mnemonic; this key only signs consent proofs in tests.
const nodeTestMnemonic = "legal winner thank year wave sausage worth useful legal winner thank yellow"

func testAttester(t *testing.T, st *store.Store, peer string) *Attester {
	t.Helper()
	a, err := NewAttester(&config.Config{
		ChainRegistration: "on", WalletChain: "SOLANA", Mnemonic: nodeTestMnemonic,
		Region: "de", WGEndpointHost: "203.0.113.9",
	}, st, peer, "did:erebrus:"+peer)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("chain registration is on, but no attester was built")
	}
	return a
}

// The gateway sponsors one paid registry write per nonce, six a day per
// operator, so the nonce of a registration in flight has to outlive both the
// connection and the process: a node that forgets it asks for another one.
func TestQueuedSponsorshipSurvivesARestart(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "node.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	const nonce = "7f1a0c2e-6b3d-4f7a-9d1c-2a8e5b0f3c41"
	testAttester(t, st, "peer-1").Queue(nonce)

	// A new process, same node: the registration is still the gateway's.
	restarted := testAttester(t, st, "peer-1")
	if got := restarted.PendingNonce(); got != nonce {
		t.Fatalf("after a restart the queued nonce is %q, want %q", got, nonce)
	}
	if !restarted.Pending() {
		t.Fatal("a queued registration is not confirmed, so it is still pending")
	}

	// Another node's record is not this node's to resume.
	if got := testAttester(t, st, "peer-2").PendingNonce(); got != "" {
		t.Fatalf("a different identity inherited the nonce %q", got)
	}

	// Settling ends it, for this process and the next.
	restarted.Settled("confirmed", "0xabc")
	if got := restarted.PendingNonce(); got != "" {
		t.Fatalf("a confirmed registration still reports %q in flight", got)
	}
	done := testAttester(t, st, "peer-1")
	if done.PendingNonce() != "" || done.Pending() {
		t.Fatalf("a restart after confirmation reports nonce %q pending %t", done.PendingNonce(), done.Pending())
	}

	// A failed write is retried, but not by polling the job that failed.
	done.Settled("failed", "")
	retry := testAttester(t, st, "peer-1")
	if !retry.Pending() || retry.PendingNonce() != "" {
		t.Fatalf("a failed registration reports nonce %q pending %t", retry.PendingNonce(), retry.Pending())
	}
}
