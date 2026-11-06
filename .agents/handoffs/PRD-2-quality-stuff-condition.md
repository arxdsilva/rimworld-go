# PRD-2: quality, stuff, condition, inspiration

Date: 2026-10-29. Wave C. Backlog `.cursor/PRODUCTION_GAP_ANALYSIS.md` (section 3.2).

## What changed
- **Quality** (P2, opus; `game/craft/quality.go`, domain `economy/quality`): rolled from the worker's skill at craft completion and at construction. Items: apparel and weapons (grenades not). Buildings: bed, bedroll, animal bed, end table, dresser, table, chair, horseshoe pin. Stacks and `Building` carry Quality (gob-additive).
- **Stuff** (`domain/economy/item/stuff.go`): `StuffOf` permille factors (wiki Materials page), `MadeFrom`, `DefaultStuff`, `StuffFactor`, `Stuffs`, `StackValue`, `Insulation`. A bill's allowed-stuff is the Forbid filter; `Bill.Stuff` pins a material through `ConfigureBill`.
- **Condition** (`domain/economy/item/condition.go`, `craft/condition.go`): outdoor deterioration on an index. Raw resources, metal and stone never deteriorate.
- **Worn apparel** (`craft/garment.go`): `craft.Outfit{On [4]Piece; Forced}`, `Piece{Kind, Stuff, Quality, Wear, Of}`, four slots with Legs = 3, `Bands()` insulation by stuff and quality. Save: `Worn map[id]craft.OutfitSave`; the legacy `Apparel` map is read when `Worn` is nil. Dead pawns: outfit tainted and dropped beside the body; auto-dress skips tainted pieces (a player Wear order still works).
- **Inspiration** (`game/mind/inspiration.go`): creativity only.
- **Read models:** `ItemView` Quality/Stuff/HP/MaxHP/Tainted; `BuildingView.Quality`; `PawnView` Worn/Inspiration/InspirationTicks; `BillView` Stuff/StuffKind/Quality. Mood kinds `RattyApparel`, `TatteredApparel`, `TaintedApparel`.

## Wiki numbers versus original tuning
- Wiki: quality tables and multipliers (PRD-0), stuff factors, ratty -3 / tattered -5, tainted -5/-8/-11/-14 by count, rain x5, water or marsh x3, roofed 0.
- Original tuning: 2 HP/day outdoors (the wiki gives no base); worn apparel loses 1 HP on 40 % of days; inspiration trigger (mood >= 50, Crafting or Construction >= 3, mean time 410 - 4 x mood days, hourly check), 8-day duration, 1-day cooldown.
- Conflict resolved: tattered under 20 % HP (Apparel page), not 25 % (Deterioration page); ratty at 50 % or less.

## UI
Item inspector title from stuff and quality ("Wood spear (normal)"), hit points line, tainted line (`inspector/gear_facts.go`, `cmds_thing.go`); gear tab lists each worn garment with tattered/ratty/tainted flags; pawn status shows the inspiration. `InspirationTicks` is in the view but not drawn.

- Bill material: `inspector/bill_config.go` seeds `BillSettings.Stuff` from `BillView.StuffKind` and sends it with every `ConfigureBill` (copy and paste keep it); a Material picker (Any or one material) shows when every ingredient is a material.

## Hysteresis
Mood never ends an inspiration; trigger has a 1-day cooldown (`inspiration_test.go`).

## Tests and numbers
`game/quality_condition_test.go`, `craft/{quality,condition,garment}_test.go`, `mind/inspiration_test.go`, `item/{stuff,condition}_test.go`; journey `TestJourneyCraftedItemShowsQuality`. Coverage: craft 98.4 %, mind 96.9 %, item 89.5 %, recipe 97.1 %. Game package 32 lines smaller.

## Gaps and follow-ups
- Wielded weapons store only a kind: crafted quality and stuff are lost on equip; melee weapons are not stuff-scaled.
- No shelves; no stockpile or bill quality/HP ranges (PRD-4).
- Other inspiration types (checklist 10.2); show `InspirationTicks`.
- Colony `Wealth()` could reuse `item.StackValue`.
- Art and sculptures (PRD-3) will roll quality once they exist.
