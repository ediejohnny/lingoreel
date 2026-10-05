package project

import "github.com/ediejohnny/lingoreel/internal/domain/subtitle"

// Change is one edit of a line as the history keeps it: the line before and
// after, on one of the session's subtitle lines (its own numbering).
type Change struct {
	Line   int
	Before subtitle.Event
	After  subtitle.Event
}

// History is the undo and redo of a session's edits. A new change drops
// what was undone; only the latest Limit changes can be undone.
type History struct {
	limit  int
	done   []Change
	undone []Change
}

// DefaultHistoryLimit is how many changes a session can undo.
const DefaultHistoryLimit = 200

// NewHistory keeps at most limit changes; below 1 it keeps one.
func NewHistory(limit int) *History {
	return &History{limit: max(limit, 1)}
}

// Record adds a change made by the user.
func (h *History) Record(change Change) {
	h.done = append(h.done, change)
	if len(h.done) > h.limit {
		h.done = h.done[len(h.done)-h.limit:]
	}
	h.undone = nil
}

// Undo takes back the latest change: put its Before back.
func (h *History) Undo() (Change, bool) {
	if len(h.done) == 0 {
		return Change{}, false
	}
	change := h.done[len(h.done)-1]
	h.done = h.done[:len(h.done)-1]
	h.undone = append(h.undone, change)
	return change, true
}

// Redo makes the latest undone change again: put its After back.
func (h *History) Redo() (Change, bool) {
	if len(h.undone) == 0 {
		return Change{}, false
	}
	change := h.undone[len(h.undone)-1]
	h.undone = h.undone[:len(h.undone)-1]
	h.done = append(h.done, change)
	return change, true
}

// Forget drops every change of one line, from both undo and redo, keeping
// the order of the others. It runs when the line takes another subtitle
// file: its lines are numbered afresh, so an old change would write a line
// of the old file over a line of the new one.
func (h *History) Forget(line int) {
	keep := func(changes []Change) []Change {
		kept := changes[:0]
		for _, change := range changes {
			if change.Line != line {
				kept = append(kept, change)
			}
		}
		return kept
	}
	h.done, h.undone = keep(h.done), keep(h.undone)
}

// CanUndo and CanRedo tell whether there is something to take back or make
// again.
func (h *History) CanUndo() bool { return len(h.done) > 0 }
func (h *History) CanRedo() bool { return len(h.undone) > 0 }
