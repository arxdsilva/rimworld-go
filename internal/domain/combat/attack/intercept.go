// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package attack

// Interception distance endpoints, in tiles: no interception scaling at or
// below InterceptNear, full at or beyond InterceptFull.
const (
	InterceptNear = 5
	InterceptFull = 12
)

// InterceptScale is the distance factor of interception in permille: 0 up
// to InterceptNear tiles, 1000 from InterceptFull, linear between
// (truncating). wiki: Combat; linear between the endpoints is original
// tuning.
func InterceptScale(dist int) Permille {
	switch {
	case dist <= InterceptNear:
		return 0
	case dist >= InterceptFull:
		return one
	}
	return one * (dist - InterceptNear) / (InterceptFull - InterceptNear)
}
