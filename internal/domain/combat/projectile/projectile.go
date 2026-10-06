package projectile

// Trace is one resolved attack.
type Trace struct {
	FromX, FromY int
	ToX, ToY     int
	Tick         uint64
	Hit          bool
	Melee        bool
}

// Lifetime is how many ticks a trace stays visible.
const Lifetime = 18

// Prune drops traces older than Lifetime at now.
func Prune(ts []Trace, now uint64) []Trace {
	out := ts[:0]
	for _, t := range ts {
		if now-t.Tick < Lifetime {
			out = append(out, t)
		}
	}
	return out
}
