package gatewayclient

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

// Registry frame types. The node always initiates; the gateway verifies the
// signature, submits the transaction and pays the gas.
const (
	TypeRegistryChallenge = "registry_challenge"
	TypeRegistryRegister  = "registry_register"
	TypeRegistryStatus    = "registry_status"
	TypeRegistryResult    = "registry_result"
	TypeUptimeAttestation = "uptime_attestation"
)

// Attester signs the registry challenges the gateway issues for this node.
// A nil Attester keeps the node out of the on-chain registry entirely: no
// frames are sent and nothing changes for the gateway, which simply never
// receives a registration for this peer.
type Attester interface {
	// Pending reports whether this node still needs to register. It is false
	// once a registration is confirmed for the current identity, so a node that
	// reconnects does not spend the operator's sponsorship budget again.
	Pending() bool
	// PendingNonce returns the nonce of a registration already asked for and
	// not yet settled, which a reconnecting node resumes instead of replacing.
	PendingNonce() string
	// Queue records the nonce a registration went out under; "" forgets it.
	Queue(nonce string)
	// SignRegistry returns the signature and the challenge nonce.
	SignRegistry(message string) (signature, nonce string, err error)
	// Settled records a terminal gateway result (confirmed or failed).
	Settled(state, txHash string)
	SignUptime(bucket int64) (string, error)
	// UptimeBucket returns the bucket the gateway accepts at now.
	UptimeBucket(now time.Time) int64
}

type registryChallenge struct {
	Message string `json:"message"`
}

type registryRegistration struct {
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

type registryStatus struct {
	Nonce string `json:"nonce"`
}

type registryResult struct {
	State string `json:"state"`
	Hash  string `json:"tx_hash,omitempty"`
}

type uptimeAttestation struct {
	Bucket    int64  `json:"bucket"`
	Signature string `json:"signature"`
}

// SetAttester enables the node's half of the sponsored registry. Call before Run.
func (c *Client) SetAttester(a Attester) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attester = a
}

func (c *Client) registryAttester() Attester {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.attester
}

// requestRegistryChallenge asks the gateway for the message to sign. Failures
// are never fatal to the session: the registry is optional and a node that
// cannot register must keep serving VPN traffic.
func (c *Client) requestRegistryChallenge(ws *websocket.Conn) {
	a := c.registryAttester()
	if a == nil || !a.Pending() {
		return
	}
	frame, err := wrap(TypeRegistryChallenge, struct{}{})
	if err != nil {
		return
	}
	if err := c.writeMessage(ws, websocket.TextMessage, frame); err != nil {
		c.log.Debug("registry challenge not sent", "err", err)
	}
}

// resumeRegistry picks up where the last session left off. The gateway answers
// registry frames on the connection they arrive on, so the per-session state is
// reset — but a registration it has already been asked to sponsor stays its to
// finish. Starting over mints a second nonce, which the gateway sponsors as a
// separate paid job and charges to the operator's daily allowance of six, so a
// handful of reconnects could spend the budget on one node's registration and
// then leave it unable to register at all.
func (c *Client) resumeRegistry(ws *websocket.Conn) {
	a := c.registryAttester()
	c.mu.Lock()
	c.uptimeBucket, c.registryNonce = 0, ""
	if a != nil {
		c.registryNonce = a.PendingNonce()
	}
	queued := c.registryNonce
	c.mu.Unlock()
	if queued != "" {
		c.pollRegistry(ws)
		return
	}
	c.requestRegistryChallenge(ws)
}

// onRegistryChallenge verifies the challenge against this node's own identity
// before signing it, then returns the proof.
func (c *Client) onRegistryChallenge(ws *websocket.Conn, data json.RawMessage) {
	a := c.registryAttester()
	if a == nil {
		return
	}
	var challenge registryChallenge
	if err := json.Unmarshal(data, &challenge); err != nil {
		return
	}
	signature, nonce, err := a.SignRegistry(challenge.Message)
	if err != nil {
		// A challenge that does not describe this node is refused, not signed.
		c.log.Warn("registry challenge refused", "err", err)
		return
	}
	frame, err := wrap(TypeRegistryRegister, registryRegistration{Message: challenge.Message, Signature: signature})
	if err != nil {
		return
	}
	if err := c.writeMessage(ws, websocket.TextMessage, frame); err != nil {
		c.log.Debug("registry registration not sent", "err", err)
		return
	}
	c.mu.Lock()
	c.registryNonce = nonce
	c.mu.Unlock()
	// Persisted before any answer comes back: the sponsorship is spent the
	// moment the gateway accepts this frame, so a disconnect in between must
	// not leave the node ready to ask for another one.
	a.Queue(nonce)
}

func (c *Client) onRegistryResult(data json.RawMessage) {
	a := c.registryAttester()
	if a == nil {
		return
	}
	var result registryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return
	}
	switch result.State {
	case "confirmed", "failed":
		c.mu.Lock()
		c.registryNonce = ""
		c.mu.Unlock()
		a.Settled(result.State, result.Hash)
		c.log.Info("node registry write settled", "state", result.State, "tx", result.Hash)
	case "queued":
		c.log.Debug("node registry write queued")
	case "deferred":
		// The gateway could not look the job up, which says nothing about
		// whether it exists. Keep the nonce and ask again: forgetting it here
		// had a reconnect request a second sponsored write for a registration
		// already queued, and spend another of the operator's daily allowance.
		c.log.Debug("node registry lookup deferred")
	default:
		// The gateway has no enabled registry contract, this node's record is
		// not complete enough to register, or it has no record of the nonce we
		// asked about. Nothing was sponsored in any of those cases, so the
		// queued nonce is dropped rather than resumed for ever, and the node
		// stops asking for this session.
		c.mu.Lock()
		c.registryNonce = ""
		c.mu.Unlock()
		a.Queue("")
		c.log.Debug("node registry unavailable", "state", result.State)
	}
}

// pollRegistry re-asks for the outcome of a queued sponsorship.
func (c *Client) pollRegistry(ws *websocket.Conn) {
	c.mu.Lock()
	nonce := c.registryNonce
	c.mu.Unlock()
	if nonce == "" {
		return
	}
	frame, err := wrap(TypeRegistryStatus, registryStatus{Nonce: nonce})
	if err != nil {
		return
	}
	_ = c.writeMessage(ws, websocket.TextMessage, frame)
}

// attestUptime signs the current bucket once. The gateway only accepts the
// bucket that is open when the frame arrives, so this runs on a short tick
// rather than once per bucket boundary.
func (c *Client) attestUptime(ws *websocket.Conn) {
	a := c.registryAttester()
	if a == nil {
		return
	}
	bucket := a.UptimeBucket(time.Now())
	c.mu.Lock()
	already := c.uptimeBucket == bucket
	if !already {
		c.uptimeBucket = bucket
	}
	c.mu.Unlock()
	if already {
		return
	}
	signature, err := a.SignUptime(bucket)
	if err != nil {
		c.log.Debug("uptime attestation not signed", "err", err)
		return
	}
	frame, err := wrap(TypeUptimeAttestation, uptimeAttestation{Bucket: bucket, Signature: signature})
	if err != nil {
		return
	}
	_ = c.writeMessage(ws, websocket.TextMessage, frame)
}
