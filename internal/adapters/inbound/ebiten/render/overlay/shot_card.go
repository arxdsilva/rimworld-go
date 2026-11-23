// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package overlay

import "rimworld-go/internal/domain/combat/attack"

// Hit-card line labels, one per step of attack.Report.Chain after the
// weapon band (SHT-201).
var stepLabels = [attack.NumSteps + 1]string{
	attack.StepBand:    "weapon",
	attack.StepShooter: "shooter",
	attack.StepWeather: "weather",
	attack.StepSize:    "target size",
	attack.StepLight:   "light",
	attack.StepDowned:  "downed target",
	attack.StepClamp:   "cap",
	attack.StepCover:   "cover",
	attack.NumSteps:    "melee", // the one line of a melee card
}

// ShotInfoOf is the hit-chance card of a report: the headline chance and
// one line per factor, or why the shot is blocked. Each line is the
// difference of two successive Chain values in percent (the first is the
// weapon band itself), so the lines add up to the headline exactly (AC-8).
// A melee report has the one line "melee". factors is reused scratch.
func ShotInfoOf(factors []ShotFactor, rep attack.Report, fx, fy, tx, ty int) ShotInfo {
	chain := rep.Chain()
	head := int(chain[attack.StepCover]) / 10
	s := ShotInfo{FromX: fx, FromY: fy, ToX: tx, ToY: ty, Chance: head, Factors: factors}
	switch {
	case !rep.InRange:
		s.Blocked, s.Reason = true, "out of range"
	case !rep.InSight:
		s.Blocked, s.Reason = true, "no line of sight"
	}
	if rep.Melee {
		s.Factors = append(s.Factors, ShotFactor{Label: stepLabels[attack.NumSteps], Pct: head})
		return s
	}
	prev := 0
	for i := attack.StepBand; i < attack.NumSteps; i++ {
		cur := int(chain[i]) / 10
		if d := cur - prev; d != 0 || i == attack.StepBand {
			s.Factors = append(s.Factors, ShotFactor{Label: stepLabels[i], Pct: d})
		}
		prev = cur
	}
	return s
}
