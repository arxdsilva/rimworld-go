package animal

// Threat cost and pack sizes (EAI-005). Original tuning, from the Def
// alone (CLEAN_ROOM: no outside numbers): what an animal is worth to the
// storyteller is how long it lasts (MaxHP) plus how hard it hits (its
// blows' damage over a 600-tick stretch, counted twice), three quarters
// of that, scaled by its pace (a step of 16 ticks counts as is; faster
// costs more, slower less). A timber wolf costs 93, a squirrel 27, an
// elephant 192.
const (
	// threatStretch is the ticks a blow rate is counted over.
	threatStretch = 600
	// threatPace is the step ticks a pace counts as 1 against.
	threatPace = 16
	// MinThreatCost keeps the cheapest animal worth something.
	MinThreatCost = 10
	// MaxPack caps a pack sent for any points.
	MaxPack = 24
)

// ThreatCost is the threat points one grown animal of def is worth.
func ThreatCost(def Def) int {
	hits := 0
	if def.Cooldown > 0 {
		hits = def.Damage * threatStretch / def.Cooldown
	}
	c := (def.MaxHP + 2*hits) * 3 / 4
	if def.StepTicks > 0 {
		c = c * 2 * threatPace / (threatPace + def.StepTicks)
	}
	return max(c, MinThreatCost)
}

// PackSize is how many animals of s a pack worth points holds: as many as
// the points buy, at least one, at most MaxPack.
func PackSize(points int, s Species) int {
	return min(max(points/ThreatCost(DefOf(s)), 1), MaxPack)
}
