package body

// Conditions (MED-00): whatever ails a body besides its wounds — an
// infection on a part, a disease, an anaesthetic, an age-related illness.
// A condition carries its own severity, immunity and tend progress; its
// definition (CondDef, cond_kinds.go) says how they move and what each
// stage does to the capacities and to pain. The owner of a condition (the
// medical phase) advances it and sets Stage (with its own hysteresis);
// Capacity and Pain only read the current stage.

// CondKind is a kind of condition (append only, saved). The kinds and
// their definitions are listed in cond_kinds.go.
type CondKind uint8

// Condition is one condition on a body (saved). Sev, Immunity, TendQ and
// TendSum are permille: severity, immunity gained, the last tend's quality
// and the sum of every tend's quality (for conditions cured by tending).
// TendUntil is the tick the last tend stops helping, Since the tick it
// began; Stage indexes its definition's Stages.
type Condition struct {
	Kind                          CondKind
	Part                          Part
	Sev, Immunity, TendQ, TendSum int32
	TendUntil, Since              uint64
	Stage                         uint8
	// Rate is a condition's own hourly rate where the game steps it (the
	// malnutrition rate rolled per pawn, tenths of a permille); 0 else.
	Rate int32
}

// CapMod changes one capacity: Off permille is added and, with HasMax, the
// result is capped at Max permille.
type CapMod struct {
	Cap    Capacity
	Off    int32
	Max    int32
	HasMax bool
}

// CapMods are a stage's capacity changes, applied in order.
type CapMods []CapMod

// Stage is one band of a condition's severity, from At permille up to the
// next stage's At: its label, capacity changes and added pain (permille).
type Stage struct {
	At      int32
	Label   string
	Mods    CapMods
	PainAdd int32
}

// CondDef describes a kind of condition. Rates are permille per day:
// severity while not immune (SevPerDay, negative recedes), severity once
// immune (ImmuneSevPerDay) and immunity gained (ImmPerDay). A tend lowers
// the daily severity by up to TendMax at full quality and lasts TendEvery
// hours. Lethal conditions kill at full severity; NoImmunity ones never
// build immunity, and with CureTendSum set they are cured once the summed
// tend quality reaches it.
type CondDef struct {
	Label                                          string
	SevPerDay, ImmuneSevPerDay, ImmPerDay, TendMax int32
	TendEvery                                      int
	Stages                                         []Stage
	Lethal, NoImmunity                             bool
	CureTendSum                                    int32
}

// DefOfCond is the definition of kind k (the zero CondDef for CondNone
// and unknown kinds).
func DefOfCond(k CondKind) CondDef {
	if k >= NumCondKinds {
		return CondDef{}
	}
	return condDefs[k]
}

// StageAt is the stage a severity falls in: the last one whose At it has
// reached (0 when there are no stages).
func (d CondDef) StageAt(sev int32) uint8 {
	var s uint8
	for i, st := range d.Stages {
		if sev >= st.At {
			s = uint8(i)
		}
	}
	return s
}

// condLookup finds a kind's definition (DefOfCond; tests pass their own).
type condLookup func(CondKind) CondDef

// stageOf is c's current stage under look (false: none, or out of range).
func stageOf(c Condition, look condLookup) (Stage, bool) {
	st := look(c.Kind).Stages
	if int(c.Stage) >= len(st) {
		return Stage{}, false
	}
	return st[c.Stage], true
}

// AddCond adds c and reports whether it did: CondNone is no condition,
// and a body holds one condition of a kind per part (a second is refused;
// change the first through Cond). A kind with no definition is kept but
// has no effect.
func (b *Body) AddCond(c Condition) bool {
	if c.Kind == CondNone {
		return false
	}
	for _, x := range b.Conds {
		if x.Kind == c.Kind && x.Part == c.Part {
			return false
		}
	}
	b.Conds = append(b.Conds, c)
	return true
}

// Cond is the body's first condition of kind k, live (changes stick).
func (b *Body) Cond(k CondKind) (*Condition, bool) {
	for i := range b.Conds {
		if b.Conds[i].Kind == k {
			return &b.Conds[i], true
		}
	}
	return nil, false
}

// RemoveCond removes every condition of kind k and reports whether there
// was one; Conds goes back to nil when none are left.
func (b *Body) RemoveCond(k CondKind) bool {
	kept := b.Conds[:0]
	for _, c := range b.Conds {
		if c.Kind != k {
			kept = append(kept, c)
		}
	}
	removed := len(kept) != len(b.Conds)
	clear(b.Conds[len(kept):])
	b.Conds = kept
	if len(b.Conds) == 0 {
		b.Conds = nil
	}
	return removed
}

// condPain is the pain the conditions' current stages add (permille).
func (b *Body) condPain(look condLookup) int32 {
	var n int32
	for _, c := range b.Conds {
		if st, ok := stageOf(c, look); ok {
			n += st.PainAdd
		}
	}
	return n
}

// condCap applies the conditions' current stage mods for capacity c to v,
// never below zero. A body with no conditions keeps v as it is.
func (b *Body) condCap(c Capacity, v int32, look condLookup) int32 {
	if len(b.Conds) == 0 {
		return v
	}
	for _, x := range b.Conds {
		st, ok := stageOf(x, look)
		if !ok {
			continue
		}
		for _, m := range st.Mods {
			if m.Cap != c {
				continue
			}
			v += m.Off
			if m.HasMax && v > m.Max {
				v = m.Max
			}
		}
	}
	if v < 0 {
		return 0
	}
	return v
}
