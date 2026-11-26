// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package expectations defines the wealth bands used by mood and recreation.
package expectations

// The boundaries and bonuses are wiki numbers (CLEAN_ROOM expectations row,
// 2026-11-26).
// https://rimworldwiki.com/wiki/Wealth#Expectations
const (
	VeryLowAt  = 15_000
	LowAt      = 31_000
	ModerateAt = 81_000
	HighAt     = 182_000
	SkyHighAt  = 308_000
)

type Band uint8

const (
	ExtremelyLow Band = iota
	VeryLow
	Low
	Moderate
	High
	SkyHigh
)

type Effects struct {
	MoodBonus         int
	ToleranceDecayPct int
}

var effects = [...]Effects{
	{30, 18},
	{24, 13},
	{18, 11},
	{12, 10},
	{6, 8},
	{0, 7},
}

func ForWealth(w int) Band {
	switch {
	case w < VeryLowAt:
		return ExtremelyLow
	case w < LowAt:
		return VeryLow
	case w < ModerateAt:
		return Low
	case w < HighAt:
		return Moderate
	case w < SkyHighAt:
		return High
	default:
		return SkyHigh
	}
}

func (b Band) Effects() Effects {
	if b > SkyHigh {
		return Effects{}
	}
	return effects[b]
}
