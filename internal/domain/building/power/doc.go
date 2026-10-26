// Package power is the colony's electricity, as pure rules (PLAY-021):
// what each building kind does on a grid (SpecOf), how conduits, switches
// and devices join into nets (Builder.Build), and how one net balances
// its generation, its consumers and its batteries over a span of ticks
// (Balance).
//
// Units are integers: power in W, stored energy in W·ticks (WTicksPerWh
// of them make one Wh, shown to the player). The package is headless and
// deterministic: no clock, no RNG, no map access; the simulation hands it
// nodes and devices and stores what it returns. It imports construction
// only.
package power
