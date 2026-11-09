package cadence

// The periods the calendar names, in ticks (clock.TicksPerHour and
// TicksPerDay; the tests hold them equal).
const (
	hour = 2500
	day  = 24 * hour
)

// The per-entity offsets: each spread site visits id on the ticks where
// (now+Off+id)%every == 0. Distinct offsets keep two sites from visiting
// the same entity on the same tick.
const (
	OffEgg    = 83  // fertile eggs incubate (animal_life.go)
	OffTrough = 167 // troughs take in feed (trough_feed.go)
	OffPen    = 41  // unpenned animals, and once a day their leaving roll (pens.go)
)

// The whole-map passes' offsets: each runs on the ticks where
// (now+Off)%every == 0, so a pass whose period is an hour runs that many
// ticks after the hour (day minus the slot keeps the same slot for its
// daily work, at that many ticks after midnight). The climate block
// keeps the hour's multiples of 250; these slots fall between them. Rot
// and deterioration, and the pots, stay whole passes: each stack's or
// pot's change moves the item or building views, which then rebuild once
// a period, not on every tick.
const (
	OffRot       = 60        // craft.Condition.Rot, every climate interval at 190 past it
	OffWear      = 1190      // craft.Condition.Weather, hourly at 1310 past the hour
	OffPot       = 625       // furnish.Pots, hourly at 1875 past the hour
	OffMedical   = day - 125 // phaseMedical, hourly; the aging rolls at 00:00+125
	OffStory     = day - 375 // the setup story (losses, storyHour), hourly; the day rollover at 00:00+375
	OffCensus    = day - 625 // game/maps' temporary-map census, hourly
	OffSocialDay = day - 875 // the social day at 00:00+875
)

// Cost is how much a site's tick weighs on the frame budget.
type Cost uint8

// Light sites may share a tick; no two Heavy whole-map passes do
// (TestCalendarSlots).
const (
	Light Cost = iota
	Heavy
)

// Site is one periodic piece of work: its period, its offset, whether it
// is spread per entity (Each, Due with the id) or one whole-map pass (Due
// with id 0), and its weight.
type Site struct {
	Name      string
	Every     uint64
	Offset    uint64
	PerEntity bool
	Cost      Cost
}

// Calendar documents the periodic sites and their slots. Daily work that
// runs inside an hourly pass shares that pass's offset (and its slot); it
// is Light because it rides on the hourly pass's tick. The hand-staggered
// per-pawn checks (social, mind, drugs, escape, combat, skills) keep their
// own staggers and are not listed.
func Calendar() []Site {
	return []Site{
		{"climate", 250, 0, false, Heavy}, // phaseClimate: roofs, fuel, pumps, exposure, drying
		{"heat", 120, 0, false, Light},    // room temperatures (temperature.EqualiseInterval)
		{"rot", 250, OffRot, false, Heavy},
		{"wear", hour, OffWear, false, Heavy},
		{"egg", 250, OffEgg, true, Light},
		{"trough", 250, OffTrough, true, Light},
		{"pot", hour, OffPot, false, Light},
		{"pen", 250, OffPen, true, Light},
		{"pen-day", day, OffPen, true, Light},
		{"medical", hour, OffMedical, false, Heavy},
		{"aging", day, OffMedical, false, Light}, // inside medical's pass
		{"story", hour, OffStory, false, Heavy},
		{"rollover", day, OffStory, false, Light}, // inside story's pass
		{"census", hour, OffCensus, false, Heavy},
		{"social-day", day, OffSocialDay, false, Heavy},
	}
}
