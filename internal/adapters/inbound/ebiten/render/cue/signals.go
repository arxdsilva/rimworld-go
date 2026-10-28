package cue

import (
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// Sound cues from the colony renderer (SND-001). The renderer only notes
// what happened and where; the App's sound director (sound_director.go)
// decides what is heard. Everything here reads presentation state, never
// the domain.

// ActionSounds is the sound of each work action's strike; actions not
// listed (eating, research, tending...) are silent.
var ActionSounds = map[string]string{
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

// Memo is last frame's world, to hear what changed.
type Memo struct {
	init      bool
	frames    map[entity.EntityID][2]int
	cutPlants map[[2]int]entity.EntityID // the plant on each tile a pawn was cutting last frame
	mines     map[[2]int]uint8           // marked tile -> its terrain code
	smooths   map[[2]int]uint8
	doorsOpen map[[2]int]bool
	notice    uint64                  // newest non-danger message heard
	plantBuf  []int32                 // plantOn's PlantsIn buffer
	Pods      map[[3]uint64]podSounds // arrival pods heard so far (listenArrivals)
	power     powerHeard              // the electric grid last frame (listenPower)
}

// ListenWorld cues builds finishing, trees falling, rock mined out or
// smoothed and doors opening or closing, from the difference with the
// last frame. A mine or smooth mark that vanished cues only when its
// tile's terrain changed (a cancelled mark is silent).
func (b *Bus) ListenWorld(v *render.View, w game.WorldView, pawns []game.PawnView) {
	m := &b.Memo
	b.Host.ListenCaskets(w) // vault_draw.go: a casket turning open
	b.listenPower(v, w)     // switches thrown, short circuits, solar flares
	frames := make(map[entity.EntityID][2]int, len(w.Frames))
	for _, f := range w.Frames {
		frames[f.ID] = [2]int{f.X, f.Y}
	}
	marks := func(ts [][2]int) map[[2]int]uint8 {
		out := make(map[[2]int]uint8, len(ts))
		for _, t := range ts {
			out[t], _ = b.Host.Terrain(w, t[0], t[1])
		}
		return out
	}
	mines, smooths := marks(w.Mines), marks(w.Smooths)
	standing := map[[2]int]bool{}
	chopped := map[[2]int]bool{}
	for _, p := range pawns {
		standing[[2]int{p.X, p.Y}] = true
		if b.Host.Chopping(p) {
			chopped[[2]int{p.ActX, p.ActY}] = true
		}
	}
	// the plant on each tile being cut, from that tile alone (no pass over
	// every plant on the map, NG-1 §6.7)
	cut := make(map[[2]int]entity.EntityID, len(chopped))
	for t := range chopped {
		if id, ok := b.plantOn(&w, t); ok {
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
				b.Cue("sfx.build_done", float64(at[0])+0.5, float64(at[1])+0.5, 1)
			}
		}
		for at, id := range m.cutPlants { // a plant cut last frame is gone
			if now, ok := b.plantOn(&w, at); !ok || now != id {
				b.Cue("sfx.tree_fall", float64(at[0])+0.5, float64(at[1])+0.5, 1)
			}
		}
		done := func(was map[[2]int]uint8, now map[[2]int]uint8, id string) {
			for t, code := range was {
				if _, still := now[t]; still {
					continue
				}
				if c, ok := b.Host.Terrain(w, t[0], t[1]); ok && c != code {
					b.Cue(id, float64(t[0])+0.5, float64(t[1])+0.5, 1)
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
				b.Cue(id, float64(t[0])+0.5, float64(t[1])+0.5, 1)
			}
		}
		if notice > m.notice {
			b.Global("ui.notice")
		}
	}
	*m = Memo{init: true, frames: frames, cutPlants: cut, mines: mines, smooths: smooths, doorsOpen: doors,
		notice: notice, plantBuf: m.plantBuf, Pods: m.Pods, power: m.power}
	b.ListenArrivals(v, w)
}

// plantOn is the plant on tile t of w (Simulation.PlantsIn: that tile's
// chunk only).
func (b *Bus) plantOn(w *game.WorldView, t [2]int) (entity.EntityID, bool) {
	m := &b.Memo
	m.plantBuf = b.Host.PlantsOn(w.Plants, t[0], t[1], t[0], t[1], m.plantBuf[:0])
	for _, i := range m.plantBuf {
		if int(i) < len(w.Plants) {
			return w.Plants[i].ID, true
		}
	}
	return 0, false
}

// TraceSound is the sound of a new attack trace, heard where it lands
// (shots where they are fired).
func (b *Bus) TraceSound(t game.TraceView) {
	voiced := false
	if sp, ok := b.Voice.ByID[t.Attacker]; ok && t.Attacker != 0 {
		ax, ay := float64(t.FromX)+0.5, float64(t.FromY)+0.5
		voiced = b.CueAnimal(sp, VoiceAttack, ax, ay, VolAttack)
	}
	if sp, ok := b.Voice.ByID[t.Target]; ok && t.Target != 0 && t.Hit && !t.Building {
		tx, ty := float64(t.ToX)+0.5, float64(t.ToY)+0.5
		voiced = b.CueAnimal(sp, VoiceHurt, tx, ty, VolHurt) || voiced
	}
	switch {
	case t.Hit && t.Building:
		b.Cue("sfx.wood_hit", float64(t.ToX)+0.5, float64(t.ToY)+0.5, 1)
	case !t.Melee:
		b.Cue("sfx.gunshot", float64(t.FromX)+0.5, float64(t.FromY)+0.5, 1)
	case t.Hit:
		vol := 1.0
		if voiced {
			vol = VolMeleeUnder // the voice leads, the impact sits under it
		}
		b.Cue("sfx.melee_hit", float64(t.ToX)+0.5, float64(t.ToY)+0.5, vol)
	}
}

// ThunderDelay is how long after a flash its thunder rolls in, frames.
func ThunderDelay(bucket uint64) uint64 { return 20 + art.Mix64(bucket^0x7417)%70 }

// ListenThunder plays thunder once its delay after a flash has passed.
func (b *Bus) ListenThunder(v *render.View) {
	if b.ThunderAt != 0 && v.Present >= b.ThunderAt {
		b.ThunderAt = 0
		b.Global("sfx.thunder")
	}
}

// PodFallLead is how long before landing the pod's descent is heard, ticks:
// the length of sfx.pod_fall at the normal speed, so the thud lands on the
// last note of the whistle.
const PodFallLead = 84

// podSounds is which of a pod's three sounds have been cued.
type podSounds struct{ fell, landed, opened bool }

// ListenArrivals cues each arrival pod of the world (NG-1 §7.4): the fall
// when its descent starts, the landing at the stamp's tick, and the hatch
// when the pod's Landed flag flips. A pod first seen already past a stage
// is silent for it (a loaded game, a camera that was elsewhere), so each
// sound is heard once, in order. It runs only while time passes.
func (b *Bus) ListenArrivals(v *render.View, w game.WorldView) {
	m := &b.Memo
	now := uint64(v.Tick)
	seen := make(map[[3]uint64]podSounds, len(w.Arrivals))
	for _, p := range w.Arrivals {
		key := [3]uint64{uint64(p.X), uint64(p.Y), p.Tick}
		st, known := m.Pods[key]
		if !known { // first seen past a stage: silent for it
			st = podSounds{fell: now >= p.Tick, landed: now >= p.Tick, opened: p.Landed}
		}
		x, y := float64(p.X)+0.5, float64(p.Y)+0.5
		if !st.fell && now+PodFallLead >= p.Tick {
			st.fell = true
			b.Cue("sfx.pod_fall", x, y, 1)
		}
		if !st.landed && now >= p.Tick {
			st.landed = true
			b.Cue("sfx.pod_land", x, y, 1)
		}
		if !st.opened && p.Landed {
			st.opened = true
			b.Cue("sfx.pod_open", x, y, 1)
		}
		seen[key] = st
	}
	m.Pods = seen
}

// --- electricity (PLAY-021) ---------------------------------------------------

// letterShortCircuit is the title of the letter a short circuit sends
// (power_events.go); it is the one thing that makes one, so it is the
// signal: deconstructing a conduit sends no letter.
const letterShortCircuit = "Short circuit"

// letterFreshTicks is how recent a letter must be to be heard; a letter
// older than this turned up in the stack some other way (a loaded save).
const letterFreshTicks = 600

var sndSwitchKind = construction.PowerSwitch.String()

// powerHeard is last frame's electrical state, to hear what changed.
type powerHeard struct {
	seen     bool                     // a first frame has been recorded; before it nothing is heard
	switches map[entity.EntityID]bool // each power switch's PowerOn last frame
	scratch  map[entity.EntityID]bool // this frame's, swapped in at the end (no allocation per frame)
	letter   uint64                   // newest letter id heard
	flare    bool                     // a solar flare was on
}

// listenPower cues a switch clicking where it stands when PowerOn flips
// (the switch moving, not the player's order for it, which a colonist
// carries out later), a short circuit zapping where its letter says, and
// one global buzz as a solar flare begins. Whatever is already there the
// first time it looks (a loaded game) is silent.
func (b *Bus) listenPower(v *render.View, w game.WorldView) {
	p := &b.Memo.power
	cur := p.scratch
	if cur == nil {
		cur = map[entity.EntityID]bool{}
	}
	clear(cur)
	for _, bd := range w.Buildings {
		if bd.Kind != sndSwitchKind {
			continue
		}
		cur[bd.ID] = bd.PowerOn
		if was, known := p.switches[bd.ID]; p.seen && known && was != bd.PowerOn {
			b.Cue("sfx.switch_click", float64(bd.X)+0.5, float64(bd.Y)+0.5, 1)
		}
	}
	p.switches, p.scratch = cur, p.switches
	now := uint64(v.Tick)
	newest := p.letter
	for _, l := range w.Letters {
		if l.ID <= p.letter {
			continue
		}
		newest = max(newest, l.ID)
		fresh := now < l.Tick || now-l.Tick <= letterFreshTicks
		if p.seen && l.Title == letterShortCircuit && l.X >= 0 && l.Y >= 0 && fresh {
			b.Cue("sfx.short_circuit", float64(l.X)+0.5, float64(l.Y)+0.5, 1)
		}
	}
	p.letter = newest
	if p.seen && w.SolarFlare && !p.flare {
		b.Global("ui.solar_flare")
	}
	p.flare, p.seen = w.SolarFlare, true
}
