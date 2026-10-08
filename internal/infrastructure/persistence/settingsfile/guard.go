package settingsfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/jsonstore"
)

// guard keeps a settings document the app could not read from being written
// over with the defaults: one written by a newer version, or one that could
// not be opened, is never written; a damaged one is copied aside before the
// first write. It also makes the writes of one document one at a time.
type guard struct {
	mu   sync.Mutex
	path string
	// checked tells the document was read once, so its state is known.
	checked bool
	// refuse is why the document is never written; damaged tells it is
	// copied aside before the next write.
	refuse  error
	damaged bool
	// now names the copy of a damaged document.
	now func() time.Time
}

// classify words a failed read as the settings' errors, keeping the cause.
func classify(err error) error {
	var opening *fs.PathError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, jsonstore.ErrNewerVersion):
		return fmt.Errorf("%w: %w", settings.ErrNewerVersion, err)
	case errors.As(err, &opening):
		return fmt.Errorf("%w: %w", settings.ErrUnreadable, err)
	default:
		return fmt.Errorf("%w: %w", settings.ErrDamaged, err)
	}
}

// learn keeps what a read found: err is classify's.
func (g *guard) learn(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.checked = true
	g.refuse, g.damaged = nil, false
	switch {
	case errors.Is(err, settings.ErrNewerVersion), errors.Is(err, settings.ErrUnreadable):
		g.refuse = err
	case err != nil:
		g.damaged = true
	}
}

func (g *guard) known() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.checked
}

// write runs save once the document may be written: never for one kept as
// it is (ErrReadOnly), and after a damaged one is copied aside.
func (g *guard) write(save func() error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.refuse != nil {
		return fmt.Errorf("%w: %w", settings.ErrReadOnly, g.refuse)
	}
	if g.damaged {
		if err := g.setAside(); err != nil {
			return err
		}
		g.damaged = false
	}
	return save()
}

// setAside copies the damaged document beside it, as
// "<name>.damaged-<date>-<time>.json"; a document gone meanwhile has nothing
// to keep.
func (g *guard) setAside() error {
	source, err := os.Open(g.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("keeping the damaged %s: %w", g.path, err)
	}
	defer source.Close()
	name := strings.TrimSuffix(filepath.Base(g.path), filepath.Ext(g.path))
	copyPath := filepath.Join(filepath.Dir(g.path), name+".damaged-"+g.now().Format("20060102-150405")+".json")
	target, err := os.OpenFile(copyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil // copied aside a moment ago
	}
	if err != nil {
		return fmt.Errorf("keeping the damaged %s: %w", g.path, err)
	}
	_, err = io.Copy(target, source)
	if err == nil {
		err = target.Sync()
	}
	if closed := target.Close(); err == nil {
		err = closed
	}
	if err != nil {
		os.Remove(copyPath)
		return fmt.Errorf("keeping the damaged %s: %w", g.path, err)
	}
	return nil
}
