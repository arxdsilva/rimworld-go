# ART-027: weapons and items held the RimWorld way

## Why
After ART-025, pawns have no hands, and every held item hung from the body's
side `hold` point, so weapons, tools and loads looked like they floated. The
user chose RimWorld-faithful handling. This copies observable behaviour with
our own values; no code was copied.

## Behaviour
All placement is measured from the body centre (the head's x, halfway
between head and feet). Weapons turn about their own centre.

| Situation | What is drawn |
|---|---|
| Off duty: idle, walking, working, eating | nothing in hand; no slung rifle, no tools, no spoon or bowl |
| On duty at rest (drafted, hostile and not fleeing, hunting) | weapon carried 0.18 tile below centre, muzzle 0.72 rad down; east +0.14 tile, west mirrored; behind the body facing north |
| Aiming or melee | weapon 0.35 tile out toward the target, flipped upright when aiming left, behind the body only when aiming north |
| Hauling | a 0.5-tile item at the chest (0.07 below centre), behind the body facing north |

- A shot kicks the muzzle 0.3 rad and pulls the weapon back 0.06 tile; the body recoil stays.
- Melee: the knife and spear jab along the aim, and the club swings +0.8 → −0.45 rad. Fists draw nothing, but the body still lunges.
- A flinch keeps the load or the aimed weapon.
- Work keeps lean, dip and squash toward the job. Particles come from the worked tile's face toward the pawn.
- **Work progress bar** (`work_progress.go`):
  - shown while `ActionProgress` is in (0,1), for colonists only, and not for eating, research or horseshoes;
  - 0.6 × 0.06 tile, accent fill on the slider track;
  - shown only at ≥ 24 px per tile, one per work tile;
  - sits along the worked tile's bottom edge, not above it, so it doesn't cross the worker.

## Art
- Deleted every `tool/*` sprite (axe, bandage, bowl, broom, hammer, pickaxe, shovel, sickle, spoon, trowel) with its mask, normal map and metadata, and `weapon/rifle_slung`.
- `gear.py` keeps the rifle, knife, spear and club. The validator entries are updated.

## Verify
- The full ebiten package passes under `-race`, and the lead's full gate ran. The validator reports 0 errors.
- Rewritten and new tests in held, combat_anim, work_anim, burial_anim, anim, art and `work_progress_test.go`. Among them:
  - `TestNoWeaponWhenOffDuty`
  - `TestCarriedWeaponOverlapsBody`
  - `TestRifleAimsAtTarget`
  - `TestWorkDrawsNoTool`
  - `TestLoadKeptDuringFlinch`
  - `TestWorkProgressBar*`
- `BenchmarkDrawArt500` z1: 934 → 952 µs (noise).
- Captures in the session scratchpad (`look/027/`):
  - weapons sit diagonally across the torso;
  - aimed rifles point at their targets and fire from the barrel tip;
  - idle colonists hold nothing.

## Docs
ART §10, §15 and §27.2, `ebiten/AGENTS.md`, `assets/AGENTS.md`. UI checklist "Progress bar" is `[-]`: it is an in-world bar, not a shared widget.
