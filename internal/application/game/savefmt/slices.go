// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

// CanonicalSlice preserves populated records and serializes empty slices as
// nil, matching Gob's representation of allocated-but-empty slices.
func CanonicalSlice[T any](records []T) []T {
	if len(records) == 0 {
		return nil
	}
	return records
}
