// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savecanon

// Frozen is the ordered save values after the application has detached
// every mutable reference. It carries no simulation callback or Game.
// Ownership passes to one consumer; it can be discarded without encoding.
type Frozen struct {
	values []any
}

// Freeze takes ownership of values and their detached payloads. The caller
// must not subsequently mutate them or retain mutable aliases.
func Freeze(values ...any) Frozen { return Frozen{values: values} }

// Encode performs the standard Gob serialization of the frozen values.
// It reads no live state and produces the existing Captured representation.
func (f Frozen) Encode(reuse []byte) (Captured, error) {
	return Capture(reuse, f.values...)
}
