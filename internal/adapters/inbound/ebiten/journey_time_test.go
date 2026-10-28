package ebiten

// journey_time_test.go: time, speed and dev journeys (QA-001).

import (
	"strconv"
	"testing"

	"rimworld-go/internal/domain/sim/command"
)

// timeRun runs n Updates and returns the ticks the colony advanced.
func timeRun(j *journey, n int) uint64 {
	before := j.g.Tick()
	j.update(n)
	return uint64(j.g.Tick() - before)
}

// TestJourneyTimeSpaceAndSpeeds: Space starts and stops the clock and the
// speed keys set 1, 3 and 6 ticks per Update.
func TestJourneyTimeSpaceAndSpeeds(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if got := timeRun(j, 5); got != 0 {
		t.Fatalf("the paused colony ran %d ticks", got)
	}
	j.key("Space", 0)
	for _, c := range []struct {
		key  string
		want uint64
	}{{"", 10}, {"Digit2", 30}, {"Digit3", 60}, {"Digit1", 10}} {
		if c.key != "" {
			j.key(c.key, 0)
		}
		if got := timeRun(j, 10); got != c.want {
			t.Fatalf("after %q: 10 Updates ran %d ticks, want %d", c.key, got, c.want)
		}
	}
	j.key("Space", 0)
	if got := timeRun(j, 10); got != 0 {
		t.Fatalf("Space did not pause: %d ticks", got)
	}
	j.key("Space", 0)
	if got := timeRun(j, 10); got != 10 {
		t.Fatalf("Space did not resume at normal speed: %d ticks", got)
	}
}

// TestJourneyTimeHomeEndStep: Home speeds up a tier at a time, End slows
// down, both clamp.
func TestJourneyTimeHomeEndStep(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	r := j.a.renderer
	for _, c := range []struct {
		key  string
		want speed
	}{
		{"Home", speedNormal}, {"Home", speedFast}, {"Home", speedVeryFast}, {"Home", speedVeryFast},
		{"End", speedFast}, {"End", speedNormal}, {"End", speedPaused}, {"End", speedPaused},
	} {
		j.key(c.key, 0)
		if r.speed != c.want {
			t.Fatalf("%s: speed %v, want %v", c.key, r.speed, c.want)
		}
	}
}

// TestJourneyTimeSpeedButtons: the HUD's speed buttons; the pause button
// toggles like Space.
func TestJourneyTimeSpeedButtons(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	r := j.a.renderer
	for _, c := range []struct {
		id   string
		want speed
	}{{"speed.1", speedNormal}, {"speed.2", speedFast}, {"speed.3", speedVeryFast}, {"speed.0", speedPaused}, {"speed.0", speedNormal}} {
		j.clickID(c.id)
		if r.speed != c.want {
			t.Fatalf("%s: speed %v, want %v", c.id, r.speed, c.want)
		}
	}
}

// TestJourneyTimeThreatPauses: a raid pauses a running colony and the HUD
// says why.
func TestJourneyTimeThreatPauses(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.a.settings.Gameplay.PauseOnThreat {
		t.Fatal("pause on threat is off by default")
	}
	r := j.a.renderer
	j.key("Digit1", 0)
	j.update(2) // the first danger message is only noted
	j.g.Raid(1)
	j.until(func() bool { return r.speed == speedPaused }, 20)
	if !r.threatPaused || r.speedRate() != "paused (threat)" {
		t.Fatalf("paused: threatPaused %v, rate %q", r.threatPaused, r.speedRate())
	}
	j.key("Digit1", 0)
	if r.speed != speedNormal || r.speedRate() == "paused (threat)" {
		t.Fatalf("Digit1 did not resume: %v, %q", r.speed, r.speedRate())
	}
}

// TestJourneyTimeDevStepUltrafastAndActions: developer mode's keys.
func TestJourneyTimeDevStepUltrafastAndActions(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	r := j.a.renderer
	j.key("Digit4", 0)
	if r.speed == speedUltra {
		t.Fatal("ultrafast works with developer mode off")
	}
	j.a.settings.Gameplay.DevMode = true
	j.update(1)

	before := j.g.Tick()
	j.key("NumpadAdd", 0)
	j.update(2)
	if got := j.g.Tick() - before; got != 1 {
		t.Fatalf("the dev step ran %d ticks, want 1", got)
	}

	j.key("Digit4", 0)
	if r.speed != speedUltra {
		t.Fatalf("Digit4: speed %v, want ultrafast", r.speed)
	}
	if got := timeRun(j, 4); got != 60 {
		t.Fatalf("ultrafast ran %d ticks in 4 Updates, want 60", got)
	}
	j.key("Digit4", 0)
	if r.speed != speedNormal {
		t.Fatalf("second Digit4: speed %v, want normal", r.speed)
	}
	j.key("Space", 0)

	j.key("Backquote", 0)
	if !j.a.dev.log {
		t.Fatal("Backquote did not open the debug log")
	}
	j.key("Quote", 0)
	if !j.a.dev.insp {
		t.Fatal("Quote did not open the debug inspector")
	}

	j.f.MoveTo(j.a, 13, 5)
	j.key("Slash", 0)
	if !j.a.dev.menu {
		t.Fatal("Slash opened no debug menu")
	}
	row := -1
	for i, act := range j.a.devActions(j.a.dev.tx, j.a.dev.ty) {
		if act.label == "Start fire here" {
			row = i
		}
	}
	if row < 0 || !j.clickID("dev.act."+strconv.Itoa(row)) {
		t.Fatalf("no enabled Start fire row (%d)", row)
	}
	if c, ok := j.lastCmd(command.DevStartFire); !ok || c.X != 13 || c.Y != 5 {
		t.Fatalf("Start fire sent %+v (%v), want DevStartFire at (13, 5)", c, ok)
	}
}
