# SIM-020 Blood and ichor: stains on floors and bodies

## What changed
- **Domain:** `internal/domain/world/stain` (pure). It defines the `Blood` and `Ichor` kinds, `Stain{X, Y, Kind, Amount, Born, Variant, Rem}` and the fade, wash and add rules.
  - `Full` = 1000.
  - **Fade:** a full stain is gone in exactly 7 days (`FadeTicks` = 420,000).
  - **Wash:** off open ground, at intensity 1, in 2 h of rain (`RainWash`) or 3 h of snow (`SnowWash`).
  - **Cap:** 2,000 stains; the faintest are dropped first.
  - **Exact sums:** fixed-point remainders (`Rem`, saved) make 250-tick steps add up exactly.
- **Game:** `game/stains.go`.

  | Event | Amount |
  |---|---|
  | Hit | 40 + 25·dmg on the target's tile. A rifle hit (14) is a splat, and two hits make a pool. |
  | Heavy hit (dmg ≥ 6) | Also 40% of the hit amount on the tile beyond the target, away from the attacker. |
  | Death of a pawn or animal | A 700 pool. |
  | Bleeding, while moving | A drip of 80 every third tile. |
  | Downed and untended | A 60 pool per in-game hour, up to 600. |

  - **Bleeding:** colonists with an open, untended wound and Injury ≥ 15; raiders with Injury ≥ 30; animals below 50% HP.
  - **Ichor:** hive insects bleed ichor; everything else bleeds blood.
  - **Decay:** in `phaseWeather`, every climate step (250 ticks). Under a roof, stains only fade.
  - **Determinism:** no RNG; the variant comes from a hash of the tile.
  - **Views:** `WorldView.Stains`, sorted by row, then column.
  - **Saves:** `Stains` is additive, so older saves load clean.
- **Renderer** (`blood_draw.go`):
  - **Floor decals:** procedural drips (< 0.2), splats (< 0.6) and pools, from one atlas.
    - They dry from fresh red to dark brown over a day; ichor is yellow-green.
    - Alpha = amount × 0.85. They're lit like the ground and drawn over built floors, as one lit batch.
  - **Bodies:** `lit.kage` `Custom3 = 32·ichor + 2·wetStep + coverage` blotches body and apparel layers by missing health, with a wet sheen for 2 s after a hit.
    - Hair, held items and carried patients stay clean.
    - The shader branch is skipped when `Custom3` is 0.
  - **Sprays:** 4 + dmg/2 particles (at most 16), faster for hits of 10 or more, plus 2 droplets back toward a melee attacker; ichor for insects.

## Verification
- **Tests:** `stain` 100%; `game` 88.1% (the stain tests are in `stains_test.go`); the renderer blood tests are in `blood_draw_test.go`. The renderer package is at 70.5%, up from 69.7% (it was already below the 80% bar).
- **Determinism:** ×3, 0 failures.
- **Benchmarks:**

  | Benchmark | Result |
  |---|---|
  | StainDecay (2,000 stains) | 0.074 ms |
  | DrawStains (500) | 0.017 ms |
  | DrawArt | flat (2.5–2.8 ms) |
  | DrawArtStains (500) | 2.56–2.88 ms |

  Hour and Raid are within noise.
- **Captures (armory):**
  - hit splats, pools under the fallen, bloodied raiders, darkening at night
  - an 8-day time-lapse: dried brown by day 2, washed off by rain on day 3
  - a melee brawl leaves a line of pools where the fight held (traced: a dozen hits on the same few tiles)

## Known limitations
- No cleaning job, room cleanliness or tracked-in filth (Phase 13).
- Stains under a wall or door built later stay in the sim until they fade.
- Corpses don't keep body coverage; corpse sprites have their own dried blood.
