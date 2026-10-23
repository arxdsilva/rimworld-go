package ebiten

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Sound cues from the colony renderer (SND-001). The renderer only notes
// what happened and where; the App's sound director (sound_director.go)
// decides what is heard. Everything here reads presentation state, never
// the domain.

// soundCue is one thing to hear: a sound id at a world point (tiles), or
// global (interface sounds, thunder) with no position.
type soundCue struct {
	id     string
	x, y   float64
	vol    float64
	global bool
}

// actionSounds is the sound of each work action's strike; actions not
// listed (eating, research, tending...) are silent.
var actionSounds = map[string]string{
	game.ActionChop:        "sfx.chop",
	game.ActionMine:        "sfx.pick",
	game.ActionSmooth:      "sfx.pick",
	game.ActionBreach:      "sfx.pick",
	game.ActionDig:         "sfx.soil",
	game.ActionBury:        "sfx.soil",
	game.ActionSow:         "sfx.soil",
	game.ActionBuild:       "sfx.hammer",
	game.ActionRepair:      "sfx.hammer",
	game.ActionDeconstruct: "sfx.hammer",
	game.ActionHarvest:     "sfx.harvest",
	game.ActionCraft:       "sfx.saw",
	game.ActionTailor:      "sfx.saw",
	game.ActionButcher:     "sfx.cleave",
	game.ActionRefuel:      "sfx.wood_drop",
	game.ActionClean:       "sfx.sweep",
}

func (r *Renderer) cue(id string, x, y, vol float64) {
	r.cues = append(r.cues, soundCue{id: id, x: x, y: y, vol: vol})
}

func (r *Renderer) globalCue(id string) {
	r.cues = append(r.cues, soundCue{id: id, vol: 1, global: true})
}

// takeCues hands this frame's cues to the director.
func (r *Renderer) takeCues() []soundCue {
	c := r.cues
	r.cues = nil
	return c
}

// soundMemo is last frame's world, to hear what changed.
type soundMemo struct {
	init      bool
	frames    map[entity.EntityID][2]int
	cutPlants map[[2]int]entity.EntityID // the plant on each tile a pawn was cutting last frame
	mines     map[[2]int]uint8           // marked tile -> its terrain code
	smooths   map[[2]int]uint8
	doorsOpen map[[2]int]bool
	notice    uint64  // newest non-danger message heard
	plantBuf  []int32 // plantOn's PlantsIn buffer
}

