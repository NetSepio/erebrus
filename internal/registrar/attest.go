package registrar

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/internal/wallet"
	"github.com/google/uuid"
)

// Frozen with erebrus-gateway/docs/ws-protocol.md and the field order built by
// store.registryIdentity. Change the document first, then both repos.
const (
	registryDomain = "erebrus.registry.v1"
	uptimeDomain   = "erebrus.uptime.v1"

	// ["erebrus.registry.v1",chain,registryAddress,peerId,did,ipHash,region,operator,nonce,expiresUnix]
	proofDomain, proofChain, proofRegistry, proofPeer = 0, 1, 2, 3
	proofDID, proofIPHash, proofRegion, proofOperator = 4, 5, 6, 7
	proofNonce, proofExpires                          = 8, 9
	proofFields                                       = 10

	// The gateway issues five-minute deadlines; allow a little clock skew and
	// refuse anything that would stay signable for longer.
	maxProofWindow = 10 * time.Minute

	// UptimeBucketSeconds matches the gateway's floor(epoch/300)*300 bucket.
	UptimeBucketSeconds = 300
)

// Attester signs the registry challenges the gateway issues for this node.
//
// The gateway builds, submits and pays for the transaction; the node only
// consents, so this key never needs an on-chain balance. It is also the key
// that anchors the node's identity on chain, which is why SignRegistry
// reconstructs every identity field before signing instead of trusting the
// bytes it was handed — a node must never blind-sign for its own wallet.
type Attester struct {
	id       NodeIdentity
	chain    string // sol | evm
	mnemonic string
}

// NewAttester derives the operator wallet for chain and binds it to id.
func NewAttester(id NodeIdentity, chain, mnemonic string) (*Attester, error) {
	resolved := wallet.ResolveChain(chain)
	operator, err := wallet.AddressFromMnemonic(mnemonic, resolved)
	if err != nil {
		return nil, fmt.Errorf("registry operator wallet: %w", err)
	}
	if id.PeerID == "" || id.DID == "" || id.IPHash == "" {
		return nil, errors.New("registry identity is incomplete")
	}
	// The gateway stores the region upper-cased and accepts only a two-letter
	// code, so a node left on the default REGION can never be registered.
	if len(id.Region) != 2 {
		return nil, fmt.Errorf("REGION %q must be a two-letter country code for chain registration", id.Region)
	}
	id.Wallet = operator
	return &Attester{id: id, chain: resolved, mnemonic: mnemonic}, nil
}

// Operator returns the wallet recorded on chain for this node.
func (a *Attester) Operator() string { return a.id.Wallet }

// SignRegistry verifies that the gateway asked this node to attest to its own
// identity, then signs the challenge verbatim. It returns the signature and the
// challenge nonce, which identifies the sponsored job.
func (a *Attester) SignRegistry(message string) (signature, nonce string, err error) {
	var f []string
	if err = json.Unmarshal([]byte(message), &f); err != nil {
		return "", "", fmt.Errorf("registry challenge is not a field list: %w", err)
	}
	if len(f) != proofFields {
		return "", "", fmt.Errorf("registry challenge has %d fields, want %d", len(f), proofFields)
	}
	for _, check := range []struct{ name, got, want string }{
		{"domain", f[proofDomain], registryDomain},
		{"chain", f[proofChain], a.chain},
		{"peer id", f[proofPeer], a.id.PeerID},
		{"DID", f[proofDID], a.id.DID},
		{"IP hash", f[proofIPHash], a.id.IPHash},
		{"region", f[proofRegion], strings.ToUpper(a.id.Region)},
	} {
		if check.got != check.want {
			return "", "", fmt.Errorf("registry challenge %s %q is not this node's %q", check.name, check.got, check.want)
		}
	}
	// EVM addresses differ only by checksum case between the gateway and here.
	if !strings.EqualFold(f[proofOperator], a.id.Wallet) {
		return "", "", fmt.Errorf("registry challenge operator %q is not this node's wallet", f[proofOperator])
	}
	if strings.TrimSpace(f[proofRegistry]) == "" {
		return "", "", errors.New("registry challenge names no contract")
	}
	if _, err = uuid.Parse(f[proofNonce]); err != nil {
		return "", "", fmt.Errorf("registry challenge nonce: %w", err)
	}
	expires, err := strconv.ParseInt(f[proofExpires], 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("registry challenge deadline: %w", err)
	}
	now := time.Now()
	if expires <= now.Unix() || expires > now.Add(maxProofWindow).Unix() {
		return "", "", fmt.Errorf("registry challenge deadline %d is outside the signing window", expires)
	}
	// Sign the bytes as received: the gateway rebuilds this exact string from
	// its own record and compares before it will sponsor anything.
	if _, _, signature, err = wallet.SignChallengeWithMnemonic(a.mnemonic, a.chain, message); err != nil {
		return "", "", err
	}
	return signature, f[proofNonce], nil
}

// SignUptime attests that this node was online during bucket, which the gateway
// folds into the epoch merkle root it publishes.
func (a *Attester) SignUptime(bucket int64) (string, error) {
	if bucket <= 0 || bucket%UptimeBucketSeconds != 0 {
		return "", fmt.Errorf("uptime bucket %d is not a %ds boundary", bucket, UptimeBucketSeconds)
	}
	message, err := json.Marshal([]string{
		uptimeDomain, a.chain, a.id.PeerID, a.id.Wallet, strconv.FormatInt(bucket, 10),
	})
	if err != nil {
		return "", err
	}
	_, _, signature, err := wallet.SignChallengeWithMnemonic(a.mnemonic, a.chain, string(message))
	return signature, err
}

// CurrentUptimeBucket returns the bucket the gateway will accept right now.
func CurrentUptimeBucket(now time.Time) int64 {
	return now.Unix() / UptimeBucketSeconds * UptimeBucketSeconds
}
