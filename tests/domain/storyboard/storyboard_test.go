package storyboard_test

import (
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/storyboard"
)

// A film's grid: a still every 10 s, never past the film's end, spaced
// further (by whole seconds) past 1000 stills.
func TestTheBoardCoversTheFilm(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		duration time.Duration
		step     time.Duration
		count    int
	}{
		{25 * time.Second, 10 * time.Second, 3}, // 0, 10, 20
		{30 * time.Second, 10 * time.Second, 3}, // 30 is the end itself
		{2 * time.Hour, 10 * time.Second, 720},
		{10000 * time.Second, 10 * time.Second, 1000},
		{3*time.Hour + 2*time.Minute, 11 * time.Second, 993}, // 10920 s / 1000 = 10.92, up to 11
		{10 * time.Hour, 36 * time.Second, 1000},
		{0, 0, 0},
	} {
		board := storyboard.For(c.duration)
		if board.Step != c.step || board.Count != c.count {
			t.Errorf("%v: %v apart, %d stills; want %v, %d", c.duration, board.Step, board.Count, c.step, c.count)
		}
		if board.Count > 0 && board.At(board.Count-1) >= c.duration {
			t.Errorf("%v: the last still is at %v, past the end", c.duration, board.At(board.Count-1))
		}
	}
	board := storyboard.For(2 * time.Hour)
	for at, want := range map[time.Duration]int{0: 0, 4 * time.Second: 0, 5 * time.Second: 1, 61 * time.Second: 6, -time.Hour: 0, 3 * time.Hour: 719} {
		if got := board.Slot(at); got != want {
			t.Errorf("Slot(%v) = %d, want %d", at, got, want)
		}
	}
}

// Every slot is taken once, from coarse to fine: after each pass no gap is
// wider than its stride, so the first hundred or so cover a 2 h film with
// no gap over 80 s.
func TestTheOrderGoesFromCoarseToFine(t *testing.T) {
	t.Parallel()
	for _, count := range []int{1, 2, 3, 17, 720, 1000} {
		board := storyboard.Board{Step: time.Second, Count: count}
		order := board.Order()
		if sorted := slices.Sorted(slices.Values(order)); len(order) != count || !slices.Equal(sorted, rangeOf(count)) {
			t.Fatalf("%d slots: order %v is not each slot once", count, order)
		}
		// The widest gap among the first n slots taken, ends included.
		gap := func(n int) int {
			taken := slices.Sorted(slices.Values(order[:n]))
			widest := count - 1 - taken[len(taken)-1]
			for i := 1; i < len(taken); i++ {
				widest = max(widest, taken[i]-taken[i-1])
			}
			return widest
		}
		if count == 720 {
			if widest := gap(100); widest > 8 {
				t.Errorf("the first 100 of 720 leave a gap of %d slots, want at most 8", widest)
			}
		}
		for n := 1; n <= count; n *= 2 {
			// After n stills the last finished pass has a stride under about
			// twice count/n (the next pass, unfinished, would hold 2count/s).
			if widest := gap(n); widest > 2*count/n+1 {
				t.Errorf("%d slots: after %d the widest gap is %d", count, n, widest)
			}
		}
	}
}

func rangeOf(n int) []int {
	all := make([]int, n)
	for i := range all {
		all[i] = i
	}
	return all
}

// The slot under the pointer goes first; a still answers its own slot and
// the slots back to the keyframe it really shows, which are not taken again;
// a failed still is skipped.
func TestThePointerGoesFirstAndAStillAnswersBackToItsKeyframe(t *testing.T) {
	t.Parallel()
	plan := storyboard.NewPlan(storyboard.For(100 * time.Second)) // slots 0..9
	if slot, waited, ok := plan.Next(true, 52*time.Second); !ok || slot != 5 || !waited {
		t.Fatalf("the pointer at 52 s gave slot %d (waited %v, %v), want 5", slot, waited, ok)
	}
	// The keyframe for 50 s is at 31 s: it answers 40 s and 50 s too.
	plan.Done(5, 31*time.Second)
	for slot, want := range map[int]time.Duration{4: 31 * time.Second, 5: 31 * time.Second} {
		if still, own, ok := plan.Answer(time.Duration(slot) * 10 * time.Second); !ok || !own || still != want {
			t.Errorf("slot %d answered by %v (own %v, %v), want its own %v", slot, still, own, ok, want)
		}
	}
	if slot, _, _ := plan.Next(true, 41*time.Second); slot == 4 {
		t.Fatal("slot 4, answered by the keyframe at 31 s, was taken again")
	}
	plan.Skip(3)
	var taken []int
	for {
		slot, waited, ok := plan.Next(false, 0)
		if !ok {
			break
		}
		if waited {
			t.Fatalf("slot %d taken as waited with no pointer", slot)
		}
		taken = append(taken, slot)
		plan.Done(slot, time.Duration(slot)*10*time.Second)
	}
	if !slices.Equal(slices.Sorted(slices.Values(taken)), []int{0, 1, 2, 6, 7, 8, 9}) {
		t.Fatalf("taken %v, want every slot but 3 (skipped), 4 and 5 (answered)", taken)
	}
}

// A moment shows its own still when it has one (sharp), else the nearest
// still within a hundredth of the film (blurred), else none.
func TestAMomentShowsItsOwnStillOrTheNearest(t *testing.T) {
	t.Parallel()
	plan := storyboard.NewPlan(storyboard.For(2 * time.Hour)) // reach 72 s
	if _, _, ok := plan.Answer(time.Hour); ok {
		t.Fatal("a still shown before any was taken")
	}
	plan.Done(360, time.Hour) // slot 360 is 1:00:00
	for _, c := range []struct {
		at       time.Duration
		own, any bool
	}{
		{time.Hour + 3*time.Second, true, true},   // its own slot
		{time.Hour + 50*time.Second, false, true}, // blurred, 50 s off
		{time.Hour - 70*time.Second, false, true},
		{time.Hour + 80*time.Second, false, false}, // beyond the reach
	} {
		still, own, ok := plan.Answer(c.at)
		if ok != c.any || own != c.own || (ok && still != time.Hour) {
			t.Errorf("at %v: still %v, own %v, %v; want own %v, shown %v", c.at, still, own, ok, c.own, c.any)
		}
	}
}
