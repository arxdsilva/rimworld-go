# NST-A: Neat Storage kinds (domain)

Done: 27 real `defs` rows (construction.go), beauty rows (furniture.go), `axMaxHP` (1x1 = 50, 2x1 and 2x2 = 100), `construction/neat.go` (`IsNeatStorage`, `NeatProfile` 1..27), research keys (`MachineTools`: safe, drum, both weapon racks; `Metalworking`: textile rack, meat hook).
Tests: `TestNeatStorageKinds`, `TestNeatStorageResearchGates` (pass with -race).
Materials: display set/shelves/wardrobes/cabinets/crates stone+metal+wood; pedestal wood or stone; weapon rack and textile rack wood or steel; Safe, Drum, horizontal/vertical racks, Meat hook steel only; pallets and dump spot wood only. All costs, work, beauty and HP are our tuning.
No UI representation here: pure kind table; the Architect/inspector packages own the HUD side.
