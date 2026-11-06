package options

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/platform/config"
)

// Row builders for the OPT-* tabs (General, Interface, Developer and the
// rows added to the older tabs). Each is a free function over the one row
// table (screen_options.go), so Screen stays small.

// listRow is a row that cycles through string values; name gives each
// value's label (nil: the value with a capital).
// trOf is the first catalog of an optional trailing argument, English when
// there is none (tests and tools build rows without a screen).
func trOf(tr []lang.Tr) lang.Tr {
	if len(tr) > 0 && tr[0] != nil {
		return tr[0]
	}
	return lang.English{}
}

func listRow(key, label, tip string, get func(*config.Settings) *string, values []string, name func(string) string) OptionRow {
	if name == nil {
		name = titleCase
	}
	return OptionRow{Key: key, Label: label, kind: rowChoice, Tip: tip,
		value: func(s config.Settings) string { return name(*get(&s)) },
		apply: func(s *config.Settings, _ int) { p := get(s); *p = cycle(values, *p) }}
}

// numberRow is a row that cycles through int values (a drag speed, a shake
// strength) with label text for each.
func numberRow(key, label, tip string, get func(*config.Settings) *int, values []int, text func(int) string) OptionRow {
	return OptionRow{Key: key, Label: label, kind: rowChoice, Tip: tip,
		value: func(s config.Settings) string { return text(*get(&s)) },
		apply: func(s *config.Settings, _ int) { p := get(s); *p = cycle(values, *p) }}
}

// stepRow is a − bar + row over an int in lo..hi.
func stepRow(key, label, tip string, get func(*config.Settings) *int, lo, hi, step int, text func(int) string) OptionRow {
	return OptionRow{Key: key, Label: label, kind: rowStep, Tip: tip,
		value: func(s config.Settings) string { return text(*get(&s)) },
		frac:  func(s config.Settings) float64 { return float64(*get(&s)-lo) / float64(hi-lo) },
		apply: func(s *config.Settings, d int) {
			p := get(s)
			*p = max(lo, min(hi, *p+d*step))
		}}
}

// actionRow is a one-button row that does something instead of changing a
// setting (run); button is the text on it.
func actionRow(key, label, tip, button string, run func(*Screen)) OptionRow {
	return OptionRow{Key: key, Label: label, kind: rowChoice, Tip: tip, run: run,
		value: func(config.Settings) string { return button }}
}

// col2 moves a row to the second column.
func col2(r OptionRow) OptionRow { r.Col = 1; return r }

// ladder is lo, lo+step, … hi.
func ladder(lo, hi, step int) []int {
	var out []int
	for v := lo; v <= hi; v += step {
		out = append(out, v)
	}
	return out
}

func percent(v int) string { return fmt.Sprintf("%d%%", v) }
