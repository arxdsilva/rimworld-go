# GEAR handoffs

Families: GEAR. Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## GEAR-0-contract
Contract for gear, outfits, options and ailments · 2026-11-06 · 8140ec8f3
- Changed: inert contract; state hash unchanged. Analyses: `GEAR_GAP_ANALYSIS.md`, `OPTIONS_GAP_ANALYSIS.md`, `AILMENTS_GAP_ANALYSIS.md`.
- Changed: commands appended to `domain/sim/command`: `DropGear{Pawn, Mode, Index, Queue}`, `ConsumeFromPack{Pawn, Index}`, `SetApparelForced{Pawn, Index, Enabled}` (routed to `game/sense`, owner GEAR-1); `SetApparelPolicy{Pawn, Index}`, `EditApparelPolicy{Index, Mode, …}` (`ApparelPolicy*` modes), `ClearForcedApparel{Pawn}` (owner GEAR-3, `game/outfit`).
- Changed: read models `game/gearview`: `PawnView.Gear` (mass, capacity, overall armour, move %, apparel policy), `PackItemView.Gear` (grams, use), `WorldView.Gear` (policies); `craft.WornView` gains `Grams`, `Cover`, `MovePct`. Message/letter `Format`/`Args` left to OPT-4.
- Changed: settings `platform/config/prefs.go` version 10: groups `General`, `Interface`, `Camera`, `Developer` with `Normalize` clamps; `General.AutoPause` replaces `Gameplay.PauseOnThreat` (read only to migrate pre-10 files); `TempKelvin` added. Defaults keep today's behaviour, except a fresh install's AutoPause "major" (pauses on threat letters only).
- Changed: client `ebiten/lang` (`Tr`, `English`, `Tf`, `Live`); `ports.Texts` served by root `shellTexts`; `ports.OpenInfoCard{Pawn, ID, Mode, Index, Kind}` accepted by `shellDispatch.Do` (done in GEAR-2).
- Limits: `animal_view_species.go` moved to `game/animalview` (hash-neutral), freeing a file and 42 lines in `application/game`; ceiling stays until integration lowers it.

## GEAR-1-gear-actions
Drop, Eat and Lock in the simulation · 2026-11-06 · 1f8f4b427
- Changed (rows GEAR-004/005/007 sim side; `game/sense` `sense/drop.go`): `DropGear` modes 0 pack entry ID, 1 n-th worn garment, 2 weapon, 3 spare. Pack entry, weapon, spare drop at once at the feet. A garment comes off through a forced `job.UnloadPack` (Count `modeApparel`) of `dropWearWait` = 50 ticks; a prisoner's garment at once (no pack or weapon).
- Changed: queue: `tick.go` sends a queued DropGear to `queueOrder`; `execOrder` runs `Gear.Drop` in turn, label "Drop gear"; a plain drop leaves the queue alone; the take-off job keeps later orders waiting.
- Changed: `ConsumeFromPack`: food taken out and eaten in place (`job.Eat` Count 2, Carry, forced; leftovers `BackToPack` or down); a drug: one dose via `job.Ingest` (`doseTicks` 120), forced; stack returns if the job can't start. `SetApparelForced` toggles `craft.Outfit.SetForced`, colonists only.
- Decisions: refusals (exported `Why*`): Drop: not ours, down/dead, nothing there. Eat: prisoner, not ours, down, nothing, too full (food ≥ 90 %, `tabFullAt`), drug policy forbids (`dose.Allowed`), can't be eaten. A diet does not refuse a player's Eat. Refused = no-op, reason in the read model. No threshold drives any of it, so no flap.
- Changed: `PawnView.Gear.NoDrop` = `DropWhy(id, ModeAny, 0)`; `PackItemView.Gear` = Grams, Use, Why (`EatWhy`).
- Limits: root net +10 lines, no Game methods or fields. Colonies that never send the commands are hash-identical across 12 fixtures.
- Open: UI must show `NoDrop`/`Pack[i].Gear.Why` as disabled tips (done GEAR-2); apparel-policy exemption for forced garments is W1-OUT's; a colonist in a mental break is not refused by the sim (`applyOne` guards only move, prioritize, draft); vanilla take-off time varies, ours is a fixed 50 ticks.

