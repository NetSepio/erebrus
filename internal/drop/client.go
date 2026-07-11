// Package drop provides the node's private Kubo adapter and Drop runtime state.
package drop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
)

const maxRPCResponseBytes = 1 << 20

var (
	ErrByteLimit    = errors.New("Drop byte limit exceeded")
	ErrSizeMismatch = errors.New("uploaded size does not match reservation")
	ErrHashMismatch = errors.New("uploaded SHA-256 does not match reservation")
)

// Client calls Kubo's internal admin RPC API.
type Client struct {
	baseURL string
	http    *http.Client
}

// AddRequest describes one bounded streaming add-and-pin operation.
type AddRequest struct {
	UploadID     string
	Body         io.Reader
	DeclaredSize int64
	MaxBytes     int64
	SHA256       string
}

// AddResult is the immutable content result returned by Kubo.
type AddResult struct {
	CID  string
	Size int64
}

// RepoStat is Kubo's repository capacity snapshot.
type RepoStat struct {
	RepoSize   int64 `json:"RepoSize"`
	StorageMax int64 `json:"StorageMax"`
	NumObjects int64 `json:"NumObjects"`
}

// NewClient builds a Kubo client with bounded connection and response-header timeouts.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		}},
	}
}

// Version returns the running Kubo version.
func (c *Client) Version(ctx context.Context) (string, error) {
	resp, err := c.post(ctx, "/api/v0/version", nil, nil, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Version string `json:"Version"`
	}
	if err := decodeBounded(resp.Body, &out); err != nil {
		return "", fmt.Errorf("decode Kubo version: %w", err)
	}
	if out.Version == "" {
		return "", fmt.Errorf("Kubo version response is empty")
	}
	return out.Version, nil
}

// RepoStats returns Kubo's current repository statistics.
func (c *Client) RepoStats(ctx context.Context) (RepoStat, error) {
	resp, err := c.post(ctx, "/api/v0/repo/stat", nil, nil, "")
	if err != nil {
		return RepoStat{}, err
	}
	defer resp.Body.Close()
	var out RepoStat
	if err := decodeBounded(resp.Body, &out); err != nil {
		return RepoStat{}, fmt.Errorf("decode Kubo repo stat: %w", err)
	}
	return out, nil
}

// AddAndPin streams a body into Kubo and verifies its reserved size and optional digest.
func (c *Client) AddAndPin(ctx context.Context, in AddRequest) (AddResult, error) {
	if in.Body == nil {
		return AddResult{}, fmt.Errorf("upload body is required")
	}
	if in.DeclaredSize < 0 || in.MaxBytes < 0 || in.DeclaredSize > in.MaxBytes {
		return AddResult{}, ErrByteLimit
	}
	pinName := sanitizePinName(in.UploadID)
	if pinName == "" {
		return AddResult{}, fmt.Errorf("upload ID is required")
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	writerDone := make(chan error, 1)
	go func() {
		part, err := mw.CreateFormFile("file", pinName)
		if err == nil {
			bounded := &boundedReader{r: in.Body, max: in.DeclaredSize}
			hasher := sha256.New()
			_, err = io.Copy(io.MultiWriter(part, hasher), bounded)
			if err == nil && bounded.n != in.DeclaredSize {
				err = ErrSizeMismatch
			}
			if err == nil && in.SHA256 != "" &&
				!strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), strings.TrimSpace(in.SHA256)) {
				err = ErrHashMismatch
			}
		}
		if closeErr := mw.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
		writerDone <- err
	}()

	query := url.Values{
		"cid-version": {"1"},
		"pin":         {"true"},
		"pin-name":    {pinName},
		"progress":    {"false"},
		"raw-leaves":  {"true"},
	}
	resp, err := c.post(ctx, "/api/v0/add", query, pr, mw.FormDataContentType())
	if err != nil {
		_ = pr.Close()
		<-writerDone
		return AddResult{}, err
	}
	defer resp.Body.Close()

	data, err := readBounded(resp.Body)
	writerErr := <-writerDone
	if writerErr != nil {
		return AddResult{}, writerErr
	}
	if err != nil {
		return AddResult{}, fmt.Errorf("read Kubo add response: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var final struct {
		Hash string `json:"Hash"`
		Size string `json:"Size"`
	}
	for decoder.More() {
		var item struct {
			Hash string `json:"Hash"`
			Size string `json:"Size"`
		}
		if err := decoder.Decode(&item); err != nil {
			return AddResult{}, fmt.Errorf("decode Kubo add response: %w", err)
		}
		if item.Hash != "" {
			final = item
		}
	}
	if final.Hash == "" {
		return AddResult{}, fmt.Errorf("Kubo add response did not include a CID")
	}
	if err := validateCID(final.Hash); err != nil {
		return AddResult{}, fmt.Errorf("Kubo returned invalid CID: %w", err)
	}
	return AddResult{CID: final.Hash, Size: in.DeclaredSize}, nil
}

