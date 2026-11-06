package armor

import (
	"strings"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
)

// BaseSpeed is a human's move speed in hundredths of a cell per second
// (wiki: Human, 4.6): the base the stat sheet's MoveSpeed permille is of.
const BaseSpeed = 460

// MoveOffset is what wearing garment k adds to move speed, in hundredths
// of a cell per second (negative slows). wiki: Armor (flak vest -0.12,
// flak pants -0.12, marine armor -0.25, plate armor -0.8). The wiki's
// cataphract armor (-0.5) has no counterpart among our kinds; the flak
// jacket, helmets and clothes cost nothing. Penalties add.
func MoveOffset(k item.Kind) int {
	switch k {
	case item.FlakVest, item.FlakPants:
		return -12
	case item.MarineArmor:
		return -25
	case item.PlateArmor:
		return -80
	}
	return 0
}

// SpeedPermille is c hundredths of a cell per second as permille of
// BaseSpeed, rounded half away from zero (-12 is -26, -80 is -174).
func SpeedPermille(c int) int32 {
	n := c * 1000
	if n < 0 {
		return int32((n - BaseSpeed/2) / BaseSpeed)
	}
	return int32((n + BaseSpeed/2) / BaseSpeed)
}

// SpeedPct is c hundredths of a cell per second as a whole percent of
// BaseSpeed, rounded half away from zero (-12 is -3, -80 is -17).
func SpeedPct(c int) int {
	if c < 0 {
		return -SpeedPct(-c)
	}
	return (c*100 + BaseSpeed/2) / BaseSpeed
}

// Overall is the wearer's overall armor (the Gear tab's summary, GEAR-002):
// for every part of the humanlike body, the ratings of every worn layer
// covering it added up (Set.Add, capped at MaxRating), weighted by the
// part's share of hits (body.Def.Coverage, which sums to 1000). Our own
// formula: the wiki publishes none. Missing parts still count, so the
// figure describes the clothes, not the wounds.
func Overall(worn []Layer) Set {
	var sh, bl, he int
	for p := body.Part(0); p < body.NumParts; p++ {
		var s Set
		for _, l := range worn {
			if l.Covers&partGroups[p] != 0 {
				s = s.Add(l.Set)
			}
		}
		c := body.DefOf(p).Coverage
		sh, bl, he = sh+int(s.Sharp)*c, bl+int(s.Blunt)*c, he+int(s.Heat)*c
	}
	return Set{Permille(sh / 1000), Permille(bl / 1000), Permille(he / 1000)}
}

// regionNames names the coverage groups in the order Names lists them.
var regionNames = [...]struct {
	r    Region
	name string
}{
	{CoverFullHead, "head"}, {CoverUpperHead, "upper head"}, {CoverEyes, "eyes"}, {CoverTorso, "torso"},
	{CoverNeck, "neck"}, {CoverShoulders, "shoulders"}, {CoverArms, "arms"}, {CoverWaist, "waist"}, {CoverLegs, "legs"},
}

// Names lists the groups r covers, as the Gear tab shows them ("torso,
// neck, shoulders, arms"; "" for none). A garment over the whole head is
// just "head".
func (r Region) Names() string {
	if r&CoverFullHead != 0 {
		r &^= CoverUpperHead | CoverEyes
	}
	var b strings.Builder
	for _, n := range regionNames {
		if r&n.r == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		b.WriteString(n.name)
	}
	return b.String()
}
