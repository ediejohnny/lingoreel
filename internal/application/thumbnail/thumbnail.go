// Package thumbnail shows, over the seek bar, a small picture of the moment
// under the pointer: stills of the film taken one at a time, at the lowest
// priority, the moment under the pointer first.
package thumbnail

import (
	"context"
	"errors"
	"time"
)

// ErrNoStills reports a film that gives no stills: no picture at all, or one
// the app cannot show right (the bar then shows the time alone).
var ErrNoStills = errors.New("the film gives no stills")

// Picture is one still of a film: a small JPEG the window decodes.
type Picture struct {
	// At is when the still really is: the keyframe at or just before the
	// moment asked for, as near as the source can tell.
	At            time.Duration
	Width, Height int
	JPEG          []byte
}

// Shot asks a film for its still at, or just before, a moment.
type Shot struct {
	At time.Duration
	// Waited says the pointer rests on this moment, waiting for its still:
	// it is taken at once, queueing behind nothing.
	Waited bool
	// Paced says nobody is at the bar and the overview is done: the still
	// may take its time, holding the film's stills to a small share of the
	// processor.
	Paced bool
}

// Film takes the stills of one video.
type Film interface {
	Grab(ctx context.Context, shot Shot) (Picture, error)
}

// Opener prepares a video for its stills, looking at it once, or reports
// ErrNoStills.
type Opener func(ctx context.Context, file string) (Film, error)
