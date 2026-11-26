// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animalstate

const groupPageSize = 32

type groupPage[T any] struct {
	current *[groupPageSize]T
	used    int
}

func (p *groupPage[T]) next() *T {
	if p.current == nil || p.used == groupPageSize {
		p.current = new([groupPageSize]T)
		p.used = 0
	}
	value := &p.current[p.used]
	p.used++
	return value
}
