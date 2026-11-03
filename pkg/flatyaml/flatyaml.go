// Package flatyaml reads the small YAML subset our tools generate for sprite
// metadata, atlases and palettes: "key: value" lines with indentation for
// nesting, and inline [a, b] lists. It is not a YAML parser: anchors, block
// scalars, trailing comments and escapes are not understood, and an atlas or
// metadata file must not use them.
//
// Why not a YAML library is in this package's AGENTS.md.
package flatyaml

import (
	"fmt"
	"iter"
	"strconv"
	"strings"
)

// Line is one line of a file.
type Line struct {
	Number   int    // from 1, counting every line of the file
	Text     string // without trailing whitespace; the indentation stays
	Indent   int    // leading spaces
	Key      string // before the first colon, trimmed; the whole line without one
	Value    string // after the first colon, trimmed, without surrounding quotes
	HasColon bool
}

// Lines walks src, skipping blank lines and comment lines at any indentation.
// Lines of any length are fine, and CRLF endings read like LF.
func Lines(src string) iter.Seq[Line] {
	return func(yield func(Line) bool) {
		n := 0
		for raw := range strings.Lines(src) {
			n++
			text := strings.TrimRight(raw, " \t\r\n")
			trim := strings.TrimSpace(text)
			if trim == "" || trim[0] == '#' {
				continue
			}
			key, value, ok := strings.Cut(trim, ":")
			l := Line{
				Number:   n,
				Text:     text,
				Indent:   len(text) - len(strings.TrimLeft(text, " ")),
				Key:      strings.TrimSpace(key),
				Value:    strings.Trim(strings.TrimSpace(value), `"`),
				HasColon: ok,
			}
			if !yield(l) {
				return
			}
		}
	}
}

// Scalar is the value of a top-level key ("" when absent).
func Scalar(src, key string) string {
	for l := range Lines(src) {
		if l.Indent == 0 && l.HasColon && l.Key == key {
			return l.Value
		}
	}
	return ""
}

// Pair reads an inline list of two numbers, [a, b].
func Pair(v string) ([2]float64, error) {
	var out [2]float64
	parts, err := list(v, 2)
	if err != nil {
		return out, err
	}
	for i, p := range parts {
		if out[i], err = strconv.ParseFloat(strings.TrimSpace(p), 64); err != nil {
			return out, err
		}
	}
	return out, nil
}

// Ints reads an inline list of exactly n integers, n at most 4.
func Ints(v string, n int) ([4]int, error) {
	var out [4]int
	parts, err := list(v, n)
	if err != nil {
		return out, err
	}
	for i, p := range parts {
		if out[i], err = strconv.Atoi(strings.TrimSpace(p)); err != nil {
			return out, err
		}
	}
	return out, nil
}

func list(v string, n int) ([]string, error) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil, fmt.Errorf("want a list, got %q", v)
	}
	parts := strings.Split(v[1:len(v)-1], ",")
	if len(parts) != n {
		return nil, fmt.Errorf("want %d values, got %q", n, v)
	}
	return parts, nil
}
