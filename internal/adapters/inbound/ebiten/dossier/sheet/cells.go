package sheet

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strconv"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/domain/pawn/skill"
)

// SkillState is how a skill reads, in the order of priority: off beats
// saturated beats active beats a major passion beats a minor one.
type SkillState uint8

const (
	SkillPlain  SkillState = iota
	SkillLike              // minor passion
	SkillLove              // major passion
	SkillActive            // in use right now
	SkillFull              // learning is capped for today
	SkillOff               // the pawn cannot do it
)

// Key is the colour key of a state.
func (s SkillState) Key() string {
	switch s {
	case SkillLike:
		return "skill_like"
	case SkillLove:
		return "skill_love"
	case SkillActive:
		return "skill_busy"
	case SkillFull:
		return "skill_full"
	case SkillOff:
		return "text_off"
	}
	return "text"
}

// SkillStateOf picks a skill's state. A skill the card does not know (no
// index) can only be off or carry a passion.
func SkillStateOf(s game.SkillView, c dossierview.Card) SkillState {
	idx := -1
	if id, ok := skill.Parse(s.Key); ok {
		idx = int(id)
	}
	switch {
	case s.Incapable:
		return SkillOff
	case idx >= 0 && c.Saturated[idx]:
		return SkillFull
	case idx >= 0 && c.ActiveSkill == idx:
		return SkillActive
	case s.Passion >= 2:
		return SkillLove
	case s.Passion == 1:
		return SkillLike
	}
	return SkillPlain
}

// SkillValue is the number shown: a dash when disabled, the level, or with
// decimals the level and its progress capped at 99 ("12.45").
func SkillValue(s game.SkillView, decimals bool) string {
	switch {
	case s.Incapable:
		return "-"
	case decimals && s.Level < skill.MaxLevel:
		frac := min(max(s.XPPct, 0), 99)
		f := strconv.Itoa(frac)
		if frac < 10 {
			f = "0" + f
		}
		return strconv.Itoa(s.Level) + "." + f
	}
	return strconv.Itoa(s.Level)
}

// SkillCell builds one skill entry. Its Tip leaves out the weapon line;
// SkillTip with the weapon text adds it for the melee skill.
func SkillCell(tr lang.Tr, s game.SkillView, c dossierview.Card, decimals bool) Cell {
	st := SkillStateOf(s, c)
	cell := Cell{Label: s.Label, Value: SkillValue(s, decimals), Pct: s.Level * 100 / skill.MaxLevel,
		Tip: JoinTip(SkillTip(tr, s, c, "")), Active: st == SkillActive}
	if st == SkillOff {
		cell.Tone = ToneOff
		cell.Pct = 0
	}
	return cell
}

// TrainCell builds one training entry. A behaviour the species cannot learn
// gives the zero Cell. Not wanted reads off; learned reads good and active
// (bold).
func TrainCell(tr lang.Tr, t game.AnimalTrainingView) Cell {
	if !t.Allowed {
		return Cell{}
	}
	cell := Cell{Label: t.Label, Value: lang.Tf(tr, "%d of %d", t.Steps, t.Needed), Tip: JoinTip(trainLines(tr, t))}
	if t.Needed > 0 {
		cell.Pct = min(t.Steps*100/t.Needed, 100)
	}
	switch {
	case !t.On && !t.Learned:
		cell.Tone = ToneOff
	case t.Learned:
		cell.Tone, cell.Active, cell.Value, cell.Pct = ToneGood, true, tr.T("Learned"), 100
	}
	return cell
}
