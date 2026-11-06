# NST-C: Architect category, cards and inspector for Neat Storage (WP-C)

## Done
- `architect/catalog.go`: category `storage` (`CatStorage`, after Furniture; 13 categories, 7 rows of 2 fit `hudlayout.ArchCatsH`). 27 cards (`Build` = build name, `Icon` = slug, `Desc` from the doc's card text, "Q/E rotates." on the 18 rotatable pieces). Shelf, small shelf, egg box, bookcase and small bookcase moved into it (from Furniture and Misc). Locks are the existing `Prog.Locks` path; no new code.
- `inspector`: the Storage button comes from "a storage zone names this building" (`isStorage`), not a kind list. Link and Unlink are offered to the shelf and every Neat piece whose zone is a shelf zone; the pick lists only zones of the same profile. The pane line is `Stored: n/m stacks`.
- Filter window (`windows/storage.go`) and the zone readout (`architect/zone_select.go`) call `game.ProfileFilterRows` when `ZoneView.Profile != ""`.
- Tests: `architect/neat_test.go` `TestEveryNeatStorageRowHasACard`; `ebiten/journey_neat_test.go` `TestJourneyNeatStorage` (Storage category, crate card placed, built, hauler fills it, pane `n/12 stacks`, Storage button opens the filter window) and `TestJourneyNeatAdmire` (a colonist admires a display case holding a stack).
- Tests updated for the move: `architect/catalog_test.go` (`TestArchCategoriesHaveTools` category list), `journey_ax2_test.go` (small shelf, egg box), `journey_ax7_test.go` (bookcase) now open `storage`. `.cursor/ARCHITECT_GAP_ANALYSIS.md`: the five rows' category cell is `storage` (`TestEveryGapRowHasACard` parses it).

## Seen in the real renderer (temporary `-capture-ui nst` case, reverted)
All 27 placed and stocked: display case, weapon rack, 2x2 pallet, meat hook, dump spot, crates draw as designed. Findings: closed crates, Safe and Drum show no fill state in the world; the dump spot heap shows one skeleton plus two brown rectangles (bodies were bare spawned corpse stacks, so may be a test artefact) inside a heavy black outline; count labels crowd 6-slot pieces at zoom 0.8.

## Not done
Safe, Drum, racks, textile rack and meat hook show as locked cards (research gates) as intended. No permanent capture case: root has no room and capture files were not mine.
