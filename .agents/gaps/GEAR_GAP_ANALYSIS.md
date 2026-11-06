# Gear tab and outfits gap analysis (GEAR-*, OUT-*)

Scope: the colonist Gear tab (what a pawn wears, wields and carries, and what the player can do from there) and apparel policies (outfits) on the Assign window. Vanilla core only. Load only for GEAR-* or OUT-* tasks. Phase handoffs are sections of `.agents/handoffs/GEAR.md`. Sister documents: `TEMPERATURE_GAP_ANALYSIS.md` (insulation, layers), `COMBAT_GAP_ANALYSIS.md` and `DEFENCE_GAP_ANALYSIS.md` (armour, weapons), `PRODUCTION_GAP_ANALYSIS.md` (quality, item HP).

## Decision record

| Item | Decision |
|---|---|
| Goal | The tab shows what vanilla's shows, with our own layout and text. Every row can be acted on from the tab. Outfits decide what auto-dress may wear. |
| Numbers | ENG-003 extended to GEAR-* and OUT-* (2026-11-06, user decision). Names and prose stay original. |
| Outfits in scope | Yes (user, 2026-11-06), although vanilla puts them on the Assign tab, not the Gear tab. |
| Default policy | Anything. A colony that never opens the editor dresses exactly as before. |
| Drop vs Unload | Both stay. Drop puts the item at the pawn's feet now, as vanilla does. Unload (CSE-038, Common Sense behaviour) hauls it to a stockpile. |

## Sources

