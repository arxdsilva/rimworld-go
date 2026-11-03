// Package mathx holds the integer helpers the standard library leaves out:
// math.Abs is float64 only, and min and max need composing to clamp.
package mathx

// Abs is the absolute value of v. Like the two's-complement negation it is,
// Abs of the smallest int is that same negative number.
func Abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Clamp limits v to lo..hi, which needs lo <= hi: with lo > hi it returns hi.
func Clamp(v, lo, hi int) int { return min(max(v, lo), hi) }
