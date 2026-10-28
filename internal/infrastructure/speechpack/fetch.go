package speechpack

import (
	"context"
	"errors"
	"fmt"

	"github.com/ediejohnny/lingoreel/internal/application/speechpack"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/download"
)

// Fetcher fetches the packs' files with client, telling a file the server
// does not hand over from one that arrived other than published.
func Fetcher(client download.Client) speechpack.Fetcher {
	return func(ctx context.Context, url string, file speech.File, dest string, progress func(done, total int64)) error {
		err := client.Fetch(ctx, download.Request{URL: url, Size: file.Size, SHA256: file.SHA256, Dest: dest}, progress)
		switch {
		case errors.Is(err, download.ErrStatus):
			return fmt.Errorf("%w: %w", speechpack.ErrNotPublished, err)
		case errors.Is(err, download.ErrSize), errors.Is(err, download.ErrDamaged):
			return fmt.Errorf("%w: %w", speechpack.ErrNotAsPublished, err)
		}
		return err
	}
}
