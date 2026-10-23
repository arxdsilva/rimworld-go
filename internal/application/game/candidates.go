package game

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/scenario"
)

// Character creation (NG-1 §2.9, §5.4): the candidate pool exists before
// any Game. Owned by PAWN-G.

// CandidatePool is the Create Characters roster. Every candidate keeps
// the generation slot it was created in, so rerolls stay deterministic
// from (seed, slot, roll) however the roster is reordered (NG-1 §4.7).
type CandidatePool struct {
	seed   uint64
	fx     scenario.Effects
	pawns  []pawngen.Pawn
	slots  []int  // generation slot of each roster position
	rolls  []int  // roll counter of each roster position
	edited []bool // the player renamed this candidate
	sel    int    // the first sel candidates are Selected
}

// MaxNameRunes is the longest first, nick or last name.
const MaxNameRunes = 16

// Rename errors (the screen shows err.Error()).
var (
	//lint:ignore ST1005 shown to the player as a sentence
	ErrNickRequired = errors.New("Every colonist needs a nickname")
	//lint:ignore ST1005 shown to the player as a sentence
	ErrNameTooLong = errors.New("Names are at most 16 characters")
	//lint:ignore ST1005 shown to the player as a sentence
	ErrNameChars = errors.New("Names use only letters, spaces, ' and -")
	//lint:ignore ST1005 shown to the player as a sentence
	ErrNickTaken = errors.New("Another colonist already has that nickname")
)

// defaultExtra is how many more candidates than People the pool holds
// when the scenario leaves "choose from" unset.
const defaultExtra = 5

// NewCandidatePool generates ChooseFrom candidates; the first People are
// Selected. ChooseFrom 0 means People+5; it is never below People.
func NewCandidatePool(seed uint64, fx scenario.Effects) *CandidatePool {
	people := max(fx.People, 0)
	n := fx.ChooseFrom
	if n == 0 {
		n = people + defaultExtra
	}
	n = max(n, people)
	p := &CandidatePool{
		seed:   seed,
		fx:     fx,
		pawns:  make([]pawngen.Pawn, n),
		slots:  make([]int, n),
		rolls:  make([]int, n),
		edited: make([]bool, n),
		sel:    people,
	}
	for i := range n {
		p.slots[i] = i
		// Later candidates avoid the nicknames of earlier ones (slots not yet
		// generated have empty names, which taken() skips).
		p.pawns[i] = pawngen.Generate(seed, i, 0, p.rules(i))
	}
	return p
}

// rules are the scenario's rules with Taken holding every nickname in use
// by the colony (fx.Rules.Taken) and by the other candidates.
func (p *CandidatePool) rules(skip int) pawngen.Rules {
	r := p.fx.Rules
	r.Taken = p.taken(skip)
	return r
}

func (p *CandidatePool) taken(skip int) map[string]bool {
	t := make(map[string]bool, len(p.fx.Rules.Taken)+len(p.pawns))
	for k, v := range p.fx.Rules.Taken {
		if v {
			t[k] = true
		}
	}
	for i, c := range p.pawns {
		if i != skip && c.Nick != "" {
			t[c.Nick] = true
		}
	}
	return t
}

func (p *CandidatePool) valid(i int) bool { return i >= 0 && i < len(p.pawns) }

// Views are every candidate's bio, in roster order.
func (p *CandidatePool) Views() []CandidateView {
	out := make([]CandidateView, len(p.pawns))
	for i, c := range p.pawns {
		out[i] = CandidateView{BioView: bioOf(c), Selected: i < p.sel, Index: i}
	}
	return out
}

// Reroll regenerates candidate i from its slot's next roll. With keepName
// an edited name survives; otherwise (or if the name was never edited) the
// new roll's name is used and the candidate counts as unedited again.
func (p *CandidatePool) Reroll(i int, keepName bool) {
	if !p.valid(i) {
		return
	}
	p.rolls[i]++
	np := pawngen.Generate(p.seed, p.slots[i], p.rolls[i], p.rules(i))
	if keepName && p.edited[i] {
		old := p.pawns[i]
		np.First, np.Nick, np.Last = old.First, old.Nick, old.Last
		np.Temperament = temperamentOf(np)
	} else {
		p.edited[i] = false
	}
	p.pawns[i] = np
}

// RandomizeAll rerolls every candidate in roster order, keeping edited
// names.
func (p *CandidatePool) RandomizeAll() {
	for i := range p.pawns {
		p.Reroll(i, true)
	}
}

// Rename sets candidate i's names (trimmed). The nickname is required;
// each name is at most MaxNameRunes runes of letters, spaces, apostrophes
// and hyphens; nicknames are unique (ignoring case) among the candidates
// and the colony.
func (p *CandidatePool) Rename(i int, first, nick, last string) error {
	if !p.valid(i) {
		return errors.New("no such candidate")
	}
	first, nick, last = strings.TrimSpace(first), strings.TrimSpace(nick), strings.TrimSpace(last)
	if nick == "" {
		return ErrNickRequired
	}
	for _, s := range []string{first, nick, last} {
		if err := checkName(s); err != nil {
			return err
		}
	}
	for k := range p.taken(i) {
		if strings.EqualFold(k, nick) {
			return ErrNickTaken
		}
	}
	c := &p.pawns[i]
	c.First, c.Nick, c.Last = first, nick, last
	c.Temperament = temperamentOf(*c)
	p.edited[i] = true
	return nil
}

