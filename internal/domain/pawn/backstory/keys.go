// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package backstory

// Stable lowercase keys for the incapability bits and tech tags, as defs
// files and patches name them (PLG-021). Append only, like the bits.
var (
	bitKeys = [numBits]string{"violence", "caring", "menial", "intellectual", "animals",
		"cooking", "plants", "mining", "crafting", "construction"}
	tagKeys = [...]string{"neolithic", "industrial", "spacer"}
)

// BitKey is a single bit's key ("violence"); "" unless b is exactly one
// valid bit.
func BitKey(b Bits) string {
	for i := range numBits {
		if b == 1<<i {
			return bitKeys[i]
		}
	}
	return ""
}

// BitOfKey finds a bit by its key.
func BitOfKey(k string) (Bits, bool) {
	for i, s := range bitKeys {
		if s == k {
			return 1 << i, true
		}
	}
	return 0, false
}

// TagKey is a single tag's key ("neolithic"); "" unless t is exactly one
// known tag.
func TagKey(t Tag) string {
	for i, s := range tagKeys {
		if t == 1<<i {
			return s
		}
	}
	return ""
}

// TagOfKey finds a tag by its key.
func TagOfKey(k string) (Tag, bool) {
	for i, s := range tagKeys {
		if s == k {
			return 1 << i, true
		}
	}
	return 0, false
}

// AllTags is every known tag.
const AllTags = TagNeolithic | TagIndustrial | TagSpacer
