package msgfmt

import (
	"fmt"
	"slices"
	"testing"
)

func TestJoinOfSplitIsSprintf(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		format string
		args   []any
		key    string
		parts  []string
	}{
		{"%d %s rotted away in %s's pack", []any{12, "meat", "Hana"}, "%s %s rotted away in %s's pack", []string{"12", "meat", "Hana"}},
		{"%.1f °C, %5.1f%% done", []any{3.25, 40.0}, "%s °C, %s%% done", []string{"3.2", " 40.0"}},
		{"%+d mood, %q said", []any{5, "hi"}, "%s mood, %s said", []string{"+5", `"hi"`}},
		{"%v and %t", []any{[]int{1, 2}, true}, "%s and %s", []string{"[1 2]", "true"}},
		{"no verbs, 100%%", nil, "no verbs, 100%%", []string{}},
		{"index %[1]d %[1]d", []any{3}, "index 3 3", nil},
		{"star %*d", []any{4, 7}, "star    7", nil},
		{"missing %d %d", []any{1}, "missing 1 %%!d(MISSING)", nil},
		{"extra %d", []any{1, 2}, "extra 1%%!(EXTRA int=2)", nil},
		{"cut off %", nil, "cut off %%!(NOVERB)", nil},
		{"a %s with 50% inside", []any{"50%"}, "a 50%% with 50%%!i(MISSING)nside", nil},
	} {
		key, parts := Split(c.format, c.args...)
		if key != c.key || !slices.Equal(parts, c.parts) {
			t.Errorf("Split(%q) = %q %q, want %q %q", c.format, key, parts, c.key, c.parts)
		}
		if k, ok := Key(c.format); parts != nil && (!ok || k != key) {
			t.Errorf("Key(%q) = %q %v, want %q", c.format, k, ok, key)
		}
		if got, want := Join(key, parts), fmt.Sprintf(c.format, c.args...); got != want {
			t.Errorf("Join(Split(%q)) = %q, want %q", c.format, got, want)
		}
	}
}

func TestTextPrintsAsSprintf(t *testing.T) {
	t.Parallel()
	if m := Of("%d %s", 3, "meat"); m.String() != "3 meat" || m.Format != "%s %s" || !slices.Equal(m.Args(), []string{"3", "meat"}) {
		t.Fatalf("Of = %+v (%q)", m, m.String())
	}
	if m := Of("[%s]", ""); m.String() != "[]" || len(m.Args()) != 1 {
		t.Fatalf("one empty argument: %+v %q", m, m.Args())
	}
	if m := Of("none"); m.Args() != nil || m != Make("none", nil) {
		t.Fatalf("no arguments: %+v", m)
	}
}

func TestPlainPrintsUnchanged(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "plain", "50% off", "%d %s %%"} {
		if got := Join(Plain(s), nil); got != s {
			t.Errorf("Join(Plain(%q)) = %q", s, got)
		}
	}
}

func TestATranslationMayReorder(t *testing.T) {
	t.Parallel()
	key, parts := Split("%s ate %d %s", "Hex", 2, "berries")
	if got := Join("%[3]s (%[2]s) — %[1]s", parts); key != "%s ate %s %s" || got != "berries (2) — Hex" {
		t.Fatalf("key %q, reordered %q", key, got)
	}
}