func checkName(s string) error {
	if utf8.RuneCountInString(s) > MaxNameRunes {
		return ErrNameTooLong
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && r != ' ' && r != '\'' && r != '-' {
			return ErrNameChars
		}
	}
	return nil
}

// temperamentOf mirrors pawngen's rule (Sunny → Social, Brooding →
// Homebody, otherwise the nickname hash) so a renamed candidate gets the
// temperament it would have been generated with.
func temperamentOf(c pawngen.Pawn) pawngen.Temperament {
	for _, t := range c.Traits {
		switch t {
		case trait.Sunny:
			return pawngen.SocialType
		case trait.Brooding:
			return pawngen.Homebody
		}
	}
	return pawngen.TemperamentOfName(c.Nick)
}

// Move takes candidate i out of the roster and reinserts it at j. The
// first People positions are Selected, so moving a Selected candidate
// below them pulls the top left-behind one in.
func (p *CandidatePool) Move(i, j int) {
	if !p.valid(i) || !p.valid(j) || i == j {
		return
	}
	move(p.pawns, i, j)
	move(p.slots, i, j)
	move(p.rolls, i, j)
	move(p.edited, i, j)
}

func move[T any](s []T, i, j int) {
	v := s[i]
	if i < j {
		copy(s[i:j], s[i+1:j+1])
	} else {
		copy(s[j+1:i+1], s[j:i])
	}
	s[j] = v
}

// Swap exchanges candidates i and j.
func (p *CandidatePool) Swap(i, j int) {
	if !p.valid(i) || !p.valid(j) {
		return
	}
	p.pawns[i], p.pawns[j] = p.pawns[j], p.pawns[i]
	p.slots[i], p.slots[j] = p.slots[j], p.slots[i]
	p.rolls[i], p.rolls[j] = p.rolls[j], p.rolls[i]
	p.edited[i], p.edited[j] = p.edited[j], p.edited[i]
}

// Team warning texts (NG-1 §2.9), in the order Team reports them.
const (
	WarnNoFighter   = "Nobody can fight"
	WarnNoDoctor    = "No doctor"
	WarnNoCook      = "No cook"
	WarnNoGrower    = "No grower"
	WarnNoBuilder   = "No builder"
	WarnNoResearch  = "No researcher"
	WarnYoungOrOld  = "Everyone is very young or old"
	teamMinLevel    = 3
	veryYoungBefore = 18 // years; under this is very young
	veryOldFrom     = 65 // years; at or over this is very old
)

// Team is the Selected colonists' best level per skill (and the first, in
// roster order, who has it; "" when every one is incapable) and the gaps.
func (p *CandidatePool) Team() TeamReport {
	sel := p.pawns[:p.sel]
	var r TeamReport
	r.Best = make([]TeamSkill, skill.NumSkills)
	for id := skill.ID(0); id < skill.NumSkills; id++ {
		ts := TeamSkill{Key: id.Key(), Label: id.Label()}
		for _, c := range sel {
			if c.Incapable().DisablesSkill(id) {
				continue
			}
			if l := int(c.Skills[id].Level); ts.Who == "" || l > ts.Level {
				ts.Level, ts.Who = l, c.Nick
			}
		}
		r.Best[id] = ts
	}
	if len(sel) == 0 {
		return r
	}
	allIncapable := func(b backstory.Bits) bool {
		for _, c := range sel {
			if c.Incapable()&b == 0 {
				return false
			}
		}
		return true
	}
	low := func(id skill.ID) bool { return r.Best[id].Level < teamMinLevel }
	allAgeExtreme := true
	for _, c := range sel {
		if y := c.AgeYears(); y >= veryYoungBefore && y < veryOldFrom {
			allAgeExtreme = false
		}
	}
	for _, w := range []struct {
		hit  bool
		text string
	}{
		{allIncapable(backstory.Violence), WarnNoFighter},
		{low(skill.Medicine) || allIncapable(backstory.Caring), WarnNoDoctor},
		{low(skill.Cooking) || allIncapable(backstory.Cooking), WarnNoCook},
		{low(skill.Plants), WarnNoGrower},
		{low(skill.Construction), WarnNoBuilder},
		{low(skill.Intellectual) || allIncapable(backstory.Intellectual), WarnNoResearch},
		{allAgeExtreme, WarnYoungOrOld},
	} {
		if w.hit {
			r.Warnings = append(r.Warnings, w.text)
		}
	}
	return r
}

// Selected are the Selected colonists in roster order, with edited names
// (independent copies).
func (p *CandidatePool) Selected() []pawngen.Pawn {
	out := make([]pawngen.Pawn, p.sel)
	for i := range out {
		c := p.pawns[i]
		c.Traits = append([]trait.ID(nil), c.Traits...)
		c.Conditions = append([]pawngen.Condition(nil), c.Conditions...)
		out[i] = c
	}
	return out
}
