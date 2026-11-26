// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package setupview

import "rimworld-go/internal/domain/world/mapgen/pipeline"

// BuildSteps includes map generation and the final colony setup stages.
func BuildSteps() int { return pipeline.Steps() + 3 }

// BuildPhase is the user-facing progress label for a colony build step.
func BuildPhase(i int) string {
	n := pipeline.Steps()
	switch {
	case i <= 0:
		return "Preparing the site"
	case i <= n:
		return pipeline.Label(pipeline.StepNames()[i-1])
	case i == n+1:
		return "Placing the colony"
	}
	return "Arrival"
}
