// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package social

func PairRank(k PairKind) int {
	switch k {
	case PairSpouse:
		return 3
	case PairFiance:
		return 2
	case PairLover:
		return 1
	}
	return 0
}
