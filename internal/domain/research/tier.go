package research

import "rimworld-go/internal/domain/faction"

// TechLevel is a project's (or a faction's) technology level (RSCH-0
// contract; wiki: Research). faction.Tech has no Medieval step and is
// saved by value, so the research tree keeps its own ordered scale and
// maps a faction's level onto it (LevelOf).
type TechLevel uint8

const (
	Neolithic TechLevel = iota
	Medieval
	Industrial
	Spacer
	Ultra
	NumTechLevels
)

var techNames = [NumTechLevels]string{"Neolithic", "Medieval", "Industrial", "Spacer", "Ultra"}

// String is the level's display name ("Neolithic" ... "Ultra").
func (t TechLevel) String() string {
	if t >= NumTechLevels {
		return ""
	}
	return techNames[t]
}

// LevelOf is a faction's technology level on the research scale.
func LevelOf(t faction.Tech) TechLevel {
	switch t {
	case faction.Neolithic:
		return Neolithic
	case faction.Spacer:
		return Spacer
	}
	return Industrial
}

// BenchTier is the research bench a project needs (RSCH-0 contract; wiki:
// Research, the S / H / H+MA column). The zero value, any research bench,
// is what every project needed before RSCH-1.
type BenchTier uint8

const (
	// SimpleBench: any research bench will do.
	SimpleBench BenchTier = iota
	// HiTechBench: a hi-tech research bench (construction kind
	// ElectricResearchBench), never a simple one.
	HiTechBench
	// HiTechAnalyzer: a hi-tech research bench linked to a Multi-analyzer.
	HiTechAnalyzer
	NumBenchTiers
)

var benchNames = [NumBenchTiers]string{"research bench", "hi-tech research bench", "hi-tech research bench + multi-analyzer"}

// String is what the tier asks for, in words.
func (b BenchTier) String() string {
	if b >= NumBenchTiers {
		return ""
	}
	return benchNames[b]
}

// CostPct is what a project of level project costs a faction of level
// faction, in percent of its base cost (wiki: Research, tech level cost
// multiplier: a tribe pays 150 % for Medieval projects and 200 % for
// Industrial and later ones; a faction at or above the level pays 100 %).
// The formula is original tuning that reproduces those published values:
// 100 + 50 per level the project (capped at Industrial) is above the
// faction.
func CostPct(project, faction TechLevel) int {
	project = min(project, Industrial)
	if project <= faction {
		return 100
	}
	return 100 + 50*int(project-faction)
}
