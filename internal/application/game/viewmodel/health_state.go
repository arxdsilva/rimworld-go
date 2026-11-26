// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package viewmodel

import (
	"fmt"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
)

func HealthState(v health.Record, b *body.Body) string {
	bleed := b.BleedPerDay()
	switch {
	case v.Status == health.Dead:
		return "Dead"
	case bleed > 0:
		return fmt.Sprintf("Bleeding %d%%/day · death in %dh", bleed/10, b.BleedDeathHours())
	case v.Status == health.Downed:
		return "Downed"
	case body.HasOpenWound(b) || b.BloodLoss > 0:
		return "Injured"
	}
	return "Healthy"
}