// listenWorld cues builds finishing, trees falling, rock mined out or
// smoothed and doors opening or closing, from the difference with the
// last frame. A mine or smooth mark that vanished cues only when its
// tile's terrain changed (a cancelled mark is silent).
func (r *Renderer) listenWorld(w game.WorldView, pawns []game.PawnView) {
	m := &r.memo
	frames := make(map[entity.EntityID][2]int, len(w.Frames))
	for _, f := range w.Frames {
		frames[f.ID] = [2]int{f.X, f.Y}
	}
	marks := func(ts [][2]int) map[[2]int]uint8 {
		out := make(map[[2]int]uint8, len(ts))
		for _, t := range ts {
			out[t], _ = terrainCode(w, t[0], t[1])
		}
		return out
	}
	mines, smooths := marks(w.Mines), marks(w.Smooths)
	standing := map[[2]int]bool{}
	chopped := map[[2]int]bool{}
	for _, p := range pawns {
		standing[[2]int{p.X, p.Y}] = true
		if act, _, ok := actionOf(p); ok && act == game.ActionChop {
			chopped[[2]int{p.ActX, p.ActY}] = true
		}
	}
	// the plant on each tile being cut, from that tile alone (no pass over
	// every plant on the map, NG-1 §6.7)
	cut := make(map[[2]int]entity.EntityID, len(chopped))
	for t := range chopped {
		if id, ok := r.plantOn(&w, t); ok {
			cut[t] = id
		}
	}
	doors := map[[2]int]bool{}
	built := map[[2]int]bool{}
	for _, b := range w.Buildings {
		built[[2]int{b.X, b.Y}] = true
		if b.Kind == "door" {
			doors[[2]int{b.X, b.Y}] = standing[[2]int{b.X, b.Y}]
		}
	}
	notice := m.notice
	for _, msg := range w.Messages {
		if !msg.Danger && msg.Tick > notice {
			notice = msg.Tick
		}
	}
	if m.init {
		for id, at := range m.frames {
			if _, ok := frames[id]; !ok && built[at] {
				r.cue("sfx.build_done", float64(at[0])+0.5, float64(at[1])+0.5, 1)
			}
		}
		for at, id := range m.cutPlants { // a plant cut last frame is gone
			if now, ok := r.plantOn(&w, at); !ok || now != id {
				r.cue("sfx.tree_fall", float64(at[0])+0.5, float64(at[1])+0.5, 1)
			}
		}
		done := func(was map[[2]int]uint8, now map[[2]int]uint8, id string) {
			for t, code := range was {
				if _, still := now[t]; still {
					continue
				}
				if c, ok := terrainCode(w, t[0], t[1]); ok && c != code {
					r.cue(id, float64(t[0])+0.5, float64(t[1])+0.5, 1)
				}
			}
		}
		done(m.mines, mines, "sfx.rock_break")
		done(m.smooths, smooths, "sfx.pick")
		for t, open := range doors {
			if was, ok := m.doorsOpen[t]; ok && was != open {
				id := "sfx.door_close"
				if open {
					id = "sfx.door_open"
				}
				r.cue(id, float64(t[0])+0.5, float64(t[1])+0.5, 1)
			}
		}
		if notice > m.notice {
			r.globalCue("ui.notice")
		}
	}
	*m = soundMemo{init: true, frames: frames, cutPlants: cut, mines: mines, smooths: smooths, doorsOpen: doors,
		notice: notice, plantBuf: m.plantBuf}
}

// plantOn is the plant on tile t of w (Simulation.PlantsIn: that tile's
// chunk only).
func (r *Renderer) plantOn(w *game.WorldView, t [2]int) (entity.EntityID, bool) {
	m := &r.memo
	m.plantBuf = r.plantsOn(w, t[0], t[1], t[0], t[1], m.plantBuf[:0])
	for _, i := range m.plantBuf {
		if int(i) < len(w.Plants) {
			return w.Plants[i].ID, true
		}
	}
	return 0, false
}

// traceSound is the sound of a new attack trace, heard where it lands
// (shots where they are fired).
func (r *Renderer) traceSound(t game.TraceView) {
	voiced := false
	if an, ok := r.voice.byID[t.Attacker]; ok && t.Attacker != 0 {
		ax, ay := float64(t.FromX)+0.5, float64(t.FromY)+0.5
		voiced = r.cueAnimal(an.Species, voiceAttack, ax, ay, volAttack)
	}
	if an, ok := r.voice.byID[t.Target]; ok && t.Target != 0 && t.Hit && !t.Building {
		tx, ty := float64(t.ToX)+0.5, float64(t.ToY)+0.5
		voiced = r.cueAnimal(an.Species, voiceHurt, tx, ty, volHurt) || voiced
	}
	switch {
	case t.Hit && t.Building:
		r.cue("sfx.wood_hit", float64(t.ToX)+0.5, float64(t.ToY)+0.5, 1)
	case !t.Melee:
		r.cue("sfx.gunshot", float64(t.FromX)+0.5, float64(t.FromY)+0.5, 1)
	case t.Hit:
		vol := 1.0
		if voiced {
			vol = volMeleeUnder // the voice leads, the impact sits under it
		}
		r.cue("sfx.melee_hit", float64(t.ToX)+0.5, float64(t.ToY)+0.5, vol)
	}
}

// thunderDelay is how long after a flash its thunder rolls in, frames.
func thunderDelay(bucket uint64) uint64 { return 20 + mix64(bucket^0x7417)%70 }

// listenThunder plays thunder once its delay after a flash has passed.
func (r *Renderer) listenThunder() {
	if r.thunderAt != 0 && r.present >= r.thunderAt {
		r.thunderAt = 0
		r.globalCue("sfx.thunder")
	}
}
