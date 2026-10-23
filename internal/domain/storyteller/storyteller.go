// Package storyteller paces the colony's story: when threats come and how
// big, with three original personalities and a hidden legacy profile that
// reproduces the pre-NG-1 constants (NG-1 §3.2, §4.5).
package storyteller

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/pkg/randx"
)

// Mode is a pacing model.
type Mode uint8

const (
	Legacy Mode = iota
	Cycle
	Calm
	Dice
)

func (m Mode) String() string {
	switch m {
	case Cycle:
		return "cycle"
	case Calm:
		return "calm"
	case Dice:
		return "dice"
	}
	return "legacy"
}

// Category is a threat category the scheduler may fire. Plan only says
// "a big threat" (Raid); the caller picks the concrete threat with its own
// rules (pickThreat) filtered by difficulty and the available factions.
type Category uint8

const (
	None Category = iota
	Raid
	Pack
	Manhunters
	Hive
	Mad
)

// Profile is a storyteller.
type Profile struct {
	Key, Name, Title, Bio, Portrait string
	Mode                            Mode
	FirstThreatDay                  int
	GapMin, GapMax                  int
	CycleOn, CycleOff               int
	DiceP0, DiceStep                int // permille
	PointsMin, PointsMax            int // percent
	RampPerCycle, RampCap           int // percent
	PositiveBias                    int // percent of nature events drawn from the helpful set; 0 = existing weights
	LossEaseDays, LossEasePct       int
	Hidden                          bool
}

// State is the scheduler's memory (saved). The zero value is a fresh
// colony. LastLossDay is the day of the latest colonist death (0: none).
type State struct {
	LastThreatDay, CycleDay, QuietDays, LastLossDay, Cycles int
	Pressure                                                bool
	// Started is set by the first Plan (the plan made at colony start).
	Started bool
	// Eased is set when OnLoss has already eased the pending threat.
	Eased bool
}

// Decision is the next threat.
type Decision struct {
	AtTick    uint64
	Category  Category // the caller picks the concrete threat by its rules
	PointsPct int
	// Second is a second threat category on the same day (Dice mode, 5%);
	// None when there is only one.
	Second Category
}

// Pacing numbers shared by the modes.
const (
	ticksPerDay = clock.TicksPerDay
	diceHour    = 7            // the Dice roll happens at 07:00
	diceMaxDays = 400          // safety bound for the daily roll loop
	diceDouble  = 5            // percent chance of a second category
	salt        = 0x73746f7279 // "story": Spark's rng salt
)

