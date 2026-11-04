package sheet

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"rimworld-go/internal/domain/sim/clock"
)

// capFirst upper-cases the first letter.
func capFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// lowerFirst lower-cases the first letter.
func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToLower(r)) + s[n:]
}

// hoursText reads whole hours as "3h" or "2d 3h".
func hoursText(h int) string {
	if h < 0 {
		h = 0
	}
	if h < clock.HoursPerDay {
		return strconv.Itoa(h) + "h"
	}
	d, r := h/clock.HoursPerDay, h%clock.HoursPerDay
	if r == 0 {
		return strconv.Itoa(d) + "d"
	}
	return strconv.Itoa(d) + "d " + strconv.Itoa(r) + "h"
}

// ticksText reads a span of ticks as hours ("under 1h", "5h", "1d 2h").
func ticksText(t uint64) string {
	h := int(t / clock.TicksPerHour)
	if h < 1 {
		return "under 1h"
	}
	return hoursText(h)
}

// secondsOf is a tick span rounded up to whole seconds.
func secondsOf(ticks int) int {
	if ticks <= 0 {
		return 0
	}
	return (ticks + clock.TicksPerSecond - 1) / clock.TicksPerSecond
}

// signedInt prints v with an explicit sign.
func signedInt(v int) string {
	if v > 0 {
		return "+" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}

// pct prints permille as a whole or one-decimal percentage.
func pctText(permille int) string {
	whole, rem := permille/10, permille%10
	if rem < 0 {
		rem = -rem
	}
	if rem == 0 {
		return strconv.Itoa(whole) + "%"
	}
	if permille < 0 && whole == 0 {
		return "-0." + strconv.Itoa(rem) + "%"
	}
	return strconv.Itoa(whole) + "." + strconv.Itoa(rem) + "%"
}

// joinNonEmpty joins the non-empty parts.
func joinNonEmpty(sep string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
