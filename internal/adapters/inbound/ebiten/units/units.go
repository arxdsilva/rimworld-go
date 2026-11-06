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

// UnitF and UnitK are the Gameplay.TempUnit settings that select Fahrenheit
// and Kelvin (OPT-007); any other value (normally "") is Celsius.
const (
	UnitF = "F"
	UnitK = "K"
)

// Temp is how temperatures read. The zero value is Celsius; F and K are
// exclusive (F wins if both are set).
type Temp struct{ F, K bool }

// kelvinZero is 0 °C in kelvin.
const kelvinZero = 273.15

// Celsius is the default formatter.
var Celsius = Temp{}

// FromUnit builds the formatter of a Gameplay.TempUnit setting.
func FromUnit(unit string) Temp { return Temp{F: unit == UnitF, K: unit == UnitK} }

// Name is the unit's long name ("Celsius").
func (u Temp) Name() string {
	if u.F {
		return "Fahrenheit"
	}
	if u.K {
		return "Kelvin"
	}
	return "Celsius"
}

// Symbol is "°C", "°F" or "K" (Kelvin has no degree sign).
func (u Temp) Symbol() string {
	if u.F {
		return "°F"
	}
	if u.K {
		return "K"
	}
	return "°C"
}

// Value converts degrees Celsius to the chosen unit.
func (u Temp) Value(c float64) float64 {
	if u.F {
		return c*9/5 + 32
	}
	if u.K {
		return c + kelvinZero
	}
	return c
}

// FromDisplay is the inverse of Value: a reading in the chosen unit back
// to degrees Celsius.
func (u Temp) FromDisplay(v float64) float64 {
	if u.F {
		return (v - 32) * 5 / 9
	}
	if u.K {
		return v - kelvinZero
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
	if (!u.F && !u.K) || !strings.Contains(s, mark) {
		return s
	}
	suffix := " °F"
	if u.K {
		suffix = " K"
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
		b.WriteString(strconv.FormatFloat(u.Delta(n), 'f', -1, 64) + suffix)
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
	return strconv.Itoa(round(u.Value(float64(tenths)/10))) + strings.TrimPrefix(u.Symbol(), "°")
}

// Target formats a thermostat or comfort figure in tenths of a degree
// Celsius: whole degrees when whole, else one decimal ("21.5 °C").
func (u Temp) Target(tenths int) string {
	if !u.F && !u.K {
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
	deg := " " + u.Symbol()
	if math.Abs(v-math.Round(v)) < 0.05 {
		return fmt.Sprintf("%d%s", round(v), deg)
	}
	return fmt.Sprintf("%.1f%s", v, deg)
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
	if u.K {
		return "1 K"
	}
	return "1 °C"
}
