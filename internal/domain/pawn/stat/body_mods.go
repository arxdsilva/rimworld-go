// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package stat

func BodyMods(k [2]int32) []Mod {
	if k == ([2]int32{1000, 1000}) {
		return nil
	}
	var ms []Mod
	if k[0] != 1000 {
		ms = append(ms, Mod{Stat: MoveSpeed, Mul: max(k[0], 1)})
	}
	if k[1] != 1000 {
		ms = append(ms, Mod{Stat: WorkSpeed, Mul: max(k[1], 1)})
	}
	return ms
}
