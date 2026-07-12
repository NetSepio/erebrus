package drop

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

func TestClientHealthStatsAndStreamingOperations(t *testing.T) {
	testCID := mustCID(t, []byte("hello"))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v0/version", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"Version":"0.42.0"}`)
	})
	mux.HandleFunc("/api/v0/repo/stat", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"RepoSize":123,"StorageMax":1000,"NumObjects":4}`)
	})
	mux.HandleFunc("/api/v0/add", func(w http.ResponseWriter, r *http.Request) {
		part, err := r.MultipartReader()
		if err != nil {
			t.Errorf("multipart reader: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		file, err := part.NextPart()
		if err != nil {
			t.Errorf("multipart part: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(file)
		if err != nil {
			t.Errorf("read upload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if string(data) != "hello" {
			t.Errorf("upload body = %q", data)
		}
		io.WriteString(w, `{"Name":"upload-1","Hash":"`+testCID+`","Size":"5"}`+"\n")
	})
	mux.HandleFunc("/api/v0/cat", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "hello")
	})
	mux.HandleFunc("/api/v0/pin/ls", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"Keys":{"`+testCID+`":{"Type":"recursive"}}}`)
	})
	mux.HandleFunc("/api/v0/pin/rm", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"Pins":["`+testCID+`"]}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()
	version, err := client.Version(ctx)
	if err != nil || version != "0.42.0" {
		t.Fatalf("version = %q err=%v", version, err)
	}
	stats, err := client.RepoStats(ctx)
	if err != nil || stats.RepoSize != 123 || stats.NumObjects != 4 {
		t.Fatalf("stats = %+v err=%v", stats, err)
	}
	added, err := client.AddAndPin(ctx, AddRequest{
		UploadID: "upload-1", Body: strings.NewReader("hello"), DeclaredSize: 5, MaxBytes: 5,
		SHA256: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
	})
	if err != nil || added.CID != testCID || added.Size != 5 {
		t.Fatalf("add = %+v err=%v", added, err)
	}
	body, err := client.Cat(ctx, testCID, 5)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil || string(data) != "hello" {
		t.Fatalf("cat = %q err=%v", data, err)
	}
	pinned, err := client.PinStatus(ctx, testCID)
	if err != nil || !pinned {
		t.Fatalf("pin status = %v err=%v", pinned, err)
	}
	if err := client.Unpin(ctx, testCID); err != nil {
		t.Fatal(err)
	}
}

func TestClientEnforcesStreamingLimits(t *testing.T) {
	testCID := mustCID(t, []byte("hello!"))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v0/add", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		io.WriteString(w, `{"Hash":"`+testCID+`","Size":"6"}`+"\n")
	})
	mux.HandleFunc("/api/v0/cat", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "hello!")
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(server.URL)

	_, err := client.AddAndPin(context.Background(), AddRequest{
		UploadID: "upload-2", Body: strings.NewReader("hello!"), DeclaredSize: 5, MaxBytes: 5,
	})
	if !errors.Is(err, ErrByteLimit) {
		t.Fatalf("add error = %v, want byte limit", err)
	}
	body, err := client.Cat(context.Background(), testCID, 5)
	if errors.Is(err, ErrByteLimit) {
		return
	}
	if err != nil {
		t.Fatalf("cat: %v", err)
	}
	_, err = io.ReadAll(body)
	body.Close()
	if !errors.Is(err, ErrByteLimit) {
		t.Fatalf("cat error = %v, want byte limit", err)
	}
}

func TestClientRejectsInvalidCID(t *testing.T) {
	client := NewClient("http://127.0.0.1")
	if _, err := client.Cat(context.Background(), "not-a-cid", 10); err == nil {
		t.Fatal("expected invalid CID error")
	}
}

func mustCID(t *testing.T, data []byte) string {
	t.Helper()
	hash, err := multihash.Sum(data, multihash.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	return cid.NewCidV1(cid.Raw, hash).String()
}
