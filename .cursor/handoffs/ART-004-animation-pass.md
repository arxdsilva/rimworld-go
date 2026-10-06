# Task Handoff

Task ID: ART-004
Status: complete (animation pass)

## Summary

Presentation-only animation across pawns, animals, combat, work and the world. Nothing feeds back into the simulation, and visual randomness is `mix64` of stable keys.

**Blender walk cycles**
- Pawns: 8-frame cycle (neutral, w1, w2, w1, neutral, w3, w4, w3), one stride per tile so the feet stay planted.
  - Legs, pant legs, arms and sleeves swing about hip and shoulder pivots, and the body holdout gets the same pose.
  - Rendered for body/pants/shirt/vest in 4 facings. The hand attachment follows the swinging arm, so held weapons swing too.
- Animals: diagonal-pair trot frames for dustbuck and ridgewolf.

**Pawns**
- Walk frame and bob from tiles walked plus step progress.
- Idle breathing, with a per-pawn phase.
- Work swings towards the worked tile, with particles at each strike:
  - cut / construct / deconstruct: wood chips
  - mine: stone dust
  - harvest: leaves
  - sow: soil
  - workbench: shavings
  - butcher table: blood
  - stove: steam, plus a flickering burner glow
- Rifle recoil, melee lunge, knockback and a red hit flash when struck.
- Falling over when downed or killed: dead pawns go grey, downed ones breathe.
- In bed: laid along the bed's axis with the head on the pillow, breathing, under a blanket overlay (new Blender `*_blanket_<facing>` sprites).
- Carried across the rescuer's shoulders.

**Animals**
- Trot frames, a bounce that grows when fleeing or attacking, idle breathing.
- Bite lunges and hit reactions.
- Dust puffs on each step while running.

**Combat effects**
- Blood spray on hits and muzzle smoke, spawned once per trace (on top of the existing tracers and swing arcs).

**World**
- Wind sway on trees, bushes and crops: per-tile phase with slow gusts.
- New drops pop in (ease-out-back).
- Hunt crosshairs spin, the selection bracket pulses and danger alerts pulse.

**Simulation fixes found while animating**
- Fully rested sleepers stay in bed during scheduled sleep. They used to complete and re-start sleep every tick.
- Doctors stand beside the patient or bed instead of on it.
- `PawnView.Acting/ActX/ActY` exposes the hands-on work in progress.

**Dev tooling**
- `-capture-frames N` records a frame sequence (lock-stepped: one simulation frame per saved image).
- `-capture-ui follow` follows a working or rescuing colonist.
- `-scenario rescue|beds`, plus `Game.Injure` and `Game.BuildNow` helpers.

## Follow-up: work arm animation

- Blender arm frames `work1` (wind-up overhead), `work2` (mid-swing) and `work3` (strike, low and forward) for the body and shirt layers in 4 facings. It is a two-handed grip so it reads in every facing, and the hand attachment follows the tool arm.
- Tool sprites `tool/hammer`, `tool/axe` and `tool/pickaxe` (grip and head attachments, pointing east).
- Swing schedule: mid, wind-up hold, a fast snap to the strike at 48% of the swing, recover.
  - The body leans back on the wind-up and forward on the strike.
  - Impact particles land on the strike frame.
- Tools by job:
  - hammer: construct, deconstruct, workbench
  - axe: cut
  - pickaxe: mine
  - bare-hand chop: butcher
- Low reaching (work3/work2) for harvest, sow, stove and tend.
- While working, the tool replaces the held weapon.

## Verification

- `go test -race ./...` green; vet and staticcheck clean, apart from the known SA4000s.
- New tests:
  - walk cycle cadence and phase
  - attack reactions (recoil, knockback, flash)
  - work swing and strike timing
  - particle expiry and cap
  - item pop and wind sway
- Art-coverage test now requires every walk/trot frame and the blanket overlays.
- Asset validator: 320 sprites, 0 errors. It now checks complete walk/trot cycles and matching canvases.
- Sequences checked frame by frame: work, fight, rescue, and night beds in all facings. GIFs are kept locally in `_to_delete/gif/` (not committed).

## Known Limitations

- No body-part animation for animals (tail, head).
- Particles are drawn under the night veil, so they dim at night (fine for dust, less so for sparks).
- No animation LOD at far zoom yet.
