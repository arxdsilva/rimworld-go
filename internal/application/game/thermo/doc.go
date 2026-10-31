// Package thermo is the room temperature model (TEMP-1, ENG-003: the
// RimWorld wiki's numbers, original names and prose): every
// temperature.EqualiseInterval ticks each room's air moves toward the
// outside through its walls and roof (domain/world/temperature
// Exposure.Equalise, calibrated to the wiki's wall table), open doors and
// vents mix it with the rooms and outdoors beside them, and the heat
// sources in it push heat in heat/s spread over its cells: heaters,
// coolers, passive coolers, campfires, torch lamps, working benches,
// burning generators, fires, steam vents and warm bodies.
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. The
// simulation comes in through the consumer-owned roles in deps.go, which
// the game package implements with small adapter types (thermo_host.go).
// The game keeps one *Model and calls Step on the climate phase and Remap
// on every room rebuild.
//
// Per-room exposure (wall faces, roof cells), the door and vent links and
// the heat-source sites are counted once per room rebuild, roof change or
// structure change and cached; a step walks rooms, links, sites and the
// host's body list only, never the map. The model is deterministic: no
// random numbers, no wall clock, no map iteration that changes results.
package thermo