var profiles = []Profile{
	{
		Key: "steady", Name: "Orla Vane", Title: "the Steady Hand", Portrait: "ui_portrait/steady",
		Bio: "Orla spent thirty years in a dispatch booth, sending convoys across country that did not care whether they arrived. " +
			"She came away believing that trouble is a tide, not an accident, and that a crew that knows when the water will rise can build a wall in time.\n\n" +
			"She keeps a rhythm: a stretch of hard weather, then a stretch to mend and breathe. She will not apologise for the hard stretch, and she never skips the rest.\n\n" +
			"Her headset still hangs round her neck. Out here it picks up nothing but wind, and she seems to like it that way.",
		Mode: Cycle, FirstThreatDay: 6, GapMin: 3, GapMax: 5, CycleOn: 8, CycleOff: 5,
		PointsMin: 100, PointsMax: 100, RampPerCycle: 4, RampCap: 40, PositiveBias: 20,
	},
	{
		Key: "hearth", Name: "Bram Holloway", Title: "the Hearthkeeper", Portrait: "ui_portrait/hearth",
		Bio: "Bram kept an inn at a crossroads for forty winters and never once turned a stranger away without a bowl of something hot. " +
			"He has seen what hurried people do, and he prefers to let you finish your soup.\n\n" +
			"He is slow to send trouble, and slower to send it twice. When someone in your colony is lost he goes quiet for a while, the way an innkeeper does when a regular stops coming in.\n\n" +
			"He thinks a settlement is mostly kindness with a roof on it. He is not wrong, but he is not a fool either.",
		Mode: Calm, FirstThreatDay: 10, GapMin: 9, GapMax: 15,
		PointsMin: 100, PointsMax: 100, PositiveBias: 45, LossEaseDays: 6, LossEasePct: 70,
	},
	{
		Key: "gambler", Name: "Kestrel Quay", Title: "the Gambler", Portrait: "ui_portrait/gambler",
		Bio: "Kestrel made a living at card tables on three continents and left each of them a little faster than she arrived. " +
			"She does not remember the last hand, only the next one, and she will bet that you do not either.\n\n" +
			"With her there is no schedule. Some weeks nothing happens and you start to trust the quiet; then two bad things turn up at breakfast. " +
			"She will tell you the odds were fair. She will not tell you which side of them you were on.\n\n" +
			"Play with her if you like surprises. Everyone else should keep a spare bed made up.",
		Mode: Dice, FirstThreatDay: 4, GapMin: 1, DiceP0: 125, DiceStep: 25,
		PointsMin: 60, PointsMax: 150, PositiveBias: 30,
	},
	{
		Key: "legacy", Name: "The old rhythm", Title: "", Portrait: "", Hidden: true,
		Bio:  "The pre-NG-1 pacing: a small raid on day 5, then one every 5 to 9 days.",
		Mode: Legacy, FirstThreatDay: 5, GapMin: 5, GapMax: 9, PointsMin: 100, PointsMax: 100,
	},
}

// Profiles lists the visible storytellers; ByKey finds any (incl. legacy).
func Profiles() []Profile {
	var out []Profile
	for _, p := range profiles {
		if !p.Hidden {
			out = append(out, p)
		}
	}
	return out
}

// ByKey finds a storyteller by key, hidden ones included.
func ByKey(key string) (Profile, bool) {
	for _, p := range profiles {
		if p.Key == key {
			return p, true
		}
	}
	return Profile{}, false
}

// Phase says whether day is in the pressure part of a Cycle storyteller's
// rhythm and which cycle (from 0) it belongs to. Every other mode is
// always in pressure, cycle 0.
func Phase(p Profile, day int) (pressure bool, cycle int) {
	if p.Mode != Cycle || p.CycleOn+p.CycleOff <= 0 {
		return true, 0
	}
	length := p.CycleOn + p.CycleOff
	return day%length < p.CycleOn, day / length
}

// Ramp is the percent added to threat points on day by a Cycle
// storyteller's per-cycle ramp (0 for the others).
func Ramp(p Profile, day int) int {
	if p.Mode != Cycle {
		return 0
	}
	_, cycle := Phase(p, day)
	return min(p.RampCap, p.RampPerCycle*cycle)
}

// span draws a value in [lo, hi] (lo when hi <= lo, without a draw).
func span(rng *randx.Rand, lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + rng.IntN(hi-lo+1)
}

// pointsPct draws the decision's points percent.
func pointsPct(p Profile, rng *randx.Rand, day int) int {
	pct := span(rng, p.PointsMin, p.PointsMax)
	if pct == 0 {
		pct = 100
	}
	return pct + Ramp(p, day)
}

// Plan decides the next threat after the one that just fired at tick now
// (or the first one, on the first call with a fresh State). It updates st.
//
// Random draws are fixed in order so a legacy colony reproduces the
// pre-NG-1 scheduler bit for bit: the start plan draws IntN(ticksPerDay);
// each later plan draws IntN(gap spread) then IntN(ticksPerDay).
func Plan(p Profile, st *State, now uint64, rng *randx.Rand) Decision {
	day := int(now / ticksPerDay)
	start := !st.Started
	st.Started = true
	if !start {
		st.LastThreatDay = day
		st.QuietDays = 0
		st.Eased = false
	}
	var d Decision
	if p.Mode == Dice {
		d = planDice(p, st, day, start, rng)
	} else {
		d = planGapped(p, st, now, start, rng)
	}
	atDay := int(d.AtTick / ticksPerDay)
	pressure, cycle := Phase(p, atDay)
	st.Pressure, st.Cycles = pressure, cycle
	if p.CycleOn+p.CycleOff > 0 && p.Mode == Cycle {
		st.CycleDay = atDay % (p.CycleOn + p.CycleOff)
	}
	return d
}

