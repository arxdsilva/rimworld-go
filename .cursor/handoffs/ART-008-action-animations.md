# ART-008 Action animations: holding, combat, hits and work

## What changed
- **Spec:** ART §27.2 (normative) covers the pose sets, two-hand holding,
  holster rules, strike and hit timing, and per-action particles. §15
  covers the gear attachments.
- **Pawn rig** (`pawn.py`, ~10 min build):
  - Joints: elbows, wrists, shoulder abduction and twist, torso lean and
    side bend, head yaw and pitch, hip spread, pelvis sink.
  - Two-bone arm and leg solvers; quaternion shoulder blending.
  - Every body frame has `hand`, `hand_l` and `head`.
  - Pose sets in all facings, for body, pants, shirt, vest, parka and
    duster:
    - combat: ready, aim, fire1-3, stab1-6, thrust1-6, swing1-6, punch1-4
    - work: chop1-6, mine1-6, hammer1-4, kneel and plant1-4, pick1-4,
      bench1-4, tend1-4, eat1-4
    - other: carry and carry_walk01-12, flinch1-3, fall1-4, offer1-2
  - Pawn PNGs 792 → 4232 (28 MB). Walk and work frames are unchanged
    within 2/255.
- **Gear** (new Blender family `gear`, split out of `pawn.py`):
  - `weapon/rifle_basic` (stock, receiver, bolt, magazine, sights, sling)
    and `weapon/rifle_slung`
  - `weapon/knife`, `weapon/spear`, `weapon/club`
  - tools: axe, pickaxe, hammer, trowel, sickle, bandage, bowl, spoon
  - Attachments: `grip`, `support` (between `grip` and the working end),
    `muzzle`/`stock`, `tip` and `head`.
  - Ground items `item/knife` and `item/spear`.
- **Animals:** dustbuck `kick1-4`, ridgewolf `bite1-4`, and `graze1-2` and
  `flinch1-2` for both.
- **Renderer seams:** `anim.go` was split into `combat_anim.go`,
  `work_anim.go` and `held.go`. `standingPose` tries `hitReaction`, then
  `combatPose`, `carryPose` and `workPose`, and returns the held item.
  `drawHeld` draws it behind or in front of the body.
- **Holding** (`held.go`):
  - Two-handed items put `grip` on `hand` and turn so `support` lies on
    the line to `hand_l`.
  - One-handed items lie along the forearm.
  - Off duty the rifle is slung, the knife stays on the belt (not drawn),
    the spear is carried upright and the club stays in hand.
  - An off-hand slot (`off`) carries the bowl while eating.
  - Loads are drawn between the hands.
- **Combat** (`combat_anim.go`, `draw_combat.go`):
  - Aiming blends ready to aim with `AttackCycle` and tracks the target.
  - Firing plays recoil and muzzle rise, with a flash and smoke at the
    muzzle, an ejected casing, and a tracer from the muzzle.
  - Stab, thrust, swing and punch strike frames land on the attack's trace
    (first sighting, `Age <= 1`); melee arcs follow the tip.
  - A hit plays a flinch with a 0.12-tile stagger and blood away from the
    attacker. A downed pawn falls, then lies away from the last hit.
  - Animals bite, kick and flinch in sync.
  - Pawns turn to their attack target before the first shot
    (`targetFacing`).
- **Work** (`work_anim.go`): one action table drives every work action:

  | Action | Held | Particles |
  |---|---|---|
  | chop | axe | wood chips |
  | mine, breach | pickaxe | sparks and dust |
  | build, deconstruct, repair | hammer | sawdust |
  | sow (kneel) | trowel | soil |
  | harvest (crouch) | sickle | leaves |
  | craft, cook, tailor, butcher, research | bench pose | per action |
  | tend (kneel) | bandage | none |
  | tame | open hand | none |
  | eat | spoon and bowl | none |
  | refuel | bench pose | chips |

  Hauling and rescue show a carry pose while walking. Work loops run on
  `animT`.
- **Particles:** `partSpark`, `partCasing`, `partSawdust` and `burstDir`.
- **Validator:** checks every action set, the hand/hand_l/head anchors,
  animal action frames, and the gear grip → support → end order.

## Verification
- Tests: `combat_anim_test.go`, `held_test.go`, `work_anim_test.go`,
  `pawn_anim_test.go`, plus adapted `anim_test.go`. ebiten coverage
  43.4% → 52.3%.
- All gates green. Staticcheck still shows 3 old warnings in untouched
  test files.
- Contact sheets of every pose set (east/south/north/west at 25%), gear
  and animal sheets with attachment dots, and frame strips reviewed:
  - armory melee: knife stab, spear thrust, club wind-up, rifle aim and
    fire, hit flashes and blood, falls
  - chores: hammering a door from behind, two-handed axe chopping with
    chips, then the tree falling
- README: new `melee` and `chores` clips; media regenerated.

## Known limitations / follow-ups
- The chibi proportions limit overhead raises: chop and mine hands come up
  in front of the forehead, and in south view they cover the face. North
  poses are foreshortened.
- The `hits` capture view never finds a blow to centre on (pre-existing).
  Use `-scenario armory -capture-ui raid` instead.
- Floating damage text can overlap ("-11" over "miss"), which is
  pre-existing.
- `horseshoes` uses the generic work frames (no dedicated set).
- No pistol, shotgun or explosions; no animation LOD at far zoom.
