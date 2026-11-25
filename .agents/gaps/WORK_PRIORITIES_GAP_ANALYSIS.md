# Work priorities gap analysis (WRK-001..)

Scope: the Work tab and how a colonist picks work: the tie-break between work types, the order of tasks inside a type, and the tab itself. Vanilla core only, no DLC columns (Childcare, Fish, Dark study). Load only for WRK-* tasks. Sibling: `HARVEST_GAP_ANALYSIS.md`.

## Decision record

| Item | Decision |
|---|---|
| Goal | The tie-break a colonist follows is the vanilla one, and the Work tab shows it: left to right is what the colonist does. |
| Numbers | ENG-003 in `AGENTS.md` is extended to work priorities and harvesting (2026-11-04, user approved): published wiki data is tuning data; names and prose stay original. |
| Haul near the end | Accepted: with everything at 3, piles wait until all other work is done, as in vanilla. The default priorities do not change. |
| Hoppers | Filling them is Haul work only (wiki). A cook with Haul off leaves them empty. |
| Saves | `work.Type` keeps its values, so priorities in a save still mean the same work. Only `work.Order` moves. |

## Sources

- [Work](https://rimworldwiki.com/wiki/Work): the work type table, the tasks of each type, and the rules below. The page is flagged a stub.
- Rules taken from it: in manual mode each type has a priority from 1 to 4, and blank is never; for equal priorities "the tasks on the left are done first"; all work of one priority comes before the next; inside a type "tasks higher on the list will be prioritized".
- **Limit of the source:** the page never says its table is the tab's left-to-right order. It matches what we remember of the game; nobody checked it in the game.
- **Not on the page:** sorting by a column and the priority a newly enabled cell takes. We remember sorting existing in the game: memory, not wiki fact. The game's own tips cover the header click and painting (`MOUSE_CONTROLS_GAP_ANALYSIS.md`).

## Order of work types

| Wiki | Type | Before WRK-001 | Now | Note |
|---|---|---|---|---|
| 1 | Firefight | 1 | 1 |  |
| 2 | Patient | no column | 2 | Patient work; urgent care remains independent of the work schedule (WRK-101). |
| 3 | Doctor | 2 | 3 |  |
| 4 | Bed rest | no column | 4 | BedRest column and legacy rest setting migration (WRK-101). |
| 5 | Basic | no column | 5 | Basic jobs including flick/open (WRK-101). |
| 6 | Warden | 3 | 6 |  |
| 7 | Handle | 9 | 7 |  |
| 8 | Cook | 6 | 8 |  |
| 9 | Hunt | 8 | 9 |  |
| 10 | Construct | 5 | 10 |  |
| 11 | Grow | 10 | 11 |  |
| 12 | Mine | 12 | 12 |  |
| 13 | Plant cut | 11 | 13 |  |
| 14 | Smith | in Craft | 14 | Separate recipe/bench admission (WRK-102). |
| 15 | Tailor | in Craft | 15 | Separate recipe/bench admission (WRK-102). |
| 16 | Art | in Craft | 16 | Independent sculpture admission and Artistic skill (WRK-108). |
| 17 | Craft | 7 | 17 | |
| 18 | Haul | 4 | 18 | |
| 19 | Clean | 14 | 19 | |
| 20 | Research | 13 | 20 | |

## Rows

State: `done` shipped; `open` owed to a later change; `kept` differs from the wiki on purpose.

| Id | Gap | State | Where |
|---|---|---|---|
| WRK-001 | `work.Order` had Haul 4th, Craft before Hunt, Plant cut before Mine, Research before Clean. | done | `domain/ai/work/work.go` |
| WRK-002 | Tab columns followed `work.Type`, while the tie-break followed `work.Order`: the player read Grow before Haul and the colonist did Haul first. Columns now follow `work.Order`; everything else stays indexed by type. | done | `ebiten/windows/work.go` |
| WRK-003 | Delivering to frames was Haul work only. The wiki lists it under Construct and Haul. With Haul near the end, this would stall every building site whenever plants or rock had work left. Construct now delivers, and a forced Supply order takes Haul or Construct. | done | `game/priorities.go`, `game/tick.go` |
| WRK-004 | Construct's inner order: wiki has uninstall, roofs, frames, deliveries, deconstruct, repair, floors. Ours had frames, uninstall, repair, roofs. | done | `game/priorities.go` |
| WRK-005 | Handle's inner order: wiki has pen, slaughter, release, milk, shear, tame, train. Ours had pen, tame, train, collect, slaughter, so a long training queue kept milk waiting. | done | `game/priorities.go`, `game/herd/herd.go` |
| WRK-006 | Clean's inner order: wiki clears snow before filth; ours the reverse. | done | `game/priorities.go` |
| WRK-007 | Haul's inner order: wiki has refuel, load, strip, corpses, hoppers, cremation, general. Ours put hoppers first. Our "urgent" mark (STO-2) stays first; it has no wiki counterpart. | done | `game/priorities.go` |
| WRK-008 | Cook also filled hoppers; the wiki lists that under Haul only. | done | `game/priorities.go` |
| WRK-009 | Tab text: the header did not state the tie-break, and Growing claimed to cut grass, which is Plant cut since FOOD-2. | done | `ebiten/windows/work.go` |
| WRK-010 | Doctor's inner order: wiki has tends, operations, then rescue; ours rescues first. | kept | MED-03's decision |
| WRK-011 | Haul delivers to frames before general hauling; the wiki lists general first. Kept so that a hauler with Construct off still feeds a site before clearing the map. | kept | `game/tick.go` |
| WRK-101 | Patient, Bed rest and Basic columns; preserve existing IDs and legacy priorities/masks. | done | `domain/ai/work/work.go`, `migrate.go`, `game/priorities.go`, native care and Work tab |
| WRK-102 | Smith and Tailor split from Craft; preserve old Craft settings in migration and route by recipe work type. | done | `domain/ai/work/migrate.go`, `domain/economy/recipe/benches.go`, `game/priorities.go` |
| WRK-103 | Simple mode: the manual priorities toggle with plain checks. | done | `windows/work.go`: checkbox editing retains existing numeric preferences; new enables set 3. |
| WRK-104 | Sorting by a column (CTL-103). Changing a whole column from its header moved to CTL-014. | done | `windows/work.go`: stable detached name/priority sort; skill breaks ties, Shift still changes the column. |
| WRK-105 | The sound when assigning a pawn to work it will do poorly (wiki). | open | |
| WRK-106 | Dragging to paint priorities: CTL-015. | done | `ebiten/windows/work.go` |
| WRK-107 | A delivery taken from the Construct slot is a `job.Haul`, so the colonist card names it Hauling and its speed reads as Haul work. Cosmetic. | open | `game/bio_view.go` |
| WRK-108 | Separate Art from Craft, append stable IDs and migrate historical priority rows. | done | `work`, `recipe.Def.WorkType`, `game/priorities.go`, `saveutil`, Work tab and Dossier. |

## Already matching

Priorities 1 to 4 with blank as never; all of one priority before the next; skill level and passion in each cell; incapable cells hatched with the reason; a forced order needs the work type on; shift queues orders (`order_queue.go`).
