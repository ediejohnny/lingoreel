package download_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/download"
)

// published is a file as its page publishes it: the bytes, their size and
// SHA-256.
var published = bytes.Repeat([]byte("speech model "), 50_000)

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// serve hands out body at /model over HTTPS, at /chunked in pieces with no
// length told before, and at /to-http sends to the same file over plain
// HTTP.
func serve(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(plain.Close)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/model":
			_, _ = w.Write(body)
		case "/chunked":
			for part := range slices.Chunk(body, 64*1024) {
				_, _ = w.Write(part)
				w.(http.Flusher).Flush()
			}
		case "/to-http":
			http.Redirect(w, r, plain.URL+"/model", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// client trusts the test server's certificate; the redirect rule is the
// package's own.
func client(server *httptest.Server) download.Client {
	return download.Client{HTTP: server.Client()}
}

// nothingLeft fails when the folder holds anything but the files named.
func nothingLeft(t *testing.T, dir string, keep ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		found := false
		for _, k := range keep {
			found = found || e.Name() == k
		}
		if !found {
			t.Errorf("left behind: %s", e.Name())
		}
	}
}

func TestAPublishedFileArrivesWhole(t *testing.T) {
	t.Parallel()
	server := serve(t, published)
	dir := t.TempDir()
	dest := filepath.Join(dir, "en", "model.onnx")
	var last, total int64
	err := client(server).Fetch(t.Context(), download.Request{
		URL: server.URL + "/model", Size: int64(len(published)), SHA256: sum(published), Dest: dest,
	}, func(done, all int64) { last, total = done, all })
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(got, published) {
		t.Fatalf("the file arrived other than published (%v)", err)
	}
	if last != total || total != int64(len(published)) {
		t.Errorf("progress ended at %d of %d", last, total)
	}
	nothingLeft(t, filepath.Dir(dest), "model.onnx")
}

func TestAFileOtherThanPublishedNeverTakesItsPlace(t *testing.T) {
	t.Parallel()
	tampered := bytes.Clone(published)
	tampered[1000] ^= 1
	cases := []struct {
		name string
		body []byte
		size int64
		want error
	}{
		{"changed", tampered, int64(len(published)), download.ErrDamaged},
		{"longer", append(bytes.Clone(published), 'x'), int64(len(published)), download.ErrSize},
		{"shorter", published[:len(published)-1], int64(len(published)), download.ErrSize},
	}
	for _, route := range []string{"/model", "/chunked"} {
		for _, c := range cases {
			t.Run(c.name+route, func(t *testing.T) {
				t.Parallel()
				server := serve(t, c.body)
				dir := t.TempDir()
				dest := filepath.Join(dir, "model.onnx")
				if err := os.WriteFile(dest, []byte("the model kept before"), 0o644); err != nil {
					t.Fatal(err)
				}
				err := client(server).Fetch(t.Context(), download.Request{
					URL: server.URL + route, Size: c.size, SHA256: sum(published), Dest: dest,
				}, nil)
				if !errors.Is(err, c.want) {
					t.Fatalf("Fetch = %v, want %v", err, c.want)
				}
				if kept, _ := os.ReadFile(dest); string(kept) != "the model kept before" {
					t.Errorf("the file before was replaced by %d bytes", len(kept))
				}
				nothingLeft(t, dir, "model.onnx")
			})
		}
	}
}

func TestOnlyHTTPSIsFetched(t *testing.T) {
	t.Parallel()
	server := serve(t, published)
	dir := t.TempDir()
	request := download.Request{Size: int64(len(published)), SHA256: sum(published), Dest: filepath.Join(dir, "m")}
	for _, address := range []string{"http://example.com/model", "file:///C:/model", strings.Replace(server.URL, "https", "ftp", 1)} {
		request.URL = address
		if err := (download.Client{}).Fetch(t.Context(), request, nil); !errors.Is(err, download.ErrNotHTTPS) {
			t.Errorf("Fetch(%s) = %v, want ErrNotHTTPS", address, err)
		}
	}
	request.URL = server.URL + "/to-http"
	if err := client(server).Fetch(t.Context(), request, nil); !errors.Is(err, download.ErrNotHTTPS) {
		t.Errorf("a redirect to HTTP: %v, want ErrNotHTTPS", err)
	}
	// A client with its own redirect rule still never leaves HTTPS.
	lenient := server.Client()
	lenient.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }
	if err := (download.Client{HTTP: lenient}).Fetch(t.Context(), request, nil); !errors.Is(err, download.ErrNotHTTPS) {
		t.Errorf("a redirect to HTTP with the client's own rule: %v, want ErrNotHTTPS", err)
	}
	nothingLeft(t, dir)
}

func TestAStoppedFetchLeavesNothing(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		_, _ = w.Write(published[:1000])
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-started
		cancel()
	}()
	err := download.Client{HTTP: server.Client()}.Fetch(ctx, download.Request{
		URL: server.URL, Size: 1000000, SHA256: sum(published), Dest: filepath.Join(dir, "model.onnx"),
	}, nil)
	if err == nil {
		t.Fatal("a cancelled fetch succeeded")
	}
	nothingLeft(t, dir)
}

func TestWhatAStoppedFetchLeftLongAgoIsSwept(t *testing.T) {
	t.Parallel()
	server := serve(t, published)
	// A bracket in the folder's name is a name, not a pattern.
	dir := filepath.Join(t.TempDir(), "models [en]")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "model.onnx")
	old, fresh := dest+".partial-111", dest+".partial-222"
	for _, path := range []string{old, fresh} {
		if err := os.WriteFile(path, []byte("half"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	if err := os.Chtimes(old, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	c := client(server)
	c.Now = func() time.Time { return now }
	if err := c.Fetch(t.Context(), download.Request{
		URL: server.URL + "/model", Size: int64(len(published)), SHA256: sum(published), Dest: dest,
	}, nil); err != nil {
		t.Fatal(err)
	}
	// The fresh one may belong to a fetch still running: it stays.
	nothingLeft(t, dir, "model.onnx", filepath.Base(fresh))
}
