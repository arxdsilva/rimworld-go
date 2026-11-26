// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package item

import "rimworld-go/internal/domain/economy/quality"

// ConditionRange filters the actual ingredients, not their kinds. Upper
// endpoints are stored one up (zero = full range); zero is unrestricted.
type ConditionRange struct {
	HPMin, HPMax           uint8
	QualityMin, QualityMax uint8
}

func rangeEnds(lo, hi, top int) (uint8, uint8) {
	lo, hi = min(max(lo, 0), top), min(max(hi, 0), top)
	if lo > hi {
		lo, hi = hi, lo
	}
	if hi == top {
		return uint8(lo), 0
	}
	return uint8(lo), uint8(hi + 1)
}
func (r ConditionRange) WithHP(lo, hi int) ConditionRange {
	r.HPMin, r.HPMax = rangeEnds(lo, hi, 100)
	return r
}
func (r ConditionRange) WithQuality(lo, hi int) ConditionRange {
	r.QualityMin, r.QualityMax = rangeEnds(lo, hi, int(quality.Legendary))
	return r
}
func (r ConditionRange) HP() (lo, hi int) {
	hi = 100
	if r.HPMax != 0 {
		hi = int(r.HPMax) - 1
	}
	return int(r.HPMin), hi
}
func (r ConditionRange) Quality() (lo, hi int) {
	hi = int(quality.Legendary)
	if r.QualityMax != 0 {
		hi = int(r.QualityMax) - 1
	}
	return int(r.QualityMin), hi
}
func (r ConditionRange) Takes(st Stack) bool {
	if r == (ConditionRange{}) {
		return true
	}
	if r.HPMin != 0 || r.HPMax != 0 {
		if maxHP := st.MaxHP(); maxHP > 0 {
			lo, hi := r.HP()
			hp := st.HP() * 100 / maxHP
			if hp < lo || hp > hi {
				return false
			}
		}
	}
	if (r.QualityMin != 0 || r.QualityMax != 0) && HasQuality(st.Kind) {
		q, ok := st.QualityLevel()
		if !ok {
			q = quality.Default
		}
		lo, hi := r.Quality()
		if int(q) < lo || int(q) > hi {
			return false
		}
	}
	return true
}
