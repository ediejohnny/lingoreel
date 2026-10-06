package voice

import (
	"context"
	"time"
)

// Decoding ahead runs in bursts with rests between them, so it takes at most
// aheadDuty of one core and the video and the rest of the computer keep
// theirs. Rests shorter than restAtLeast are gathered into one: sleeping a
// millisecond at a time costs more than it saves.
const (
	aheadDuty   = 0.25
	restAtLeast = 10 * time.Millisecond
)

// pace rests after work, long enough for the work to be its duty of the
// time.
type pace struct {
	duty float64
	owed time.Duration
}

// worked counts a piece of work and rests what is owed once it is worth it;
// it tells whether ctx goes on.
func (p *pace) worked(ctx context.Context, work time.Duration) bool {
	p.owed += time.Duration(float64(work) * (1 - p.duty) / p.duty)
	if p.owed < restAtLeast {
		return ctx.Err() == nil
	}
	owed := p.owed
	p.owed = 0
	return wait(ctx, owed)
}
