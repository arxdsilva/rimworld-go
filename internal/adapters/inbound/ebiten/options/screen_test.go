package options

import (
	"testing"

	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

func TestVolumeStepGoesThroughSetSettings(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	before := f.settings.Audio.Master
	s.ApplyOption("options.master.dec")
	if len(f.set) != 1 || f.set[0].Audio.Master != before-config.VolumeStep {
		t.Fatalf("settings handed back %+v, want Master %d", f.set, before-config.VolumeStep)
	}
	if len(f.cues) != 1 || f.cues[0] != "ui.toggle" {
		t.Fatalf("cues %v, want the toggle sound", f.cues)
	}
}

func TestRebindEscapeCancelsAndAKeyIsSaved(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.Bindings = input.Defaults()
	f.rb = RebindState{On: true, act: input.Draft}
	f.env.Pressed = []string{"Escape"}
	s.CaptureBinding()
	if f.rb.On || len(f.set) != 0 {
		t.Fatalf("Escape: capture on=%v, saved %d; want it off and nothing saved", f.rb.On, len(f.set))
	}
	f.rb = RebindState{On: true, act: input.Draft}
	f.env.Pressed = []string{"F12"}
	s.CaptureBinding()
	if f.rb.On || f.rb.Pending != nil || len(f.set) != 1 {
		t.Fatalf("a free key: on=%v pending=%v saved %d; want it bound", f.rb.On, f.rb.Pending, len(f.set))
	}
}

func TestStoryTabExistsOnlyWhileAColonyRuns(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	s.Tab = "story"
	if got := s.CurrentTab(); got != "audio" {
		t.Fatalf("no colony: tab %q, want audio", got)
	}
	f.env.Running = true
	if got := s.CurrentTab(); got != "story" {
		t.Fatalf("colony: tab %q, want story", got)
	}
	if tabs := s.TabList(); len(tabs) != 9 || tabs[3] != "story" || tabs[4] != "habits" {
		t.Fatalf("tabs %v, want story in fourth place and habits after it", tabs)
	}
}

func TestEveryTabsButtonsFitAndDoNotOverlap(t *testing.T) {
	t.Parallel()
	f, s := newFake()
	f.env.Bindings = input.Defaults()
	for _, tab := range OptionTabs() {
		s.Tab = tab
		bs := s.Buttons(1280, 720)
		for i, b := range bs {
			if b.X+b.W > 1280 || b.Y+b.H > 720 {
				t.Fatalf("%s: %s is off screen", tab, b.ID)
			}
			for _, o := range bs[i+1:] {
				if b.X < o.X+o.W && o.X < b.X+b.W && b.Y < o.Y+o.H && o.Y < b.Y+b.H {
					t.Fatalf("%s: %s overlaps %s", tab, b.ID, o.ID)
				}
			}
		}
	}
}
