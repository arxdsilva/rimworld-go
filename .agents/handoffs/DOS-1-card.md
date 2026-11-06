# DOS-1: the pawn card's data

`Game.Dossier(id)` fills `dossierview.Card` for one id, colonists and animals, with no scan and no writes. `command.SetName` renames colonists and tame animals. `Game.LearnedThisWeek` is gone; read `Card.LearnedWeek`.

## Where things live

- `game/bio_view.go`: `Dossier`, `fillPawnCard`, `fillAnimalCard`, `applySetName`.
- `game/skills.go`: `skillState.stun` (unsaved), `petNames` (saved as `PetNames`), `saturated`, `lastWeek`, free `activeSkill`.
- `game/dossierview/build.go`: `New`, `TicksLeft`, `WorkTypeOf`, `RunsBill`, `Lines` (the `ExtraLines` formulas), `CleanName`.
- `hitHarm.Hold` in `game/body.go` is the one writer of the stun record.

## What a reader should know

- Nothing was added to `Card`. RHUD-147 takes the highest severity as its stand-in and `sheet` derives wound tone from `PartView`.
- Every `ExtraLines` row has a real formula, none dropped: 228 `training.StepChance` (unbonded), 235 cook poison, 236 butchery, 245 surgery (clean, lit room, herbal medicine, implant), 249 and 250 melee hit and dodge, 251 mining yield, 255 harvest yield.
- `ExtraLines` holds whole percents, so 235 reads 0 at 0.25 % and 1 at 0.5 %.
- `Saturated` is true one point below the cap, since `learn` can stop a passionate pawn a point short of it.
- Empty by design: a bio-less pawn's `AgeDayOfYear` is 0; an animal's `Faction` is "" (no colony faction row, own ones show `ui_icon/faction_player`).
- `StunTicks` does not cover EMP-stunned mechs: `raid.Mechs` exposes a bool only, and `sheet` falls back to `a.Stunned`.
- `dossierview.CleanName` mirrors private `roster.checkName`. The rename dialog should call it, so what it accepts is what the handler accepts.

## SetName and saves

- A colonist's "" restores `bioName(personBio)`, recoverable because a rename never touches the bio.
- A pet's generated name came from `g.rng`, so its first rename keeps the old one in `PetNames`. New field, additive, nil until a rename. `SaveVersion` stays 13. The schema refresh moved only the four `saveState.PetNames*` lines.
- A rename moves the state hash through `Names` or the animal's `Name`, by design. Restoring brings it back (`TestSetNameRoundTripsASave`).

## Settled rows (`.cursor/RIMHUD_GAP_ANALYSIS.md`)

- 119 excluded: no pack flag; a caravan takes any tame animal, so the label would name them all. Removed from the DOS-1 list.
- 182 stays partial: sleepers skip `decayNeeds`, bed or not, so the bed clause goes.
- 228 partial, 244 partial: real formulas, DOS-2's guesses agreed.
- 298: `WorkTypeOf`, from `assignWorkType`; bills by the recipe's skill.

Statuses of delivered rows other than these are left for the lead's final pass.

## Open

- `inspector/AGENTS.md:12` still names `LearnedThisWeek` (not owned here).
- `journey_mods_owed_test.go` still owes SetName.
- One `Game` method freed; the lead lowers `solid_budget.txt`.
