// Package download fetches a file the user asked for, such as a speech
// model, over HTTPS: its size and SHA-256 are known before, so a file that
// arrives other than published never takes the place of the one wanted.
// The file is written beside its place and moved there only once whole.
package download

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrNotHTTPS reports an address, or a redirect, that is not HTTPS.
	ErrNotHTTPS = errors.New("download: only HTTPS addresses are fetched")
	// ErrStatus reports a server that did not hand the file over.
	ErrStatus = errors.New("download: the server did not send the file")
	// ErrSize reports a file of another size than published.
	ErrSize = errors.New("download: the file is not the size published")
	// ErrDamaged reports a file whose SHA-256 is not the one published.
	ErrDamaged = errors.New("download: the file is not the one published (its SHA-256 differs)")
)

// partial marks a file being fetched; one left by a fetch that stopped is
// swept after staleAfter.
const (
	partial    = ".partial-"
	staleAfter = time.Hour
	// progressEvery bounds how often progress is told.
	progressEvery = 100 * time.Millisecond
)

// Request is a file to fetch: where from, its exact size and SHA-256 (hex),
// and the path it goes to.
type Request struct {
	URL    string
	Size   int64
	SHA256 string
	Dest   string
}

// Client fetches files with HTTP (the default client when nil), following
// redirects only to HTTPS, whatever rule HTTP has of its own.
type Client struct {
	HTTP *http.Client
	// Now is the clock that tells a stale partial file; time.Now when nil.
	Now func() time.Time
}

// Fetch downloads a file to its place, telling progress (bytes done of the
// total) as it goes, when progress is not nil. Nothing is left behind when
// it fails or ctx is cancelled.
func (c Client) Fetch(ctx context.Context, req Request, progress func(done, total int64)) (err error) {
	if err := https(req.URL); err != nil {
		return err
	}
	want, err := hex.DecodeString(req.SHA256)
	if err != nil || len(want) != sha256.Size || req.Size <= 0 || req.Dest == "" {
		return fmt.Errorf("download: a request needs its size, SHA-256 and place: %+v", req)
	}
	dir := filepath.Dir(req.Dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	c.sweep(req.Dest)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return err
	}
	response, err := c.client().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", ErrStatus, response.Status)
	}
	if response.ContentLength >= 0 && response.ContentLength != req.Size {
		return fmt.Errorf("%w: %d bytes, not %d", ErrSize, response.ContentLength, req.Size)
	}

	file, err := os.CreateTemp(dir, filepath.Base(req.Dest)+partial+"*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() {
		if file != nil {
			_ = file.Close()
		}
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	hash := sha256.New()
	counter := &counting{total: req.Size, progress: progress}
	written, err := io.Copy(io.MultiWriter(file, hash, counter), io.LimitReader(response.Body, req.Size+1))
	if err != nil {
		return err
	}
	if written != req.Size {
		return fmt.Errorf("%w: %d bytes, not %d", ErrSize, written, req.Size)
	}
	if !bytes.Equal(hash.Sum(nil), want) {
		return ErrDamaged
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	file = nil
	if err = os.Rename(name, req.Dest); err != nil {
		return err
	}
	if progress != nil {
		progress(req.Size, req.Size)
	}
	return nil
}

func (c Client) client() *http.Client {
	var client http.Client // a model is large: ctx bounds the wait, not a timeout
	if c.HTTP != nil {
		client = *c.HTTP
	}
	own := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := https(req.URL.String()); err != nil {
			return err
		}
		if own != nil {
			return own(req, via)
		}
		if len(via) >= 10 {
			return errors.New("download: too many redirects")
		}
		return nil
	}
	return &client
}

// sweep removes what a stopped fetch of the same file left long ago.
func (c Client) sweep(dest string) {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	dir, base := filepath.Dir(dest), filepath.Base(dest)+partial
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), base) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && now().Sub(info.ModTime()) > staleAfter {
			_ = os.Remove(path)
		}
	}
}

func https(address string) error {
	u, err := url.Parse(address)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
		return fmt.Errorf("%w: %q", ErrNotHTTPS, address)
	}
	return nil
}

// counting tells progress as bytes pass, at most every progressEvery.
type counting struct {
	done, total int64
	progress    func(done, total int64)
	last        time.Time
}

func (c *counting) Write(p []byte) (int, error) {
	c.done += int64(len(p))
	if c.progress != nil && time.Since(c.last) >= progressEvery {
		c.last = time.Now()
		c.progress(min(c.done, c.total), c.total)
	}
	return len(p), nil
}
