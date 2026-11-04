package roster

import (
	"fmt"
	"strings"

	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/infection"
)

// The Health tab's pure text: part status lines and hover cards (the game
// package's healthView fills the rest of the tab).

// ConditionView is one condition row of the Health tab: severity,
// immunity and tend quality in percent, Due when it needs tending now,
// and the tooltip text. MED-02: TendPct is the last tend's quality (0:
// never tended), DueInHours the whole hours until it needs its next tend
// (0 when Due or it takes no tending), Losing whether it is on course to
// kill before immunity wins.
type ConditionView struct {
	Label                   string
	SevPct, ImmPct, TendPct int
	Due                     bool
	Tip                     string
	DueInHours              int
	Losing                  bool
}

// CapacityTip is the capacity's hover card: its name, then one line a
// factor (body.CapacityFactors) and the result.
func CapacityTip(b *body.Body, c body.Capacity) string {
	var sb strings.Builder
	sb.WriteString(CapacityLabels[c])
	for _, f := range b.CapacityFactors(c) {
		if f.Label == "=" {
			fmt.Fprintf(&sb, "\n= %d%%", f.Pct)
			continue
		}
		fmt.Fprintf(&sb, "\n%s %d%%", f.Label, f.Pct)
	}
	return sb.String()
}

// CapacityLabels name body.Capacity values for the Health tab.
var CapacityLabels = [body.NumCaps]string{
	body.Consciousness: "Consciousness", body.Moving: "Moving", body.Manipulation: "Manipulation",
	body.Sight: "Sight", body.Hearing: "Hearing", body.Talking: "Talking", body.Breathing: "Breathing",
	body.BloodPumping: "Blood pumping", body.BloodFiltration: "Blood filtration", body.Digestion: "Digestion", // MED-05
}

// Part status words of the Health tab (PartView.Status starts with one).
const (
	PartHealthy    = "healthy"
	PartMissing    = "missing"
	PartReplaced   = "replaced"   // under an artificial limb
	PartArtificial = "artificial" // "artificial: peg leg"
	PartWounded    = "wounded"    // "wounded: 2 wounds (tended), 1 scar"
	PartScarred    = "scarred"
)

// coveredByImplant reports a missing part p standing under an implant on
// one of its ancestors (a foot under a prosthetic leg).
func coveredByImplant(b *body.Body, p body.Part) bool {
	for q, i := body.DefOf(p).Parent, 0; i < int(body.NumParts); q, i = body.DefOf(q).Parent, i+1 {
		if b.Implant(q).IsImplant() {
			return true
		}
		if body.DefOf(q).Parent == q {
			return false
		}
	}
	return false
}

// PartStatus is the Health tab's status line for one part.
func PartStatus(b *body.Body, s body.PartStatus) string {
	switch {
	case s.Missing && s.Fitted:
		return PartArtificial + ": " + s.Implant.String()
	case s.Missing && coveredByImplant(b, s.Part):
		return PartReplaced
	case s.Missing:
		return PartMissing
	case s.Wounds > 0:
		t := fmt.Sprintf("%s: %d wound%s", PartWounded, s.Wounds, plural(s.Wounds))
		if s.Tended {
			t += " (tended)"
		}
		if s.Scars > 0 {
			t += fmt.Sprintf(", %d scar%s", s.Scars, plural(s.Scars))
		}
		return t
	case s.Scars > 0:
		return fmt.Sprintf("%s: %d scar%s", PartScarred, s.Scars, plural(s.Scars))
	}
	return PartHealthy
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ConditionTip is a condition row's hover card: severity and immunity,
// the tend and its quality, and how the race goes.
func ConditionTip(c body.Condition, d body.CondDef, cv ConditionView, tended bool) string {
	t := fmt.Sprintf("%s\nSeverity %d%% · immunity %d%%.", cv.Label, cv.SevPct, cv.ImmPct)
	switch {
	case tended && d.CureTendSum > 0:
		t += fmt.Sprintf("\nTended at %d%% quality; the next tend can follow in %d hours.", cv.TendPct, cv.DueInHours)
	case tended:
		t += fmt.Sprintf("\nTended at %d%% quality: slowed by %d%% a day for %d more hours.", cv.TendPct, int(d.TendMax*c.TendQ/1000)/10, cv.DueInHours)
	case d.TendEvery > 0 && c.TendQ > 0:
		t += fmt.Sprintf("\nLast tended at %d%% quality; it needs tending again now.", cv.TendPct)
	case d.TendEvery > 0:
		t += "\nUntended: it needs a doctor now."
	}
	switch {
	case d.CureTendSum > 0: // MED-04: parasites
		t += fmt.Sprintf("\nNo immunity: cured by tending once the tends add up to %d%% quality (%d%% so far).", d.CureTendSum/10, int(c.TendSum)/10)
	case d.ImmPerDay == 0: // no race to win: food poisoning, anesthetic, overdose, chronic conditions
	case c.Immunity >= infection.Full:
		t += "\nImmune: it is receding."
	case cv.Losing:
		t += "\nLosing: at this rate it kills before immunity builds. Better tends (skill, medicine, a hospital bed) slow it."
	default:
		t += "\nWinning: immunity should build before it gets worse."
	}
	return t
}
