package registrar

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetSepio/erebrus/internal/wallet"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/mr-tron/base58"
)

// verifyAsGateway mirrors erebrus-gateway/internal/wallet.Verify. The node never
// verifies its own proofs in production, so this is the only place the two
// repositories' crypto is checked against each other.
func verifyAsGateway(chain, message, signature, operator string) error {
	if chain == wallet.ChainSOL {
		pub, err := base58.Decode(operator)
		if err != nil {
			return err
		}
		sig, err := base58.Decode(signature)
		if err != nil {
			return err
		}
		if !ed25519.Verify(pub, []byte(message), sig) {
			return fmt.Errorf("solana signature did not verify")
		}
		return nil
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil || len(sig) != 65 {
		return fmt.Errorf("signature is not 65 bytes: %v", err)
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	pub, err := ethcrypto.SigToPub(ethcrypto.Keccak256Hash([]byte(prefixed)).Bytes(), sig)
	if err != nil {
		return err
	}
	if recovered := ethcrypto.PubkeyToAddress(*pub).Hex(); !strings.EqualFold(recovered, operator) {
		return fmt.Errorf("recovered %q, want %q", recovered, operator)
	}
	return nil
}

// A throwaway BIP39 mnemonic; this key only ever signs consent proofs.
const testMnemonic = "legal winner thank year wave sausage worth useful legal winner thank yellow"

func testIdentity() NodeIdentity {
	return NodeIdentity{
		PeerID:  "peer-1",
		DID:     "did:erebrus:peer-1",
		IPHash:  HashIP("203.0.113.9"),
		Region:  "de",
		Version: "v2",
	}
}

func newTestAttester(t *testing.T, chain string) *Attester {
	t.Helper()
	a, err := NewAttester(testIdentity(), chain, testMnemonic)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// challenge mirrors erebrus-gateway store.registryIdentity field order.
func challenge(a *Attester, chain string, edit func([]string)) string {
	f := []string{
		registryDomain, chain, "0x1111111111111111111111111111111111111111",
		a.id.PeerID, a.id.DID, a.id.IPHash, strings.ToUpper(a.id.Region), a.id.Wallet,
		uuid.NewString(), strconv.FormatInt(time.Now().Add(4*time.Minute).Unix(), 10),
	}
	if edit != nil {
		edit(f)
	}
	out, _ := json.Marshal(f)
	return string(out)
}

func TestSignRegistryAcceptsThisNodesChallenge(t *testing.T) {
	for _, chain := range []string{"SOLANA", "ETHEREUM"} {
		a := newTestAttester(t, chain)
		code := wallet.ResolveChain(chain)
		message := challenge(a, code, nil)
		signature, nonce, err := a.SignRegistry(message)
		if err != nil {
			t.Fatalf("%s: %v", chain, err)
		}
		if signature == "" || nonce == "" {
			t.Fatalf("%s: empty proof", chain)
		}
		// The gateway recovers the signer from the bytes it reissued, so the
		// signature must verify against the operator wallet over that exact string.
		if err := verifyAsGateway(code, message, signature, a.Operator()); err != nil {
			t.Fatalf("%s: gateway could not verify: %v", chain, err)
		}
	}
}

// The node's wallet is its identity anchor on chain, so anything that does not
// describe this node must be refused rather than signed.
func TestSignRegistryRefusesForeignChallenges(t *testing.T) {
	a := newTestAttester(t, "SOLANA")
	for name, edit := range map[string]func([]string){
		"another domain":   func(f []string) { f[proofDomain] = "erebrus.something.v1" },
		"another chain":    func(f []string) { f[proofChain] = "evm" },
		"another peer":     func(f []string) { f[proofPeer] = "peer-2" },
		"another did":      func(f []string) { f[proofDID] = "did:erebrus:peer-2" },
		"another ip":       func(f []string) { f[proofIPHash] = HashIP("198.51.100.4") },
		"another region":   func(f []string) { f[proofRegion] = "FR" },
		"another operator": func(f []string) { f[proofOperator] = "7bXk9mJ2qL4pR8vN3wT6yC1aF5dH0eG9sU2iO4kM8nQp" },
		"no contract":      func(f []string) { f[proofRegistry] = "  " },
		"bad nonce":        func(f []string) { f[proofNonce] = "not-a-uuid" },
		"expired":          func(f []string) { f[proofExpires] = strconv.FormatInt(time.Now().Add(-time.Second).Unix(), 10) },
		"open ended": func(f []string) {
			f[proofExpires] = strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
		},
		"unparsable deadline": func(f []string) { f[proofExpires] = "soon" },
	} {
		if _, _, err := a.SignRegistry(challenge(a, "sol", edit)); err == nil {
			t.Errorf("%s was signed", name)
		}
	}
	for name, message := range map[string]string{
		"not json":    "{",
		"not a list":  `{"message":"sign me"}`,
		"short list":  `["erebrus.registry.v1","sol"]`,
		"empty":       ``,
		"transaction": `["0xdeadbeef"]`,
	} {
		if _, _, err := a.SignRegistry(message); err == nil {
			t.Errorf("%s was signed", name)
		}
	}
}

func TestSignUptimeMatchesGatewayMessage(t *testing.T) {
	a := newTestAttester(t, "SOLANA")
	bucket := CurrentUptimeBucket(time.Now())
	signature, err := a.SignUptime(bucket)
	if err != nil {
		t.Fatal(err)
	}
	// Rebuild the message exactly as store.RecordSignedUptime does.
	want, _ := json.Marshal([]string{
		uptimeDomain, "sol", a.id.PeerID, a.Operator(), strconv.FormatInt(bucket, 10),
	})
	if err := verifyAsGateway(wallet.ChainSOL, string(want), signature, a.Operator()); err != nil {
		t.Fatalf("gateway could not verify the attestation: %v", err)
	}
	if _, err := a.SignUptime(bucket + 1); err == nil {
		t.Fatal("signed a bucket off the 300s boundary")
	}
}

func TestCurrentUptimeBucketIsOnBoundary(t *testing.T) {
	bucket := CurrentUptimeBucket(time.Unix(1_700_000_123, 0))
	if bucket%UptimeBucketSeconds != 0 || bucket != 1_700_000_100 {
		t.Fatalf("bucket = %d", bucket)
	}
}

// A node left on the default REGION cannot be registered: the gateway requires
// a two-letter code, so fail loudly at startup rather than silently never
// registering.
func TestNewAttesterRequiresRegistryFields(t *testing.T) {
	for name, edit := range map[string]func(*NodeIdentity){
		"default region": func(id *NodeIdentity) { id.Region = "unknown" },
		"no region":      func(id *NodeIdentity) { id.Region = "" },
		"no peer":        func(id *NodeIdentity) { id.PeerID = "" },
		"no did":         func(id *NodeIdentity) { id.DID = "" },
		"no ip hash":     func(id *NodeIdentity) { id.IPHash = "" },
	} {
		id := testIdentity()
		edit(&id)
		if _, err := NewAttester(id, "SOLANA", testMnemonic); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if _, err := NewAttester(testIdentity(), "SOLANA", "not a mnemonic"); err == nil {
		t.Error("an invalid mnemonic was accepted")
	}
}
