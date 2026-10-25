# Task Handoff

Task ID: SIM-051 (PLAY-013, MAPGEN-2 §5)
Status: complete

## Summary
New colonies start under fog of war. Everything reachable from the landing spot over walkable ground is revealed, and so is each reached tile's ring of neighbours (cliff faces, walls). Deep water doesn't stop the reveal, so lakes and far shores show. Sealed caves and rock interiors stay hidden. When a tile becomes walkable next to a revealed one (mining, deconstruction, a moved building), only the newly connected hidden area is flooded and revealed; opening 20+ tiles posts "A cave opened up." A 400-tile cave reveals in about 15 µs. Legacy colonies and old saves have no fog.

## Files
- `game/fog.go`: state, `initFog` (called by `finishSetup` after the arrival), `fogOpen`, save bitset.
- `game/fog_view.go`: `WorldView.Fog`/`FogVer` (copy-on-write); hidden items, plants, animals, buildings and frames are left out of the read models (the simulation still sees them); order refusals.
- Hooks: `setTerrain`, `removeBuilding` and moves reveal. Placement and orders refuse fogged tiles with "unexplored", except mining designations, which any hidden tile accepts (so marks don't give away where caves are; a reveal drops marks that land on non-rock). Cut, harvest, forbid, smooth and handling designations skip hidden things.
- Renderer: `fog_draw.go` draws a near-black (#0B0B0E) layer with a 1-tile soft edge, baked per 32-tile chunk and rebuilt only on `FogVer` change. The minimap shows hidden tiles black; selection and box-select skip hidden things; hover says "Unexplored". The camera starts on `WorldView.Landing`.

## Notes
- Zone, Home, plan and roof painting are not filtered on hidden tiles, nor are stains.
- Save: `saveState.Fog` (absent = no fog). StateHash skips the field for colonies without fog.
