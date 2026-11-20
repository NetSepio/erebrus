package gatewayclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// stubAttester stands in for the node's signer and remembers what the client
// asks it to keep.
type stubAttester struct {
	pending string
	nonce   string
}

func (s *stubAttester) Pending() bool          { return true }
func (s *stubAttester) PendingNonce() string   { return s.pending }
func (s *stubAttester) Queue(nonce string)     { s.pending = nonce }
func (s *stubAttester) Settled(string, string) { s.pending = "" }
func (s *stubAttester) SignRegistry(string) (string, string, error) {
	return "signature", s.nonce, nil
}
func (s *stubAttester) SignUptime(int64) (string, error) { return "signature", nil }
func (s *stubAttester) UptimeBucket(time.Time) int64     { return 0 }

// wsPair returns a connected client and server websocket.
func wsPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		accepted <- conn
	}))
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	server := <-accepted
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
		srv.Close()
	})
	return client, server
}

func readFrame(t *testing.T, ws *websocket.Conn) Envelope {
	t.Helper()
	if err := ws.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, raw, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("no frame arrived: %v", err)
	}
	var env Envelope
	if err = json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return env
}

// The gateway answers registry frames on the connection they arrive on, so a
// new session starts the per-connection state over. The registration itself is
// not per-connection: the gateway sponsors one paid write per nonce and allows
// an operator six a day. Asking again after a reconnect minted a second nonce,
// so the gateway paid twice for the same facts and a node that reconnected a
// few times exhausted the allowance and could then not register at all.
func TestReconnectResumesTheQueuedSponsorship(t *testing.T) {
	client, server := wsPair(t)
	c := New("https://gateway.example", "node", "token", nil, nil, nil)
	a := &stubAttester{nonce: "7f1a0c2e-6b3d-4f7a-9d1c-2a8e5b0f3c41"}
	c.SetAttester(a)

	// A node with nothing in flight asks for a challenge.
	c.resumeRegistry(client)
	if env := readFrame(t, server); env.Type != TypeRegistryChallenge {
		t.Fatalf("a fresh session sent %q", env.Type)
	}

	// Signing it spends the sponsorship, so the nonce is kept before any answer
	// can arrive — a disconnect in between must not look like a fresh start.
	c.onRegistryChallenge(client, json.RawMessage(`{"message":"challenge"}`))
	if env := readFrame(t, server); env.Type != TypeRegistryRegister {
		t.Fatalf("the signed proof was sent as %q", env.Type)
	}
	if a.PendingNonce() != a.nonce {
		t.Fatalf("the queued nonce was not kept: %q", a.PendingNonce())
	}

	// Reconnect. The job is the gateway's to finish, so the node asks after it.
	c.resumeRegistry(client)
	env := readFrame(t, server)
	if env.Type != TypeRegistryStatus {
		t.Fatalf("a reconnect sent %q, which asks the gateway to sponsor a second write", env.Type)
	}
	var status registryStatus
	if err := json.Unmarshal(env.Data, &status); err != nil {
		t.Fatal(err)
	}
	if status.Nonce != a.nonce {
		t.Fatalf("polled nonce %q, want the one already queued %q", status.Nonce, a.nonce)
	}

	// A settled result ends it; the session after that starts over.
	c.onRegistryResult(json.RawMessage(`{"state":"confirmed","tx_hash":"0xabc"}`))
	if a.PendingNonce() != "" {
		t.Fatalf("a confirmed registration left %q in flight", a.PendingNonce())
	}
	c.resumeRegistry(client)
	if env = readFrame(t, server); env.Type != TypeRegistryChallenge {
		t.Fatalf("with nothing in flight the node sent %q", env.Type)
	}

	// A gateway with no record of the job must not be polled for ever either.
	c.onRegistryChallenge(client, json.RawMessage(`{"message":"challenge"}`))
	readFrame(t, server)
	c.onRegistryResult(json.RawMessage(`{"state":"unavailable"}`))
	if a.PendingNonce() != "" {
		t.Fatalf("an unavailable registry left %q to be polled", a.PendingNonce())
	}
}
