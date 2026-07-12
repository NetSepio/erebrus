package drop

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

var probeCID string

func init() {
	const probeData = "erebrus drop gateway probe"
	h, err := multihash.Sum([]byte(probeData), multihash.SHA2_256, -1)
	if err != nil {
		probeCID = ""
		return
	}
	probeCID = cid.NewCidV1(cid.Raw, h).String()
}

// ProbePublicGatewayURL checks whether the public HTTPS gateway at baseURL is
// reachable with valid TLS. It makes a conservative GET to a deterministic,
// non-pinned CID path and treats any response that is not a 5xx proxy error as
// reachable. The probe never requests user content or exposes the Kubo RPC.
func ProbePublicGatewayURL(ctx context.Context, baseURL string) bool {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" || probeCID == "" {
		return false
	}
	probeURL := baseURL + "/ipfs/" + probeCID

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
		// Do not follow redirects; a 3xx from the gateway is still reachable.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	// 5xx responses indicate the TLS endpoint is up but the backend cannot be
	// reached, so the gateway is not ready to be advertised.
	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		return false
	}
	return true
}
