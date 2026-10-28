package options

import (
	"fmt"

	"rimworld-go/internal/platform/sysinfo"
)

// PLAY-019 RAM warnings. The game warns about small machines and big maps;
// it never blocks a size or a start. The model is two numbers fitted to the
// peak memory footprint of the real game after the PLAY-019 art diet
// (/usr/bin/time -l with -fpsprobe on an Apple-silicon Mac). The footprint
// counts the GPU's share of unified memory, which max RSS (about 1.1 GiB
// on every size) leaves out, so it is the safe figure:
//
//	48x40 map      2242 MiB
//	250x250 map    2141 MiB
//	500x500 grown  3422 MiB (a grown colony: the late game's worst case)
//
// The line runs through the larger small-map figure at 250 (2242 MiB) and
// the grown 500: (3422 - 2242) MiB over (500^2 - 250^2) tiles is about
// 6600 bytes per tile (terrain, chunk bakes, things and their art), and
// the base is what is left at no tiles, about 1850 MiB. Re-measure both
// when the art or the map's memory changes.
const (
	// RamBase is the game's memory without a map, in bytes (1850 MiB).
	RamBase uint64 = 1850 << 20
	// RamPerTile is the memory a map tile adds, in bytes.
	RamPerTile uint64 = 6600
)

// The thresholds the warnings use.
const (
	// ramMapSharePct: a map size warns when its estimate is over this
	// share of the computer's memory.
	ramMapSharePct = 70
	// LargeMapSide is where the fallback for unknown hardware starts
	// noting a map (it mirrors game.MapSizes' Large flag; a test keeps
	// them equal).
	LargeMapSide = 350
	// ramHeadroomGiB is the memory the system and other programs keep.
	ramHeadroomGiB = 2
	// DefaultMapSide is the map the minimum is figured for (it mirrors the
	// wizard's default; a root test keeps them equal).
	DefaultMapSide = 250
)

// largeMapNote is what a 350+ map says when the hardware is unknown.
const largeMapNote = "Large map: needs a capable computer"

// EstimatedRAM is the memory the game needs for a side x side map, bytes.
func EstimatedRAM(side int) uint64 {
	if side < 0 {
		side = 0
	}
	return RamBase + uint64(side)*uint64(side)*RamPerTile
}

// MinimumRAM is the least memory the game runs smoothly on: the default
// map's estimate x 1.5 plus 2 GiB, rounded up to a whole GiB.
func MinimumRAM() uint64 {
	need := EstimatedRAM(DefaultMapSide)*3/2 + ramHeadroomGiB*sysinfo.GiB
	return (need + sysinfo.GiB - 1) / sysinfo.GiB * sysinfo.GiB
}

// GBCeil is b in whole GB, rounded up ("needs about").
func GBCeil(b uint64) int { return int((b + sysinfo.GiB - 1) / sysinfo.GiB) }

// GBRound is b in whole GB, to the nearest (a "16 GB" PC reports 15.9 GiB).
func GBRound(b uint64) int { return int((b + sysinfo.GiB/2) / sysinfo.GiB) }

// LowRAMText is the main-menu notice for a computer with total bytes.
func LowRAMText(total uint64) string {
	need := GBCeil(MinimumRAM())
	has := min(GBRound(total), need-1) // never "has 6 GB; needs about 6 GB"
	return fmt.Sprintf("This computer has %d GB of memory; the game needs about %d GB to run smoothly. It will use low graphics settings.", has, need)
}

// MapSizeNote is the warning for a side x side map on a computer with total
// bytes of memory, "" when there is none:
//   - hardware known: the estimate is over 70% of it, "Needs about N GB;
//     this computer has M GB.";
//   - hardware unknown (total 0): only 350 and up, a gentle large-map note.
//
// The size picker and the landing site panel both ask this.
func MapSizeNote(side int, total uint64) string {
	if total == 0 {
		if side >= LargeMapSide {
			return largeMapNote
		}
		return ""
	}
	est := EstimatedRAM(side)
	if est*100 <= total*ramMapSharePct {
		return ""
	}
	return fmt.Sprintf("Needs about %d GB; this computer has %d GB.", GBCeil(est), GBRound(total))
}
