package main

import "testing"

func TestParseSize(t *testing.T) {
	for in, want := range map[string][2]int{"1600x900": {1600, 900}, "2560x1440": {2560, 1440}} {
		if w, h, ok := parseSize(in); !ok || w != want[0] || h != want[1] {
			t.Errorf("%q: %d %d %v", in, w, h, ok)
		}
	}
	for _, in := range []string{"", "x", "0x900", "big"} {
		if _, _, ok := parseSize(in); ok {
			t.Errorf("%q parsed", in)
		}
	}
}
