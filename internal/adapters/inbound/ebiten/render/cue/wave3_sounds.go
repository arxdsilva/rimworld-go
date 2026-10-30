package cue

import (
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Wave 3 sounds (CBT-3, CBT-4, STO-2). The weapon shots, the gun-stock blow
// and the smokepop burst ride weaponVoices, combatTrace and blastSounds
// (combat_sounds.go). This file hears state that turns on: a stagger, a
// tantrum, a paste dispenser's job and a shield belt breaking.

const (
	tantrumBreak = "tantrum"        // PawnView.MentalBreak of a tantrum (CD, game/mind)
	dispenseJob  = "dispense paste" // job.DispensePaste's line, PawnView.Job
	wave3MemoMax = 4096
)

// wave3 flags, one bit per state heard last frame.
const (
	wStaggered uint8 = 1 << iota
	wTantrum
	wDispensing
)

// wave3Memo is what these cues remember: each creature's flags and each
// shield's charge last frame. The maps are reused; nothing is allocated
// once warm.
type wave3Memo struct {
	flags, next map[entity.EntityID]uint8
	shields     map[entity.EntityID]int
	seen        bool // a first frame was heard (what is already so is not new)
}

// ShieldState is one wearer's shield belt: its charge in percent (0 once
// it broke) and the wearer's tile. The renderer feeds it from
// PawnView.ShieldPct once CC adds that field.
type ShieldState struct {
	ID   entity.EntityID
	Pct  int
	X, Y int
}

// listenWave3 cues a stagger, a tantrum or a dispenser job the moment it
// starts, once each (the flag must clear before it can sound again).
func (b *Bus) listenWave3(w game.WorldView, pawns []game.PawnView) {
	m := &b.Wave3
	if m.flags == nil {
		m.flags, m.next = map[entity.EntityID]uint8{}, map[entity.EntityID]uint8{}
	}
	clear(m.next)
	hear := func(id entity.EntityID, f uint8, x, y int, sound string) {
		if m.flags[id]&f == 0 && m.seen {
			b.Cue(sound, float64(x)+0.5, float64(y)+0.5, 1)
		}
		m.next[id] |= f
	}
	for i := range pawns {
		p := &pawns[i]
		if p.Staggered {
			hear(p.ID, wStaggered, p.X, p.Y, "sfx.stagger_thud")
		}
		if p.MentalBreak == tantrumBreak {
			hear(p.ID, wTantrum, p.X, p.Y, "sfx.tantrum_smash")
		}
		if strings.HasPrefix(p.Job, dispenseJob) {
			hear(p.ID, wDispensing, p.X, p.Y, "sfx.paste_dispense")
		}
	}
	for i := range w.Animals {
		if a := &w.Animals[i]; a.Staggered {
			hear(a.ID, wStaggered, a.X, a.Y, "sfx.stagger_thud")
		}
	}
	m.flags, m.next = m.next, m.flags
	m.seen = true
}

// ListenShields cues each shield belt that broke (its charge fell to 0
// from above 0) once.
func (b *Bus) ListenShields(shields []ShieldState) {
	m := &b.Wave3
	if m.shields == nil || len(m.shields) > wave3MemoMax {
		m.shields = map[entity.EntityID]int{}
	}
	for i := range shields {
		s := &shields[i]
		if was := m.shields[s.ID]; was > 0 && s.Pct <= 0 {
			b.Cue("sfx.shield_pop", float64(s.X)+0.5, float64(s.Y)+0.5, 1)
		}
		m.shields[s.ID] = s.Pct
	}
}
