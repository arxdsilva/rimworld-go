package research

import (
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/building/construction"
	proj "rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// Research speed and bench factor numbers (wiki: Research speed,
// Research, Simple research bench, Hi-tech research bench, Multi-analyzer).
// Fixed point: permille unless noted; points in billionths (nano).
const (
	// PointsPerTick is research points a tick at research speed 1, bench
	// factor 1 and difficulty 1, in billionths (wiki: 0.00825).
	PointsPerTick = 8_250_000
	nano          = 1_000_000_000

	intellectualBase  = 80  // wiki: 8 % at Intellectual 0
	intellectualLevel = 115 // wiki: +11.5 % a level
	minSpeed          = 100 // wiki: research speed is at least 10 %
	capacityCap       = 1100
	// capacityWeight: manipulation and sight count at 50 % importance (wiki).
	capacityWeight = 500

	simpleFactor   = 750  // wiki: simple research bench x0.75
	hiTechFactor   = 1000 // wiki: hi-tech research bench x1.0
	analyzerBonus  = 100  // wiki: a linked, powered multi-analyzer +0.10
	notLabFactor   = 800  // wiki: not in a laboratory x0.8
	outdoorClean   = 750  // wiki: outdoors, cleanliness counts as x0.75
	outdoorFactor  = 900  // original tuning (the wiki's outdoors x0.9 is [MEM], unconfirmed)
	badTempFactor  = 700  // wiki: below 10 C or above 35 C x0.7
	dirtyClean     = -500 // wiki: very dirty (cleanliness -5) x0.75
	dirtyFactor    = 750
	sterileClean   = 60 // wiki: sterile (cleanliness 0.6) x1.09
	sterileFactor  = 1090
	coldEnter      = 100 // wiki: below 10 C (tenths) the bench is too cold
	hotEnter       = 350 // wiki: above 35 C too hot
	tempHysteresis = 20  // original tuning: it leaves the bad band 2 C inside it
	// lab scores (original tuning after the wiki's room role scores: a
	// research bench outweighs a bill bench a little; any bed makes a
	// bedroom).
	labScore, shopScore = 30, 27
)

// Speed is a researcher's research speed, permille (wiki: Research speed):
// the Intellectual factor (8 % + 11.5 % a level) x global work speed x
// manipulation and sight at 50 % importance (each capped at 110 %) x the
// trait factor, at least 10 %.
func Speed(p Pawn) int {
	s := int64(intellectualBase + intellectualLevel*int(p.Intellectual))
	s = s * int64(p.GlobalWork) / 1000
	s = s * int64(capacityFactor(p.Manipulation)) / 1000
	s = s * int64(capacityFactor(p.Sight)) / 1000
	s = s * int64(p.TraitPct) / 100
	return max(int(s), minSpeed)
}

// capacityFactor is a capacity's factor at 50 % importance, permille.
func capacityFactor(c int) int {
	return 1000 + (min(c, capacityCap)-1000)*capacityWeight/1000
}

// PointsNano is the research points a tick, in billionths, at research
// speed s and bench factor b (permille) and difficulty d (percent).
func PointsNano(s, b, d int) int64 {
	return int64(PointsPerTick) * int64(s) * int64(b) * int64(d) / (1000 * 1000 * 100)
}

// Research bench kinds: the hi-tech one is the electric research bench.
func isBench(k construction.Kind) bool {
	return construction.RecipeBench(k) == construction.ResearchBench
}
func hiTech(k construction.Kind) bool { return k == construction.ElectricResearchBench }

// benchCache is the research benches and each hi-tech bench's analyzer,
// per structure version; never saved.
type benchCache struct {
	ver     uint64
	ok      bool
	benches []entity.EntityID
	link    map[entity.EntityID]entity.EntityID
}

// cache rebuilds the bench list and links when the buildings changed. A
// hi-tech bench links the first multi-analyzer in building order within
// the facility reach of the tool cabinet (craft.FacilityReach, original
// tuning: the wiki gives no range); one analyzer serves many benches.
func (l *Lab) cache() *benchCache {
	c := &l.benches
	if ver := l.h.StructVer(); c.ok && c.ver == ver {
		return c
	}
	c.ver, c.ok, c.benches, c.link = l.h.StructVer(), true, c.benches[:0], map[entity.EntityID]entity.EntityID{}
	var analyzers []construction.Building
	var ids []entity.EntityID
	for _, id := range l.h.BuildingIDs() {
		b, ok := l.h.Building(id)
		switch {
		case !ok:
		case isBench(b.Kind):
			c.benches = append(c.benches, id)
		case b.Kind == construction.MultiAnalyzer:
			analyzers, ids = append(analyzers, b), append(ids, id)
		}
	}
	for _, id := range c.benches {
		b, _ := l.h.Building(id)
		if !hiTech(b.Kind) {
			continue
		}
		for i, a := range analyzers {
			if max(mathx.Abs(a.X-b.X), mathx.Abs(a.Y-b.Y)) <= craft.FacilityReach {
				c.link[id] = ids[i]
				break
			}
		}
	}
	return c
}

// Analyzer is the multi-analyzer hi-tech bench links (0: none).
func (l *Lab) Analyzer(bench entity.EntityID) entity.EntityID { return l.cache().link[bench] }

// analyzed reports bench linked to a powered analyzer.
func (l *Lab) analyzed(bench entity.EntityID) bool {
	a := l.Analyzer(bench)
	return a != 0 && l.h.Powered(a)
}

// Factor is bench's research factor now, permille (0: not a research
// bench): the bench kind (simple 0.75, hi-tech 1.0, +0.10 with a powered
// analyzer), times not a laboratory 0.8, cleanliness (very dirty 0.75 ...
// sterile 1.09; outdoors 0.75 and 0.9), the room's reading bonus (up to
// 1.2) and a bad temperature 0.7.
func (l *Lab) Factor(bench entity.EntityID) int {
	b, ok := l.h.Building(bench)
	if !ok || !isBench(b.Kind) {
		return 0
	}
	f, den := int64(simpleFactor), int64(1)
	mul := func(x int) { f, den = f*int64(x), den*1000 }
	if hiTech(b.Kind) {
		f = hiTechFactor
		if l.analyzed(bench) {
			f += analyzerBonus
		}
	}
	r, indoors := l.h.Room(b.X, b.Y)
	if !indoors {
		mul(outdoorClean)
		mul(outdoorFactor)
		mul(notLabFactor)
	} else {
		mul(cleanFactor(l.h.Cleanliness(r)))
		mul(1000 + l.h.ReadingBonus(r)) // AX-7 (wiki: Reading bonus): the room's shelved books
		if !l.lab(r) {
			mul(notLabFactor)
		}
	}
	if l.Cold[bench] {
		mul(badTempFactor)
	}
	return int(f / den)
}

// cleanFactor is the cleanliness factor (hundredths in, permille out),
// straight between the wiki's points: -5 gives 0.75, 0 gives 1, 0.6 1.09.
func cleanFactor(c int) int {
	switch {
	case c <= dirtyClean:
		return dirtyFactor
	case c < 0:
		return 1000 - (1000-dirtyFactor)*c/dirtyClean
	case c >= sterileClean:
		return sterileFactor
	}
	return 1000 + (sterileFactor-1000)*c/sterileClean
}

// lab reports indoor room r a laboratory: no bed, and its research
// benches outscore its bill benches (original tuning, after the wiki's
// role scores).
func (l *Lab) lab(r int) bool {
	beds, benches := l.h.Contents(r)
	if beds > 0 {
		return false
	}
	n := 0
	for _, id := range l.cache().benches {
		if b, ok := l.h.Building(id); ok {
			if br, in := l.h.Room(b.X, b.Y); in && br == r {
				n++
			}
		}
	}
	return n > 0 && labScore*n >= shopScore*benches
}

// feel moves bench's bad-temperature flag with hysteresis: it turns on
// below 10 C or above 35 C and off only 2 C back inside the band, so a
// bench at the edge does not flip every tick.
func (l *Lab) feel(bench entity.EntityID, b construction.Building) {
	t := l.h.TemperatureAt(b.X, b.Y)
	switch bad := l.Cold[bench]; {
	case !bad && (t < coldEnter || t > hotEnter):
		if l.Cold == nil {
			l.Cold = map[entity.EntityID]bool{}
		}
		l.Cold[bench] = true
	case bad && t >= coldEnter+tempHysteresis && t <= hotEnter-tempHysteresis:
		delete(l.Cold, bench)
	}
}

// CanWork reports bench can work project p: a hi-tech project never at a
// simple bench, an analyzer project only with a powered linked analyzer
// (wiki: Research). It does not look at the bench's own power.
func (l *Lab) CanWork(bench entity.EntityID, b construction.Building, p proj.Project) bool {
	switch proj.DefOf(p).Bench {
	case proj.HiTechBench:
		return hiTech(b.Kind)
	case proj.HiTechAnalyzer:
		return hiTech(b.Kind) && l.analyzed(bench)
	}
	return true
}