| Page | Used for |
|---|---|
| [Gear](https://rimworldwiki.com/wiki/Gear) | Contents (worn, equipped, carried; hauled items excluded), drop and eat from the tab, Shift queues drops, the two summary stats (mass carried, comfortable range) |
| [Apparel](https://rimworldwiki.com/wiki/Apparel) | Layers, armour resolution, durability, quality table, tainted, forced wear, nudity |
| [Armor](https://rimworldwiki.com/wiki/Armor) | Move-speed offsets per armour piece |
| [Carrying capacity](https://rimworldwiki.com/wiki/Carrying_capacity) | 75 × Manipulation × Body size |
| [Mass](https://rimworldwiki.com/wiki/Mass) | Apparel masses |

**Could not be read:** Inventory (404). The installed game's key names (`ITabs.xml`: mass carried, comfortable range, overall armour, equipment, inventory, forced apparel, drop, consume, worn by a corpse) confirm the tab's sections. No text from them enters the repo.

**Not on any page read:** how an "overall armour" total is formed (the Armor page resolves per body part only); how several move penalties combine; the default outfit set.

## Rows

State: `open` owed to a later change; `kept` differs on purpose; `matches` already right. The phase is in brackets.

| Id | Gap | Wiki | Today | State |
|---|---|---|---|---|
| GEAR-001 | Mass carried | Mass of gear and inventory against carrying capacity, 75 × Manipulation × Body size | Pack only, `inspector/gear_rows.go:21`; pack limit 40 kg × Manipulation, `game/sense/pack.go:22,58` | done [GEAR-1, GEAR-2] |
| GEAR-002 | Overall armour | Shown on the tab (key names); no published formula | Per garment only, `gear_facts.go:85` | done [GEAR-1, GEAR-2] |
| GEAR-003 | Row contents | Icon, label with quality, HP, mass, tainted mark | Text; ratty or tattered tag only | done [GEAR-1, GEAR-2] |
| GEAR-004 | Drop now | Drop from the tab; equipped and worn items take time; Shift queues | Unload hauls away, gated by a habit, `gear_rows.go:39` | done [GEAR-1, GEAR-2] |
| GEAR-005 | Eat or take from inventory | "use (eat) food from the tab" | Automatic only, `sense/gear.go:75` | done [GEAR-1, GEAR-2] |
| GEAR-006 | Info card | Per item (the installed game's tab) | None anywhere | done [GEAR-1, GEAR-2] |
| GEAR-007 | Forced garments | Forcing overrides the apparel policy | "(you chose this)", set only by Wear, `orders.go:434` | done [GEAR-1, GEAR-2] |
| GEAR-008 | Who has the tab | Colonists, prisoners, pack animals, guests | Prisoner replaces Gear, `cmds_pawn.go:364`; no tab on animals | partial [GEAR-2]: prisoners (drop) and guests (read only) done; pack animals have no pack in their view yet |
| GEAR-009 | Armour slows | Flak vest −0.12, flak pants −0.12, marine −0.25, cataphract −0.5, plate −0.8 cells/s | No gear term in speed, `game/skills.go:245` | done [GEAR-1, GEAR-2] |
| GEAR-010 | Coverage shown | Layers and body parts | Not drawn (UI checklist 760) | done [GEAR-1, GEAR-2] |
| GEAR-011 | Wear | Equip or wear from the item | Item pane, `item_inspector.go:150` | matches (UI 776 by the item pane) |
| GEAR-012 | Smokepop count | | Not on the lines, `game.go:204` | done [GEAR-1, GEAR-2] |
| GEAR-013 | Overflow | All rows reachable | Rows past the panel are dropped, `gear_ui.go:129` | done [GEAR-1, GEAR-2] |
| GEAR-101 | Quality factors | Armour 0.6–1.8, insulation 0.8–1.8 | `quality/quality.go:154-155` | matches |
| GEAR-102 | Ratty and tattered | 20–50 % −3, below 20 % −5 | `item/condition.go:116`, `mood.go:266` | matches |
| GEAR-103 | Tainted | −5 / −8 / −11 / −14 | `craft/garment.go:174` | matches |
| GEAR-104 | Layer order | Skin, middle, outer, belt, head | `temp_ui.go:153` | matches |

### Outfits

| Id | Gap | Vanilla | Today | State |
|---|---|---|---|---|
| OUT-001 | Policies | Named; new, rename, copy and delete; four starting policies | None; dressing scores warmth only, `game/apparel.go:157` | done [GEAR-3] |
| OUT-002 | Filter | Kinds, materials, HP range, quality range, tainted | `zone.Filter` and `zone.Special` can express all of it, `building/zone/filter.go:20`, `special.go:13` | done [GEAR-3] |
| OUT-003 | Assignment | Per colonist, default for new ones | Assign has drug, care and food columns, `windows/assign.go:137` | done [GEAR-3] |
| OUT-004 | Auto-dress keeps to it | Disallowed garments come off; forced ones stay | No policy | done [GEAR-3] |
| OUT-005 | Clear forced | From the Assign row | None | done [GEAR-3] |

## Proposed phases

- **GEAR-1, simulation.** `DropGear`, `ConsumeFromPack` and `SetApparelForced` in `game/sense`. Gear mass, capacity, overall armour and move offsets in `game/gearfacts` and `domain/combat/armor`.
- **GEAR-2, the tab.** A new `inspector/gear` sub-package (the inspector package is at 93 % of its line ceiling): summary lines, sections, rows with icon, HP, mass and buttons, tooltips, scroll, and the tab on prisoners, guests and pack animals. A read-only Info window opened by `ports.OpenInfoCard`.
- **GEAR-3, outfits.** `game/outfit` (dressing moved out of the root package first, hash-identical), the policy book, save v14, and the Assign column with the editor.

## Checklist cross-reference

UI: 760 Worn apparel `[-]`, 766 Inventory `[ ]`, 768 Mass/capacity `[ ]`, 776 Wear `[ ]`, 777 Consume `[ ]`, 778 Inspect item `[ ]`, 785 Insulation `[-]` (the note is stale: per-garment insulation is on the rows), 786 Movement penalties `[ ]`, 997 Apparel policy `[ ]`, 1020 Allowed apparel `[ ]`, 1977 Apparel `[ ]`, 2108 Gear `[ ]`. Vanilla: 199 Pawn inspect panel (stale note), 1343 Apparel policy `[-]`, 2122 Apparel policy `[ ]`, 2494 Gear `[ ]`.
