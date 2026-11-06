package devtools

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/config"
)

// logShell lets a test set the developer options and the messages.
type logShell struct {
	*fakeShell
	dev  config.Developer
	msgs []game.MessageView
}

func (l *logShell) Settings() config.Settings {
	var s config.Settings
	s.Developer = l.dev
	return s
}
func (l *logShell) World() game.WorldView { return game.WorldView{Messages: l.msgs} }

func newLogPanel(dev config.Developer) (*Panel, *logShell) {
	f := &fakeShell{env: Env{On: true, Game: struct{}{}}, tier: speedNormal}
	l := &logShell{fakeShell: f, dev: dev, msgs: []game.MessageView{{Tick: 1, Text: "old", Danger: true}}}
	p := New(l)
	p.HandleKeys(keys{}) // catches up on the old message
	return p, l
}

func TestLogOpensOnAWarningOnly(t *testing.T) {
	t.Parallel()
	p, l := newLogPanel(config.Developer{OpenLogOnWarn: true})
	if p.State.Log {
		t.Fatal("an old warning opened the log")
	}
	l.msgs = append(l.msgs, game.MessageView{Tick: 2, Text: "calm"})
	p.HandleKeys(keys{})
	if p.State.Log {
		t.Fatal("a calm message opened the log")
	}
	l.msgs = append(l.msgs, game.MessageView{Tick: 2, Text: "raid", Danger: true})
	p.HandleKeys(keys{})
	if !p.State.Log {
		t.Fatal("a warning did not open the log")
	}
}

func TestLogOptionOffLeavesTheLogShut(t *testing.T) {
	t.Parallel()
	p, l := newLogPanel(config.Developer{})
	l.msgs = append(l.msgs, game.MessageView{Tick: 5, Text: "raid", Danger: true})
	p.HandleKeys(keys{})
	if p.State.Log || l.tier != speedNormal {
		t.Fatal("options off but the log or pause reacted")
	}
}

func TestPauseOnErrorPausesOncePerMessage(t *testing.T) {
	t.Parallel()
	p, l := newLogPanel(config.Developer{PauseOnError: true})
	l.msgs = append(l.msgs, game.MessageView{Tick: 3, Text: "raid", Danger: true})
	p.HandleKeys(keys{})
	if l.tier != speedPaused {
		t.Fatal("did not pause")
	}
	l.tier = speedNormal // the player resumes: the same message must not pause again
	p.HandleKeys(keys{})
	if l.tier != speedNormal {
		t.Fatal("paused again for a message already handled")
	}
}

func TestEscapeClosesTheLogOnlyWhenAsked(t *testing.T) {
	t.Parallel()
	p, _ := newLogPanel(config.Developer{LogEscCloses: true})
	if p.Escape() {
		t.Fatal("Escape used with the log shut")
	}
	p.State.Log = true
	if !p.Escape() || p.State.Log {
		t.Fatal("Escape did not close the log")
	}
	q, _ := newLogPanel(config.Developer{})
	q.State.Log = true
	if q.Escape() || !q.State.Log {
		t.Fatal("Escape closed the log with the option off")
	}
}

func TestVerboseLogShowsMoreDetail(t *testing.T) {
	t.Parallel()
	p, l := newLogPanel(config.Developer{})
	for i := range 30 {
		l.msgs = append(l.msgs, game.MessageView{Tick: uint64(10 + i), Text: "m", X: 3, Y: 4, Danger: i == 0})
	}
	plain := p.logShown(100, 2000)
	l.dev.VerboseLog = true
	verbose := p.logShown(100, 2000)
	if len(verbose) <= len(plain) {
		t.Fatalf("verbose %d lines, plain %d", len(verbose), len(plain))
	}
	if !strings.Contains(strings.Join(verbose, "\n"), "[danger]") || !strings.Contains(strings.Join(verbose, "\n"), "(3,4)") {
		t.Fatalf("verbose lines lack position and flag: %v", verbose[:3])
	}
}
