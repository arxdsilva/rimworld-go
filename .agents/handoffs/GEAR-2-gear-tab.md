# GEAR-2: the Gear tab as a full equipment screen

The colonist inspector's Gear tab was a list of text lines with one Unload button. It now shows the same information as vanilla's Gear tab: a summary, the weapon and spare, apparel in layer order, and the pack. Each item row has a sprite and its own buttons. The layout and all wording are our own. Presentation only: the state hash is unchanged.

## What was wrong
- `inspector` was at about 11.1k of its 12k-line limit, so the tab had no room to grow there.
- Rows had no icons and no per-item actions except Unload. Garment details (armour, insulation, layer, cover, move offset) were only in a separate block of stat lines.
- Prisoners lost the Gear tab to the Prisoner tab, so their gear could not be seen or dropped (GEAR-008).
- The five `ui_icon/gear_*` icons sat in `orphan_allow.txt` because nothing drew them.

## What changed
- New package `inspector/gear` (host.go, facts.go, rows.go, tab.go, paint.go). It depends on two consumer-owned interfaces:
  - `Host` (7 methods): Enqueue, Do, Kit, Image, ItemImage, Shift, Unloading.
  - `Facts` (5 methods): Temp, LayerOrder, LayerTag, Insulation, Note.

  `inspector/gear_ui.go` is now just `gearHost` over the pane plus `gearTabBody`. The old `gear_rows.go` and `gear_facts.go` and their tests moved into the package. `inspector` went from 11,059 to 10,925 lines.
- The summary shows, in order:
  - "Mass carried X / Y kg" (warning colour when over capacity);
  - Pack X / Y kg;
  - the comfortable range;
  - overall armour (sharp, blunt, heat);
  - move speed from gear, when not 0;
  - the apparel policy.
- **Equipment section:** the weapon (its tip carries the stat lines), the spare, the shield bar, the smokepop count (GEAR-012), grenades and the kill record.
- **Apparel section:** in layer order, with HP %, mass, "locked" and a "tainted" mark in the warning colour.
- **Pack section:** "Name xQty" and mass.
- A garment's tooltip lists hit points, armour, insulation, layer, cover, move offset, what it is for, the lock, and tattered/ratty and tainted.
- Buttons on each item row, right to left:
  - Info (`ui_icon/gear_info`) sends `ports.OpenInfoCard{Pawn, Mode, Index}`.
  - Drop (`gear_drop`) sends `DropGear`; Shift sets `Queue`.
  - Lock or Unlock on a colonist's garment (`gear_lock` / `gear_unlock`) sends `SetApparelForced`; the button shows selected while the garment is locked.
  - Eat or Take on pack food and drugs (`gear_eat`) sends `ConsumeFromPack`.
  - Unload / Cancel (CSE-038) shows while that habit is on.
- When an action is refused, its button is disabled and the tooltip gives the simulation's reason (`Gear.NoDrop`, `Pack[i].Gear.Why`). During a mental break, every action except Info is refused with our own line.
- Who can act:
  - Guests, visitors, allies and hostiles get the tab read-only: Info only.
  - A prisoner has both the Gear and Prisoner tabs. It may drop, but has no lock and no Unload.
- Rows sit in a uikit `ScrollArea` (`gear.rows`). Buttons are declared only on rows that are fully visible.
- The item pane gained an Info button (`cmd.item.info`) that sends `OpenInfoCard{ID, Kind}`.
- `orphan_allow.txt`: removed the two `gear_*` lines.
- Root tests:
  - `journey_unload`, `journey_wave3` and `journey_wave_c` now point at `gear.UnloadID`, `gear.WeaponTitle` and `gear.StatLines`.
  - The prisoner tests in `prisoner_ui_test.go` and `pawn_pane_test.go` now assert that a prisoner has both tabs.

## Tests
- `inspector/gear/gear_test.go`:
  - the moved title, tip and stat-line tests;
  - the order of the summary and sections;
  - garment tips;
  - each button's command, including Shift queueing and the Info intent;
  - disabled reasons (NoDrop, the drug policy, a mental break);
  - guest and prisoner access;
  - a fit test at UI scales 75–200 %. Every widget stays inside the panel, the last pack row can be scrolled into reach, and labels keep at least 120 px beside the buttons.
- `journey_gear_test.go`, run through the real loop:
  - Drop on the weapon puts the knife on the ground at the colonist's feet.
  - Shift-Drop on a garment sends a queued `DropGear`, and the garment comes off and lies at its feet.
  - Lock sets `ApparelForced`, and a second click clears it.
  - Eat: the colonist packs a spare meal (CSE-005); then `ConsumeFromPack` empties the pack and the food bar rises.
  - Info opens the Info window and the colonist stays selected.
- `inspector/gear_ui_test.go`: the item pane's Info button sends the intent.

## Gaps
- Pack animals: no animal view exposes a pack, so animals get no Gear tab and no simulation field was added.
- The weapon row shows no HP or mass, because `PawnView` doesn't carry the weapon's condition or weight.
- The mental-break refusal exists only in the UI. Whether the simulation also refuses `DropGear` and the other commands from a broken pawn was not checked here.
