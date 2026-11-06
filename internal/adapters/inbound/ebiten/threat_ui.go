package ebiten

import (
	camerapkgpkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/config"
)

// Threat pause (presentation only); the floating damage numbers are
// render/creature's (floaters.go).

// watchThreats pauses the game when something new arrives that the player's
// automatic-pause mode (General.AutoPause, OPT-005) names: nothing, a threat
// letter, any danger message or threat letter, or any letter. Messages and
// letters already there when the session starts or a save loads never pause.
// The threat sound and the HUD alert follow any danger message, whatever the
// mode.
func (r *Renderer) watchThreats(w game.WorldView) {
	newest := r.lastDanger
	for _, m := range w.Messages {
		if m.Danger && m.Tick > newest {
			newest = m.Tick
		}
	}
	letter, major := newLetters(r, w.Letters)
	watchBlasts(r, w)
	if !r.dangerInit {
		r.dangerInit, r.lastDanger = true, newest
		return
	}
	danger := newest != r.lastDanger
	if danger {
		r.lastDanger = newest
		r.threatAt = r.present
		r.cueBus().Global("ui.threat")
	}
	var pause bool
	switch r.smooth.Prefs.Mode {
	case config.AutoPauseMajor:
		pause = major
	case config.AutoPauseThreat:
		pause = danger || major
	case config.AutoPauseLetter:
		pause = danger || letter
	}
	if pause && r.speed != speedPaused {
		r.smooth.Prefs.LastSpeed = int(r.speed)
		r.speed = speedPaused
		r.threatPaused = true
		r.onThreatPause(w, r.lastDanger) // show the player the chase
	}
}

// newLetters notes the letters not seen before and reports whether any is
// new, and whether any new one is a threat. The scenario's opening letter
// opens by itself and never pauses.
func newLetters(r *Renderer, ls []game.LetterView) (anyNew, major bool) {
	p := &r.smooth.Prefs
	newest := p.LastLetter
	for _, l := range ls {
		if l.ID <= p.LastLetter {
			continue
		}
		newest = max(newest, l.ID)
		if !l.Intro && p.LetterInit {
			anyNew = true
			major = major || l.Kind == "threat"
		}
	}
	p.LastLetter, p.LetterInit = newest, true
	return anyNew, major
}

// watchBlasts shakes the screen for a new explosion near the view (OPT-027);
// lightning shakes it from the flash (layers.go). Explosions already in the
// list when the session starts or a save loads do not.
func watchBlasts(r *Renderer, w game.WorldView) {
	p := &r.smooth.Prefs
	newest := p.BlastTick
	for _, b := range w.Bursts {
		if b.Blast == "" || b.Tick <= p.BlastTick {
			continue
		}
		newest = max(newest, b.Tick)
		if p.BlastInit && p.Shake > 0 && nearView(r, b.X, b.Y) {
			r.smooth.Shake((blastShake + blastShakeR*float64(b.Radius)/10) * camerapkgpkg.ScreenDev())
		}
	}
	p.BlastTick, p.BlastInit = newest, true
}

// nearView reports a tile is within a screen of the colony viewport.
func nearView(r *Renderer, x, y int) bool {
	vp := r.cueBus().Voice.View
	if vp.W() <= 0 {
		return true
	}
	t := r.cam.Tile()
	sx, sy := (float64(x)+0.5)*t-r.cam.OffsetX, (float64(y)+0.5)*t-r.cam.OffsetY
	return sx > vp.X0-vp.W() && sx < vp.X1+vp.W() && sy > vp.Y0-vp.H() && sy < vp.Y1+vp.H()
}

// onThreatPause brings an off-screen chase into the colony viewport when
// its warning has just paused the game (render/creature chase.go finds it;
// the camera is the renderer's).
func (r *Renderer) onThreatPause(w game.WorldView, tick uint64) {
	vp := r.cueBus().Voice.View
	if x, y, ok := r.pipe().Creatures.OnThreatPause(r.liveView(), w, r.game.Snapshot(), tick, vp); ok {
		r.centerOnView(x, y, vp, vp)
	}
}

// speedRate is the HUD speed label, saying why the game paused itself.
func (r *Renderer) speedRate(tr ...lang.Tr) string {
	if r.threatPaused && r.speed == speedPaused {
		return firstTr(tr).T("paused (threat)")
	}
	r.threatPaused = false
	return r.speed.rate(tr...)
}
