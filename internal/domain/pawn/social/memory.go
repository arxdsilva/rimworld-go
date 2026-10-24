package social

// MemKind is an opinion memory (append only, saved).
type MemKind uint8

const (
	MemChitchat MemKind = iota
	MemDeepTalk
	MemSlighted
	MemInsulted
	MemKindWords
	MemRebuffedMe
	MemFailedRomance
	MemCheatedOnMe
	MemRejectedMyProposal
	MemIRejectedProposal
	MemBrokeUpWithMe
	MemDivorcedMe
	MemAngeringFight
	MemCatharticFight
	MemHarmedMe
	MemKilledMyParentChild
	MemKilledMySibling
	MemKilledMySpouse
	MemKilledMyLover
	MemKilledMyKin
	MemRecruitedMe
	MemRescuedMe
	MemRescuedMeStranger
	MemBotchedSurgery
	NumMemKinds
)

// OpinionMemory is one remembered event shaping an opinion of Subject.
type OpinionMemory struct {
	Kind    MemKind
	Subject ID
	Expires uint64
}

// Def is a memory kind's static definition.
type Def struct {
	Label       string
	Value       int32  // centi-points per copy
	Duration    uint64 // ticks
	StackLimit  int
	StackFactor int32 // permille applied per further copy
}

const permanent = 1000 * Day

var memDefs = [NumMemKinds]Def{
	MemChitchat:            {"chitchat", memChitchat, 6 * Day, 20, stackFactorSocial},
	MemDeepTalk:            {"deep talk", memDeepTalk, 20 * Day, 5, stackFactorSocial},
	MemSlighted:            {"slighted me", memSlighted, 20 * Day, 5, stackFactorSocial},
	MemInsulted:            {"insulted me", memInsulted, 20 * Day, 5, stackFactorSocial},
	MemKindWords:           {"kind words", memKindWords, 20 * Day, 5, stackFactorSocial},
	MemRebuffedMe:          {"turned me down", memRebuffedMe, 3 * Day, 1, stackFactorDefault},
	MemFailedRomance:       {"failed to charm me", memFailedRomance, 3 * Day, 1, stackFactorDefault},
	MemCheatedOnMe:         {"cheated on me", memCheatedOnMe, 25 * Day, 1, stackFactorDefault},
	MemRejectedMyProposal:  {"turned down my proposal", memRejectedMyProp, 25 * Day, 1, stackFactorDefault},
	MemIRejectedProposal:   {"I turned down their proposal", memIRejectedProp, 25 * Day, 1, stackFactorDefault},
	MemBrokeUpWithMe:       {"left me", memBrokeUpWithMe, 25 * Day, 1, stackFactorDefault},
	MemDivorcedMe:          {"divorced me", memDivorcedMe, 30 * Day, 1, stackFactorDefault},
	MemAngeringFight:       {"angering fight", memAngeringFight, FightOutcomeDays * Day, 1, stackFactorDefault},
	MemCatharticFight:      {"cathartic fight", memCatharticFight, FightOutcomeDays * Day, 1, stackFactorDefault},
	MemHarmedMe:            {"hurt me", memHarmedMe, 10 * Day, 3, stackFactorSocial},
	MemKilledMyParentChild: {"killed my parent or child", memKilledParent, permanent, 1, stackFactorDefault},
	MemKilledMySibling:     {"killed my sibling", memKilledSibling, permanent, 1, stackFactorDefault},
	MemKilledMySpouse:      {"killed my spouse", memKilledSpouse, permanent, 1, stackFactorDefault},
	MemKilledMyLover:       {"killed my lover", memKilledLover, permanent, 1, stackFactorDefault},
	MemKilledMyKin:         {"killed my kin", memKilledKin, permanent, 1, stackFactorDefault},
	MemRecruitedMe:         {"recruited me", memRecruitedMe, 120 * Day, 1, stackFactorDefault},
	MemRescuedMe:           {"rescued me", memRescuedMe, 30 * Day, 1, stackFactorDefault},
	MemRescuedMeStranger:   {"rescued me, a stranger", memRescuedStranger, 30 * Day, 1, stackFactorDefault},
	MemBotchedSurgery:      {"botched my surgery", memBotchedSurgery, 90 * Day, 1, stackFactorDefault},
}

// MemDef is k's definition (zero for an unknown kind).
func MemDef(k MemKind) Def {
	if k >= NumMemKinds {
		return Def{}
	}
	return memDefs[k]
}

// AddMemory remembers k about subject at tick now, stacking per its def:
// each further copy counts at StackFactor of the one before; at the limit
// the oldest copy is refreshed instead of adding another.
func AddMemory(ms []OpinionMemory, k MemKind, subject ID, now uint64) []OpinionMemory {
	d := MemDef(k)
	if d.Duration == 0 {
		return ms
	}
	n, oldest := 0, -1
	for i, m := range ms {
		if m.Kind != k || m.Subject != subject || m.Expires <= now {
			continue
		}
		n++
		if oldest < 0 || m.Expires < ms[oldest].Expires {
			oldest = i
		}
	}
	exp := now + d.Duration
	if n >= d.StackLimit && oldest >= 0 {
		ms[oldest].Expires = exp
		return ms
	}
	return append(ms, OpinionMemory{Kind: k, Subject: subject, Expires: exp})
}

// PruneMemories drops expired memories (in place, keeping order).
func PruneMemories(ms []OpinionMemory, now uint64) []OpinionMemory {
	out := ms[:0]
	for _, m := range ms {
		if m.Expires > now {
			out = append(out, m)
		}
	}
	return out
}

// HasMemory reports a live memory of kind k about subject.
func HasMemory(ms []OpinionMemory, k MemKind, subject ID, now uint64) bool {
	for _, m := range ms {
		if m.Kind == k && m.Subject == subject && m.Expires > now {
			return true
		}
	}
	return false
}

// MemoryTerms is the opinion contribution of each live memory kind about
// subject (stacked diminishing), in kind order. Chitchat is capped.
func MemoryTerms(ms []OpinionMemory, subject ID, now uint64) []Term {
	var out []Term
	for k := MemKind(0); k < NumMemKinds; k++ {
		d := memDefs[k]
		n := 0
		for _, m := range ms {
			if m.Kind == k && m.Subject == subject && m.Expires > now {
				n++
			}
		}
		if n == 0 {
			continue
		}
		var sum int64
		f := int64(1000)
		for i := 0; i < n; i++ {
			sum += int64(d.Value) * f / 1000
			f = f * int64(d.StackFactor) / 1000
		}
		if k == MemChitchat && sum > ChitchatCap {
			sum = ChitchatCap
		}
		out = append(out, Term{Label: d.Label, Value: int32(sum)})
	}
	return out
}

// Sum is the total memory opinion about subject.
func Sum(ms []OpinionMemory, subject ID, now uint64) int32 {
	var t int32
	for _, x := range MemoryTerms(ms, subject, now) {
		t += x.Value
	}
	return t
}
