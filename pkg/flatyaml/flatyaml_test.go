package flatyaml

import (
	"slices"
	"strings"
	"testing"
)

func collect(src string) []Line { return slices.Collect(Lines(src)) }

func TestLines(t *testing.T) {
	t.Parallel()
	src := "# top comment\n\nid: \"core:a/b\"\ncanvas: [10, 20]\ntint:\n  # nested comment\n  mask: \"m.png\"  \r\nno colon here\n    deep: value # kept\nlast: 1"
	want := []Line{
		{Number: 3, Text: `id: "core:a/b"`, Key: "id", Value: "core:a/b", HasColon: true},
		{Number: 4, Text: "canvas: [10, 20]", Key: "canvas", Value: "[10, 20]", HasColon: true},
		{Number: 5, Text: "tint:", Key: "tint", HasColon: true},
		{Number: 7, Text: `  mask: "m.png"`, Indent: 2, Key: "mask", Value: "m.png", HasColon: true},
		{Number: 8, Text: "no colon here", Key: "no colon here"},
		{Number: 9, Text: "    deep: value # kept", Indent: 4, Key: "deep", Value: "value # kept", HasColon: true},
		{Number: 10, Text: "last: 1", Key: "last", Value: "1", HasColon: true},
	}
	if got := collect(src); !slices.Equal(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestLinesHasNoLengthLimit(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 1<<21) // past bufio.Scanner's limits
	got := collect("a: " + long + "\nb: 2\n")
	if len(got) != 2 || got[0].Value != long || got[1].Key != "b" {
		t.Errorf("read %d lines", len(got))
	}
}

func TestLinesStopsWhenTheCallerDoes(t *testing.T) {
	t.Parallel()
	n := 0
	for range Lines("a: 1\nb: 2\nc: 3\n") {
		n++
		break
	}
	if n != 1 {
		t.Errorf("walked %d lines", n)
	}
}

func TestScalar(t *testing.T) {
	t.Parallel()
	src := "nested:\n  glow: 9\nglow: \"3\"\nglow: 4\nbare\n"
	for key, want := range map[string]string{"glow": "3", "absent": "", "nested": "", "bare": ""} {
		if got := Scalar(src, key); got != want {
			t.Errorf("Scalar(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestPair(t *testing.T) {
	t.Parallel()
	if got, err := Pair(" [1.5, -2] "); err != nil || got != [2]float64{1.5, -2} {
		t.Errorf("got %v, %v", got, err)
	}
	for _, bad := range []string{"", "1, 2", "[1]", "[1, 2, 3]", "[a, 2]"} {
		if _, err := Pair(bad); err == nil {
			t.Errorf("Pair(%q) must fail", bad)
		}
	}
}

func TestInts(t *testing.T) {
	t.Parallel()
	if got, err := Ints("[1, 2, 3, 4]", 4); err != nil || got != [4]int{1, 2, 3, 4} {
		t.Errorf("got %v, %v", got, err)
	}
	if got, err := Ints("[7, 8]", 2); err != nil || got != [4]int{7, 8} {
		t.Errorf("got %v, %v", got, err)
	}
	for _, bad := range []string{"", "1, 2", "[1]", "[1, 2, 3]", "[1.5, 2]"} {
		if _, err := Ints(bad, 2); err == nil {
			t.Errorf("Ints(%q, 2) must fail", bad)
		}
	}
}
