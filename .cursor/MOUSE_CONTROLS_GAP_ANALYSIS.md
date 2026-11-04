# Mouse controls gap analysis (CTL-001..)

Scope: what the mouse does on the map, the bar, the windows and the dialogs. Load only for CTL-* tasks. Siblings: `WORK_PRIORITIES_GAP_ANALYSIS.md` (the Work tab's sim side), `ALLOW_GAP_ANALYSIS.md`.

## Decision record

| Item | Decision |
|---|---|
| Goal | A double click on furniture or an item takes the same type on screen, and any action then applies to all of them (user, 2026-11-04). Everything else the sources list comes with it. |
| Who does not group | Walls, hopper and trees, from the wiki. All other plants, floors, fences and conduits are our rule: they come in rows. A plain click still selects one. |
| Shift mixes kinds | One report only, kept because the user asked for everything. A box still takes one kind. |
| Bar order | Saved in the save file, outside the state hash, empty by default, no `SaveVersion` bump. It never reaches AI or work. |
| Work header | Shift-click gives every capable colonist the next value of the first capable one; Shift-right the previous. The tip does not define the step. |
| Painting | The value of the first click, like the Schedule brush. |
| Box result | Dragging over a base now selects furniture instead of clearing the selection. |

## Sources

- [Controls](https://rimworldwiki.com/wiki/Controls): double click works with colonists, doors, deer and chunks, and not with trees, rocks, walls or the hopper; Shift with left selects several; Shift with right queues; right drag on the bar reorders; Numpad `*` is next in the square; Ctrl ×10, Shift ×100 and both ×1000 on trade, caravan and bills; the middle button rotates.
- [Loading screen](https://rimworldwiki.com/wiki/Loading_screen), the game's own tips: right click dismisses letters and messages; click-drag paints, outfits and food included; Shift-click a work type's name for all colonists; right click a priority lowers it; right click order buttons for forbid-all and cancel-all-hunting.
- [User interface](https://rimworldwiki.com/wiki/User_interface): the pane lists the quantity; on the bar left jumps and right goes to the world map.
- [Menus](https://rimworldwiki.com/wiki/Menus): hovering a wildlife row points an arrow, a click jumps.
- [Growing zone](https://rimworldwiki.com/wiki/Growing_zone): double click an empty tile of a zone highlights the zones in view.
- Player reports ([1](https://steamcommunity.com/app/294100/discussions/0/144513248273566010), [2](https://steamcommunity.com/app/294100/discussions/0/360671727320745886)): blueprints by double click; Shift across types with a "Various" label; double click on a crop takes the zones.
- **Limit:** no source is complete and none was checked in the game. A triple click selecting the whole map has no source, so it stays out.

## Rows

State: `done` shipped; `open` owed to a later change.

| Id | Vanilla | Source | State | Where |
|---|---|---|---|---|
| CTL-001 | Double click takes the same type on screen, doors included. | Wiki | open | `selection`, `multiselect_render.go` |
| CTL-002 | Double click on a blueprint. | Report | open | same |
| CTL-003 | Double click on a zone takes the zones in view. | Wiki | open | same |
| CTL-004 | Double click on a plant in a zone takes the zones. | Report | open | same |
| CTL-005 | Shift adds any groupable kind. | Wiki | open | `selection` |
| CTL-006 | Shift mixes kinds; the pane says "Various". | Report | open | `selection`, `inspector/multiselect.go` |
| CTL-007 | The box takes furniture, blueprints and zones. | Wiki | open | `multiselect_render.go` |
| CTL-008 | The pane shows the quantity for every group. | Wiki | open | `inspector/multiselect.go` |
| CTL-009 | An action applies to every member. | User | open | `inspector/hud_commands.go` |
| CTL-010 | Right click dismisses a message. | Tip | open | `hud/messages.go` |
| CTL-011 | Right drag on the bar reorders it. | Wiki, tip | open | `hud`, `game/roster` |
| CTL-012 | Right click on the bar opens the world on that colonist. | Wiki | open | `hud` |
| CTL-013 | Right click on a Work cell lowers the priority. | Tip | open | `windows/work.go` |
| CTL-014 | Shift-click a work type's name changes all colonists. | Tip | open | `windows/work.go` |
| CTL-015 | Dragging paints Work, Assign and the food policy. | Tip, report | open | `windows/` |
| CTL-016 | Right click on orders: forbid all, cancel all hunting. | Tip | open | `architect` |
| CTL-017 | The middle button rotates the object being placed. | Wiki | open | `hotkeys.go` |
| CTL-018 | Ctrl ×10, Shift ×100, both ×1000 on trade and caravan. | Wiki | open | `uikit`, `dialogs` |
| CTL-019 | Hovering a wildlife row points an arrow at the animal. | Wiki | open | `windows/animals.go` |
| CTL-020 | Escape clears the selection. Bug: a lone animal, item or plant stayed selected. | Memory | open | `app.go` |
| CTL-101 | Right click with several undrafted colonists opens a menu. | Memory | open: no source for how it works | |
| CTL-102 | A colonist's name in Work and Assign jumps to them. Only Animals does. | Memory | open | |
| CTL-103 | A column header sorts the column. | Memory | open (WRK-104) | |

## Already matching

Click and cycling a tile's stack, Numpad `*`, Shift+right queueing, zoom at the cursor, panning with the middle button, edge scrolling, Shift for fast pan, comma and period, the bar's double click, the drafted right-drag line, Alt for cell details, hit chance on hover, clicks on alerts and letters, multipliers on bills, painting the Schedule, and right click cancelling a tool.
