// Package units formats temperatures for the player (WG-16): one value
// type built from the settings, carried by whoever draws a temperature.
// It holds no package state; the zero value is Celsius.
package units

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// UnitF is the Gameplay.TempUnit setting that selects Fahrenheit; any
// other value (normally "") is Celsius.
const UnitF = "F"

// Temp is how temperatures read. The zero value is Celsius.
type Temp struct{ F bool }

// Celsius is the default formatter.
var Celsius = Temp{}

// FromUnit builds the formatter of a Gameplay.TempUnit setting.
func FromUnit(unit string) Temp { return Temp{F: unit == UnitF} }

// Name is the unit's long name ("Celsius").
func (u Temp) Name() string {
	if u.F {
		return "Fahrenheit"
	}
	return "Celsius"
}

// Symbol is "°C" or "°F".
func (u Temp) Symbol() string {
	if u.F {
		return "°F"
	}
	return "°C"
}

// Value converts degrees Celsius to the chosen unit.
func (u Temp) Value(c float64) float64 {
	if u.F {
		return c*9/5 + 32
	}
	return c
}

// FromDisplay is the inverse of Value: a reading in the chosen unit back
// to degrees Celsius.
func (u Temp) FromDisplay(v float64) float64 {
	if u.F {
		return (v - 32) * 5 / 9
	}
	return v
}

// Delta converts a temperature difference in Celsius degrees.
func (u Temp) Delta(c float64) float64 {
	if u.F {
		return c * 9 / 5
	}
	return c
}

// Text rewrites the temperature differences in a domain sentence ("-8 °C all
// year", "Comfort +2.5 °C") into the chosen unit; the domain's sentences only
// ever state differences. Celsius returns s itself.
func (u Temp) Text(s string) string {
	const mark = " °C"
	if !u.F || !strings.Contains(s, mark) {
		return s
	}
	var b strings.Builder
	for {
		at := strings.Index(s, mark)
		if at < 0 {
			b.WriteString(s)
			return b.String()
		}
		start := at
		for start > 0 && strings.IndexByte("+-.0123456789", s[start-1]) >= 0 {
			start--
		}
		n, err := strconv.ParseFloat(s[start:at], 64)
		if err != nil { // no number before the mark: leave it
			b.WriteString(s[:at+len(mark)])
			s = s[at+len(mark):]
			continue
		}
		b.WriteString(s[:start])
		if s[start] == '+' {
			b.WriteByte('+')
		}
		b.WriteString(strconv.FormatFloat(u.Delta(n), 'f', -1, 64) + " °F")
		s = s[at+len(mark):]
	}
}

func round(v float64) int { return int(math.Round(v)) }

// Tenths formats tenths of a degree Celsius, rounded to whole degrees of
// the chosen unit and spaced like the readouts ("21 °C").
func (u Temp) Tenths(tenths int) string {
	return strconv.Itoa(round(u.Value(float64(tenths)/10))) + " " + u.Symbol()
}

// Compact is Tenths without the space or degree sign ("-3C", "37F").
func (u Temp) Compact(tenths int) string {
	return strconv.Itoa(round(u.Value(float64(tenths)/10))) + u.Symbol()[len("°"):]
}

// Target formats a thermostat or comfort figure in tenths of a degree
// Celsius: whole degrees when whole, else one decimal ("21.5 °C").
func (u Temp) Target(tenths int) string {
	if !u.F {
		if tenths%10 == 0 {
			return fmt.Sprintf("%d °C", tenths/10)
		}
		sign, t := "", tenths
		if t < 0 {
			sign, t = "-", -t
		}
		return fmt.Sprintf("%s%d.%d °C", sign, t/10, t%10)
	}
	v := u.Value(float64(tenths) / 10)
	if math.Abs(v-math.Round(v)) < 0.05 {
		return fmt.Sprintf("%d °F", round(v))
	}
	return fmt.Sprintf("%.1f °F", v)
}

// Degrees formats a temperature in degrees Celsius to the given decimals
// ("-3.5°C").
func (u Temp) Degrees(c float64, decimals int) string {
	return strconv.FormatFloat(u.Value(c), 'f', decimals, 64) + u.Symbol()
}

// Whole is Degrees with no decimals ("21°C").
func (u Temp) Whole(c float64) string { return u.Degrees(c, 0) }

// Step is the label of a one-degree-Celsius thermostat step ("1 °C", "1.8 °F").
func (u Temp) Step() string {
	if u.F {
		return "1.8 °F"
	}
	return "1 °C"
}
