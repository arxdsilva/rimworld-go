package saves

import (
	"testing"
)

// The unsaved-progress state never flaps: each event of an interleaved
// run of ticks, saves and their results moves it once, to the state the
// table names, and only a written save clears it, up to its encode tick.
func TestProgressEventTable(t *testing.T) {
	t.Parallel()
	type event struct {
		name string
		do   func(p *progress)
		want bool // dirty after it
	}
	ok := SaveResult{Slot: "a"}
	table := []event{
		{"a colony begins at 100", func(p *progress) { *p = progressBegun(100) }, true},
		{"no tick yet", func(p *progress) { p.seen(100) }, true},
		{"a save at 100 is written", func(p *progress) { p.saved(100) }, false},
		{"the same tick again", func(p *progress) { p.seen(100) }, false},
		{"the clock runs to 200", func(p *progress) { p.seen(200) }, true},
		{"a background save of a encodes 200", func(p *progress) { p.started("a", 200) }, true},
		{"the clock runs to 300 meanwhile", func(p *progress) { p.seen(300) }, true},
		{"a is written: 200..300 stay unsaved", func(p *progress) { p.finished(ok) }, true},
		{"b encodes 300", func(p *progress) { p.started("b", 300) }, true},
		{"b fails", func(p *progress) { p.finished(SaveResult{Slot: "b", Err: errDisk}) }, true},
		{"a result with no save in flight", func(p *progress) { p.finished(ok) }, true},
		{"a encodes 300", func(p *progress) { p.started("a", 300) }, true},
		{"a is written", func(p *progress) { p.finished(ok) }, false},
		{"an older result cannot undo it", func(p *progress) { p.saved(250) }, false},
		{"a load at 50", func(p *progress) { *p = progressLoaded(50) }, false},
		{"the loaded colony runs", func(p *progress) { p.seen(51) }, true},
		{"a new colony", func(p *progress) { *p = progressBegun(51) }, true},
	}
	var p progress
	was, flips := p.dirty(), 0
	for _, e := range table {
		e.do(&p)
		if got := p.dirty(); got != e.want {
			t.Fatalf("%s: dirty %v, want %v (%+v)", e.name, got, e.want, p)
		}
		if p.dirty() != was {
			flips++
		}
		was = p.dirty()
	}
	if flips != 4 {
		t.Fatalf("%d transitions, want 4: one per event that changes the state", flips)
	}
}

// Through the book: a synchronous save clears at once, a failed one never,
// a background one only when written and only up to its encode tick.
func TestSaveNowTracksProgress(t *testing.T) {
	t.Parallel()
	f, b := newFake()
	f.env.CanSave = true
	b.NewColony(0)
	f.tick = 500
	f.saveErr = errDisk
	b.SaveNow("col")
	if b.progress.known || b.progress.savedAt != 0 {
		t.Fatalf("a failed save cleared progress: %+v", b.progress)
	}
	f.saveErr = nil
	b.SaveNow("col")
	if b.progress.dirty() || b.progress.savedAt != 500 {
		t.Fatalf("a written save: %+v", b.progress)
	}

	a, s := newAsync()
	s.NewColony(0)
	a.tick = 700
	s.SaveNow("col")
	a.tick = 900
	s.PollSaves()
	if s.progress.known || !s.progress.saving() {
		t.Fatalf("a save in flight cleared progress: %+v", s.progress)
	}
	a.done = []SaveResult{{Slot: "col"}}
	s.PollSaves()
	if s.progress.savedAt != 700 || s.progress.saving() || !s.progress.known {
		t.Fatalf("written: %+v, want saved up to its encode tick 700", s.progress)
	}
	s.progress.seen(int64(a.tick))
	if !s.progress.dirty() {
		t.Fatal("ticks after the encode stay unsaved")
	}
}