// Cat streams a CID from Kubo without buffering the object.
func (c *Client) Cat(ctx context.Context, value string, maxBytes int64) (io.ReadCloser, error) {
	if err := validateCID(value); err != nil {
		return nil, err
	}
	resp, err := c.post(ctx, "/api/v0/cat", url.Values{"arg": {value}}, nil, "")
	if err != nil {
		return nil, err
	}
	if resp.ContentLength > maxBytes && resp.ContentLength >= 0 {
		resp.Body.Close()
		return nil, ErrByteLimit
	}
	return &limitReadCloser{ReadCloser: resp.Body, remaining: maxBytes}, nil
}

// PinStatus reports whether Kubo has a recursive pin for the CID.
func (c *Client) PinStatus(ctx context.Context, value string) (bool, error) {
	if err := validateCID(value); err != nil {
		return false, err
	}
	resp, err := c.postAllowStatus(ctx, "/api/v0/pin/ls", url.Values{"arg": {value}, "type": {"recursive"}}, nil, "")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := rpcStatusError(resp)
		if resp.StatusCode == http.StatusNotFound ||
			(resp.StatusCode == http.StatusInternalServerError && isNotPinnedError(err)) {
			return false, nil
		}
		return false, err
	}
	var out struct {
		Keys map[string]json.RawMessage `json:"Keys"`
	}
	if err := decodeBounded(resp.Body, &out); err != nil {
		return false, fmt.Errorf("decode Kubo pin status: %w", err)
	}
	_, ok := out.Keys[value]
	return ok, nil
}

// Unpin removes a recursive pin. Removing an absent pin is idempotent.
func (c *Client) Unpin(ctx context.Context, value string) error {
	if err := validateCID(value); err != nil {
		return err
	}
	resp, err := c.postAllowStatus(ctx, "/api/v0/pin/rm", url.Values{"arg": {value}, "recursive": {"true"}}, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	err = rpcStatusError(resp)
	if resp.StatusCode == http.StatusInternalServerError && isNotPinnedError(err) {
		return nil
	}
	return err
}

func (c *Client) post(ctx context.Context, path string, query url.Values, body io.Reader, contentType string) (*http.Response, error) {
	resp, err := c.postAllowStatus(ctx, path, query, body, contentType)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		err := rpcStatusError(resp)
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func (c *Client) postAllowStatus(ctx context.Context, path string, query url.Values, body io.Reader, contentType string) (*http.Response, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Kubo RPC %s: %w", path, err)
	}
	return resp, nil
}

func rpcStatusError(resp *http.Response) error {
	data, err := readBounded(resp.Body)
	if err != nil {
		return fmt.Errorf("Kubo RPC status %d", resp.StatusCode)
	}
	var out struct {
		Message string `json:"Message"`
	}
	_ = json.Unmarshal(data, &out)
	if out.Message == "" {
		out.Message = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("Kubo RPC status %d: %s", resp.StatusCode, out.Message)
}

func isNotPinnedError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not pinned") || strings.Contains(message, "not pinned or pinned indirectly")
}

func validateCID(value string) error {
	if _, err := cid.Decode(strings.TrimSpace(value)); err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}
	return nil
}

func sanitizePinName(value string) string {
	value = strings.TrimSpace(value)
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			out.WriteRune(r)
		}
		if out.Len() == 128 {
			break
		}
	}
	return out.String()
}

func decodeBounded(r io.Reader, out any) error {
	data, err := readBounded(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func readBounded(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxRPCResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRPCResponseBytes {
		return nil, fmt.Errorf("Kubo RPC response exceeds %d bytes", maxRPCResponseBytes)
	}
	return data, nil
}

type boundedReader struct {
	r   io.Reader
	max int64
	n   int64
}

func (r *boundedReader) Read(p []byte) (int, error) {
	if r.n == r.max {
		var extra [1]byte
		n, err := r.r.Read(extra[:])
		if n > 0 {
			return 0, ErrByteLimit
		}
		return 0, err
	}
	if int64(len(p)) > r.max-r.n {
		p = p[:r.max-r.n]
	}
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

type limitReadCloser struct {
	io.ReadCloser
	remaining int64
}

func (r *limitReadCloser) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		var extra [1]byte
		n, err := r.ReadCloser.Read(extra[:])
		if n > 0 {
			return 0, ErrByteLimit
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.ReadCloser.Read(p)
	r.remaining -= int64(n)
	return n, err
}