## GEAR-1-gear-facts
Gear tab summary numbers; armour slows the wearer · 2026-11-06 · 59bc5ac49
- Changed (GEAR-001/002/009; wiki numbers by ENG-003): `armor.MoveOffset(kind)` in hundredths of cell/s: flak vest −0.12, flak pants −0.12, marine armour −0.25, plate armour −0.8; flak jacket, helmets, clothes 0. Cataphract (−0.5) has no kind here. `SpeedPermille`/`SpeedPct` against `BaseSpeed` 460 (human 4.6 cells/s), round half away from zero: plate −174‰ (−17 %), flak vest −26‰ (−3 %).
- Decisions: `Overall(worn)` is our own formula (no wiki figure): per humanlike part sum the ratings of worn layers covering it, capped 200 % (`Set.Add`), weighted by `body.Def.Coverage` (sums 1000); missing parts still count (describes clothes, not wounds). Flak vest alone = 49 % Sharp. `Region.Names()` lists coverage.
- Changed: new pure package `application/game/gearfacts`: `Capacity` = 75 kg × Manipulation × body size (wiki Carrying capacity), `Grams`, `Moves`/`MovePermille` (summed in cells/s, converted once), `Mods`, `Of` → `gearview.Pawn`. `craft/garment.go` `WornView` fills `Grams` (`item.MassGrams`), `Cover`, `MovePct`. Pack limit 40 kg × Manipulation (`sense/pack.go`) unchanged.
- Changed: MoveSpeed stat sheet gets the gear offset as an Add step after traits/difficulty, before the body's Moving multiplier. `pawnFx.gear` records the offset used; `paceStep` (only reader of MoveSpeed) rebuilds the sheet only when the current offset differs (outfits are written from ~10 places not owned here); cost one map read + 8-slot switch, no allocation.
- Limits: root +10 lines net. Hash changes for any colony/raid with flak vest, flak pants, marine or plate armour (raider kits wear flak); lead re-pins `TestGrownGolden`. Bench: Grown 250 about flat (load noise), raid steady bytes/tick 5701 → 5646, 6 allocs/op both.
- Open: UI is GEAR-2; cataphract unmapped; `Overall` ignores missing parts.

## GEAR-2-gear-tab
The Gear tab as a full equipment screen · 2026-11-06 · ce6e4e5dc
- Changed: new `inspector/gear` package (was in `inspector`, then ~11.1k of 12k lines; now 10,925) behind consumer-owned `Host` (7 methods) and `Facts` (5 methods). The tab shows: Mass carried X / Y kg (warning colour over capacity), Pack X / Y kg, comfortable range, overall armour (sharp, blunt, heat), gear move speed (when ≠ 0), apparel policy; Equipment (weapon with stat tip, spare, shield bar, smokepop count GEAR-012, grenades, kill record); Apparel in layer order (HP %, mass, "locked", "tainted"); Pack ("Name xQty", mass). Garment tooltip: HP, armour, insulation, layer, cover, move offset, use, lock, tattered/ratty, tainted.
- Changed: per-row buttons: Info (`gear_info`, `ports.OpenInfoCard{Pawn, Mode, Index}`), Drop (`DropGear`; Shift sets `Queue`), Lock/Unlock on a colonist's garment (`SetApparelForced`), Eat/Take on pack food and drugs (`ConsumeFromPack`), Unload/Cancel (CSE-038). Refused actions are disabled with the sim's reason (`Gear.NoDrop`, `Pack[i].Gear.Why`); in a mental break every action except Info is refused with our own line (UI only).
- Changed: guests, visitors, allies, hostiles get the tab read-only (Info only). A prisoner has both Gear and Prisoner tabs (GEAR-008): may drop, no lock, no Unload. Rows in a uikit `ScrollArea` (`gear.rows`); buttons only on fully visible rows. Item pane gained Info (`cmd.item.info`, `OpenInfoCard{ID, Kind}`). `orphan_allow.txt`: two `gear_*` lines removed (the five icons are now drawn).
- Limits: presentation only, hash unchanged. Fit test at UI scales 75–200 %; `journey_gear_test.go` through the real loop (Drop, Shift-Drop, Lock, Eat, Info).
- Open: pack animals have no pack in any animal view, so no Gear tab; weapon row shows no HP or mass (`PawnView` lacks them); whether the sim refuses `DropGear` from a broken pawn was not checked.

