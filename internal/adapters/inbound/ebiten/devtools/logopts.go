package devtools

import "rimworld-go/internal/application/game"

// The developer log options (OPT-030, Settings.Developer): a verbose log,
// the log opening on a warning, Escape closing it, and the game pausing on
// an error. A warning and an error are the messages the game marks Danger:
// the dev log has no other severity.

// logWatch remembers which messages the log options already reacted to.
type logWatch struct {
	started bool
	tick    uint64 // the newest message tick handled
	n       int    // how many messages at that tick were handled
}

// fresh returns the messages not yet handled and moves the watch past them.
// The first call only catches up, so a loaded game's old messages do not
// open the log.
func (w *logWatch) fresh(msgs []game.MessageView) []game.MessageView {
	if !w.started {
		w.started = true
		w.skipAll(msgs)
		return nil
	}
	var out []game.MessageView
	at := 0
	for _, m := range msgs {
		switch {
		case m.Tick > w.tick:
			out = append(out, m)
		case m.Tick == w.tick:
			if at++; at > w.n {
				out = append(out, m)
			}
		}
	}
	w.skipAll(msgs)
	return out
}

func (w *logWatch) skipAll(msgs []game.MessageView) {
	if len(msgs) == 0 {
		return
	}
	w.tick, w.n = 0, 0
	for _, m := range msgs {
		switch {
		case m.Tick > w.tick:
			w.tick, w.n = m.Tick, 1
		case m.Tick == w.tick:
			w.n++
		}
	}
}

// watchLog applies OpenLogOnWarn and PauseOnError to the messages that
// arrived since the last Update.
func (p *Panel) watchLog() {
	dev := p.sh.Settings().Developer
	fresh := p.watch.fresh(p.sh.World().Messages)
	for _, m := range fresh {
		if !m.Danger {
			continue
		}
		if dev.OpenLogOnWarn {
			p.State.Log = true
		}
		if dev.PauseOnError && p.sh.SpeedTier() != speedPaused {
			p.sh.SetSpeed(speedPaused)
		}
	}
}

// Escape closes the log when LogEscCloses is on and the log is open; it
// reports the key was used, so the root's Escape order stops there.
func (p *Panel) Escape() bool {
	if !p.active() || !p.State.Log || !p.sh.Settings().Developer.LogEscCloses {
		return false
	}
	p.State.Log = false
	return true
}