// planGapped is Legacy, Cycle and Calm: a first threat on a fixed day,
// then a random gap in whole days plus a random time of day.
func planGapped(p Profile, st *State, now uint64, start bool, rng *randx.Rand) Decision {
	var at uint64
	if start {
		at = uint64(p.FirstThreatDay*ticksPerDay + rng.IntN(ticksPerDay))
	} else {
		gap := span(rng, p.GapMin, p.GapMax)
		at = now + uint64(gap*ticksPerDay+rng.IntN(ticksPerDay))
	}
	if p.Mode == Cycle {
		// A threat that would land in the rest phase waits for the next
		// pressure phase and arrives within its first GapMin days.
		if pressure, cycle := Phase(p, int(at/ticksPerDay)); !pressure {
			next := (cycle + 1) * (p.CycleOn + p.CycleOff)
			at = uint64(next*ticksPerDay + rng.IntN(max(p.GapMin, 1)*ticksPerDay))
		}
	}
	return Decision{AtTick: at, Category: Raid, PointsPct: pointsPct(p, rng, int(at/ticksPerDay))}
}

// planDice rolls day by day from the first eligible day until a threat
// comes: probability P0 + Step per quiet day (per mille), at 07:00.
func planDice(p Profile, st *State, day int, start bool, rng *randx.Rand) Decision {
	d0 := p.FirstThreatDay
	if !start {
		d0 = day + max(p.GapMin, 1)
	}
	quiet := 0
	for i := 0; i < diceMaxDays; i++ {
		prob := min(p.DiceP0+p.DiceStep*quiet, 1000)
		if rng.IntN(1000) < prob {
			break
		}
		quiet++
	}
	st.QuietDays = quiet
	fireDay := d0 + quiet
	out := Decision{
		AtTick:   uint64(fireDay*ticksPerDay + diceHour*clock.TicksPerHour),
		Category: Raid,
	}
	if rng.IntN(100) < diceDouble {
		out.Second = Raid
	}
	out.PointsPct = pointsPct(p, rng, fireDay)
	return out
}

// OnLoss eases the pending threat d after a colonist died on nowDay: the
// profile's LossEaseDays are added to its time and its points are scaled by
// LossEasePct. It acts once per pending threat (st.Eased) and reports
// whether it changed d.
func OnLoss(p Profile, st *State, d *Decision, nowDay int) bool {
	st.LastLossDay = nowDay
	if p.LossEaseDays <= 0 || st.Eased || d == nil || d.Category == None {
		return false
	}
	st.Eased = true
	d.AtTick += uint64(p.LossEaseDays * ticksPerDay)
	pct := p.LossEasePct
	if pct <= 0 {
		pct = 100
	}
	d.PointsPct = d.PointsPct * pct / 100
	return true
}

// Spark is an example pacing for the UI: threat strength (percent of the
// baseline, summed when two threats share a day) for each of days days.
func Spark(p Profile, days int, seed uint64) []int {
	if days <= 0 {
		return nil
	}
	rng := randx.New(seed ^ salt)
	var st State
	out := make([]int, days)
	d := Plan(p, &st, 0, rng)
	for day := int(d.AtTick / ticksPerDay); day < days; day = int(d.AtTick / ticksPerDay) {
		out[day] += d.PointsPct
		if d.Second != None {
			out[day] += d.PointsPct
		}
		d = Plan(p, &st, d.AtTick, rng)
	}
	return out
}