## GEAR-2-infocard
The read-only Info window · 2026-11-06 · c7fea7af1
- Changed (GEAR-006; UI checklist "Inspect item" 12.2): new `ebiten/infocard`: `Window.Open(ports.OpenInfoCard)` pushes a `uikit.Modal` (id `colony.infocard`), scroll view + Close; Esc and title X close it; the inspector and selection are untouched. `Build` reads `PawnView` and `WorldView.Items` plus pure catalogs. Rows: category, count, mass, value, quality, HP, material; apparel armour/covers/layers/insulation/move speed; food nutrition/raw/shelf life; weapon damage/penetration/range/burst/aim/accuracy/cooldown; original prose per category (`words.go`). Modes follow `command.DropGear` (0 pack entry ID, 1 worn slot, 2 weapon, 3 spare). Shell case builds it; `App` gains no field.
- Decisions: a modal, not a `colony_compose.go` window (kit gives draw order, hit swallowing, Esc, Close); no `infocard_compat.go` since the root package is at its 60-file ceiling; no `Host`. Presentation only, hash unchanged.
- Open: weapon numbers are the base profile (quality tuning not applied; quality shown); pack entries carry no quality or HP in the read model; only trigger is the Gear tab Info button.

## GEAR-3-outfits-sim
Apparel policies, simulation side · 2026-11-06 · 4b37cb9c3
- Changed (OUT-001…005): new `game/outfit` (dressing moved out of `apparel.go` in a hash-identical commit; four consumer-owned roles `Wardrobe`, `Stock`, `Weather`, `Walks`). Policy `Book` shaped like `diet.Book`: `zone.Filter` of allowed kinds, `zone.Filter` of refused materials, `zone.Special` (HP range, quality range, tainted toggle). Defaults (our own): Anything (default, never tainted); Worker (no armour/helmets, HP 50–100 %); Soldier (everything, HP 50–100 %; dressing still scores warmth only, no armour preference); Nudist (utility only). Kinds outside the Apparel branch are never forbidden.
- Changed: a candidate must pass `Policy.Takes` (replaces the unconditional tainted skip). If a worn garment is refused, `shed` removes the first one per check, unlocked slots only; forced slots and slots in 2 h `Cool` are untouched.
- Changed: commands (`applyOutfit`): `SetApparelPolicy` (colonists), `EditApparelPolicy` (all modes), `ClearForcedApparel`. Deleting the default or last policy refused; holders of a deleted policy go to the default.
- Decisions: no join hook; a colonist without an assignment follows the current default; `ApparelPolicyDefault` first pins everyone dressed who is neither raider nor guest (keys of `g.apparel`, includes other maps and caravans) to the old default. Book stays empty (nil `Export`) until edited.
- Save: v14, `saveState.Outfits *outfit.Save` (book, default, assignments), nil at defaults; older saves import nil = Anything for all; `case v < 14` is comment only; `save_schema.txt`, `TestSupportedSaveVersions` updated. Views: `PawnView.Gear.ApparelPolicy`/`ApparelPolicyName`, `WorldView.Gear.Policies`.
- Limits: root −223 lines (`apparel.go` 447 → 212), 10 fewer `Game` methods, no new files/fields. Hash unchanged at default policies (grown/raid pins held at sizes 120,250). No-flap: policy changing every 30 min changes no slot twice within `dressCool`.
- Open: Soldier armour preference needs `craft.Best` scoring; the book is never pruned of dead/left pawns (inert, export sorted).

## GEAR-3-outfits-ui
Apparel policies, Assign window · 2026-11-06 · 7ce3e33d4
- Changed (`ebiten/windows`, OUT-001…005; no sim change, no hash impact): Assign Apparel column: click cycles the policy, drag down paints (`paintOutfit`); columns laid out by `newAssignCols` (Name/Drugs/Medical narrower; Food policies and Manage areas moved to the default-care row). "Clear forced" button per row, disabled with a reason when nothing is forced.
- Changed: editor (`apparel_policy_editor.go`) from the Apparel header: policy list with New/Copy/Delete (disabled for default and last, with reason), rename, "default for new colonists" check, kinds tree + materials list, HP and quality low/high sliders, tainted check. Range sliders keep a draft, send one command on release, hold a sent value 90 frames. `deps.go`: `outfit *outfitState` on `Set`.
- Open: `TestEveryInputActionHasAJourney` still failed for ConsumeFromPack, DropGear, SetApparelForced at the time (GEAR-1 work, since covered by `journey_gear_test.go`).
