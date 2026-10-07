// Package storyboard plans a film's stills for the seek bar, as video sites
// do: one every few seconds over the whole film, taken from coarse to fine
// so that the first hundred already cover it, the moment under the pointer
// before any other.
package storyboard

import "time"

const (
	// Step is the time between two stills.
	Step = 10 * time.Second
	// MaxStills bounds a film's stills; longer films space them further.
	MaxStills = 1000
	// reachShare is how far a still may stand for a moment it was not taken
	// at: a hundredth of the film, the gaps the first stills leave.
	reachShare = 100
)

// Board is the grid of a film's stills: Count slots, Step apart from 0.
type Board struct {
	Duration time.Duration
	Step     time.Duration
	Count    int
}

// For lays the grid over a film. Past MaxStills steps the slots are spaced
// further, by whole seconds.
func For(duration time.Duration) Board {
	if duration <= 0 {
		return Board{}
	}
	step := Step
	if duration > MaxStills*Step {
		step = (duration/MaxStills + time.Second - 1).Truncate(time.Second)
	}
	return Board{Duration: duration, Step: step, Count: int((duration + step - 1) / step)}
}

// At is the moment of a slot.
func (b Board) At(slot int) time.Duration { return time.Duration(slot) * b.Step }

// Slot is the slot nearest to a moment, within the board.
func (b Board) Slot(at time.Duration) int {
	if b.Count == 0 {
		return 0
	}
	return min(max(int((at+b.Step/2)/b.Step), 0), b.Count-1)
}

// Reach is how far a still may be from a moment it stands for.
func (b Board) Reach() time.Duration { return max(b.Duration/reachShare, b.Step) }

// Order lists the slots from coarse to fine: each pass takes the slots
// halfway between those already taken, so after the pass with a stride of
// n slots no gap is wider than n.
func (b Board) Order() []int {
	order := make([]int, 0, b.Count)
	stride := 1
	for stride*2 < b.Count {
		stride *= 2
	}
	for slot := 0; slot < b.Count; slot += stride {
		order = append(order, slot)
	}
	for ; stride > 1; stride /= 2 {
		for slot := stride / 2; slot < b.Count; slot += stride {
			order = append(order, slot)
		}
	}
	return order
}

// Plan follows a film's stills: which slots are answered, and by the still
// taken when.
type Plan struct {
	board  Board
	order  []int
	cursor int
	// answer is, per slot, the time of the still that stands for it; -1 for
	// none yet, skipped for a slot whose still failed.
	answer []time.Duration
}

const (
	unanswered time.Duration = -1
	skipped    time.Duration = -2
)

// NewPlan starts a film's plan with no still taken.
func NewPlan(board Board) *Plan {
	answer := make([]time.Duration, board.Count)
	for i := range answer {
		answer[i] = unanswered
	}
	return &Plan{board: board, order: board.Order(), answer: answer}
}

// Board is the grid the plan follows.
func (p *Plan) Board() Board { return p.board }

func (p *Plan) open(slot int) bool { return p.answer[slot] == unanswered }

// Next is the slot to take next: the one under the pointer when it has no
// still yet (waited), else the next one in order without a still; ok is
// false once every slot has one.
func (p *Plan) Next(pointed bool, at time.Duration) (slot int, waited, ok bool) {
	if p.board.Count == 0 {
		return 0, false, false
	}
	if pointed {
		if slot := p.board.Slot(at); p.open(slot) {
			return slot, true, true
		}
	}
	for ; p.cursor < len(p.order); p.cursor++ {
		if slot := p.order[p.cursor]; p.open(slot) {
			return slot, false, true
		}
	}
	return 0, false, false
}

// Done records the still taken for a slot, which really is from at: the
// slots between at and the slot's own moment are answered by it too, since
// asking for any of them lands on that same keyframe.
func (p *Plan) Done(slot int, at time.Duration) {
	if slot < 0 || slot >= p.board.Count {
		return
	}
	for other := slot - 1; other >= 0 && p.board.At(other) >= at; other-- {
		if p.open(other) {
			p.answer[other] = at
		}
	}
	p.answer[slot] = at
}

// Skip gives a slot up: its still failed, and the plan goes on without it.
func (p *Plan) Skip(slot int) {
	if slot >= 0 && slot < p.board.Count && p.open(slot) {
		p.answer[slot] = skipped
	}
}

// Answer is the still that stands for a moment: its time, and whether it is
// the moment's own still (shown sharp) or the nearest one taken so far
// (shown blurred, as an approximation); ok is false with none within reach.
func (p *Plan) Answer(at time.Duration) (still time.Duration, own, ok bool) {
	if p.board.Count == 0 {
		return 0, false, false
	}
	slot := p.board.Slot(at)
	if answer := p.answer[slot]; answer >= 0 {
		return answer, true, true
	}
	reach := p.board.Reach()
	best := reach + 1
	for other := p.board.Slot(at - reach); other <= p.board.Slot(at+reach); other++ {
		if answer := p.answer[other]; answer >= 0 && distance(answer, at) < best {
			still, best = answer, distance(answer, at)
		}
	}
	return still, false, best <= reach
}

func distance(a, b time.Duration) time.Duration {
	if a > b {
		return a - b
	}
	return b - a
}
