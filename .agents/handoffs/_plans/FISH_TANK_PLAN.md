# Fish tanks and the MAP-5 fixes: gap analysis and plan (FISH-*, MAP-5, STK-AL 2)

Requested 2026-11-02: fix how things that drop on the map are allowed, close the four MAP-4 leftovers the vanilla way, and add fish farming in tanks "the way RimWorld would do it". Load only for this work.

## 1. What vanilla does

Sources read 2026-11-02: wiki Door, Mini-turret, Endings, Hydroponics basin, Fishing zone, Corpse. The wiki's Forbidden and Visitors pages return 404, so those rows say "game behaviour" and rest on how the game plays, not on a published page.

| Topic | Vanilla | Source |
|---|---|---|
| Death drops | A body and its gear start forbidden outside the home area, allowed inside. A hunter's kill is the hunter's to carry. | game behaviour |
| A wild predator's kill | No exception: it is a death like any other. A feeding predator holds the carcass while it eats. | game behaviour |
| Enemy doors | "Doors owned by an enemy must be claimed (which requires defeating said enemy) in order for colonists to open them." Raiders treat a colony's closed door like a wall. | wiki: Door |
| Enemy turret out of ammo | A mini-turret needs a reload after 60 rounds, by a hauler. Nothing says an enemy reloads one, and enemies don't do hauling work. A spent one stays silent. | wiki: Mini-turret; game behaviour |
| Visitors when their faction turns hostile | They stop visiting and leave the map, defending themselves, and count as enemies on the way out. | game behaviour |
| New wanderers | After a game over: "the option to create new wanderers to start the colony again after eight in-game hours have passed, using a menu similar to the game creation menu. After seven new groups have been created, the option will no longer be available." | wiki: Endings |
| Fish | Odyssey has fish and a fishing zone worked by the Fish job on the Animals skill. Nothing in vanilla farms fish; tanks are ours. | wiki: Fishing zone |
| Hydroponics basin, the model for a tank | 1x4, 70 W day and night, research-gated, Construction 4, 100 steel + 1 component, 2,800 work. "The plants will die if power is cut", quickly. | wiki: Hydroponics basin |

## 2. Gaps

| id | Vanilla | Ours | Fix |
|---|---|---|---|
| AL-1 | A wild predator's kill follows the home-area rule | `forbid.go` `killTaken` is true for any animal killer, so it stays allowed outside Home and joins the job list | `killTaken` only for a colonist hunting that prey or a tame animal on its hunt |
| AL-2 | A feeding predator holds its carcass | Only a tame feeder holds it (`feedOnKill`, `syncFeedClaim`) | A wild feeder holds it too, in Home as well, so nobody is sent for it |
| AL-3 | A job whose target is gone ends at once | A colonist walks to the spot of a carcass already eaten | The job fails when its target is gone (red test first) |
| AL-4 | Every drop has a rule | Only deaths were audited | Audit every stack-creating site, table in `ALLOW_GAP_ANALYSIS.md` |
| MAP-17 | Enemy doors shut to colonists until claimed; claiming needs the enemy defeated | `raids.go` `Settle` holds them open, so colonists walk in | Shut doors with `Building.Hostile`; defenders open them; colonists blocked, can attack them, can claim them once no defender stands |
| MAP-18 | Visitors leave and count as enemies | They carry on visiting | Leave, hostile, fight back; trade closes; a letter says so |
| MAP-19 | New wanderers after 8 hours, at most 7 groups | Missing | Option on the game-over screen and an alert, then a pawn picker |
| MAP-20 | Spent enemy turret stays silent | Same | Keep, record the check, and say it in the inspector |
| FISH-* | See §3 | Nothing | §3 |

## 3. Fish tanks

The building behaves like a hydroponics basin, the work behaves like the Fish job, and the items behave like meat.

**Building.** `construction.FishTank`, "fish tank", 1x4, faced, 70 W day and night, Construction 4, 100 steel + 1 component, 2,800 work, as the basin (wiki numbers, ENG-003: FOOD). Research `research.Aquaculture`: Industrial, needs Hydroponics, 900 (original).

**Stock.** The player picks a stock in the inspector, as a basin's crop: `aqua.Species`, all original tuning:

| Stock | Days to harvest | Fish per harvest | Comfort band °C | Animals skill |
|---|---|---|---|---|
| Tilapia | 6 | 48 | 15..35 | 0 |
| Carp | 8 | 72 | 2..30 | 4 |
| Trout | 10 | 110 | 0..20 | 8 |

**Feed.** Like a turret's or a generator's fuel: hay, kibble or raw plant food, hauled by the existing refuel work. Empty stops growth and starts the stock starving. Refuel asks at a quarter and fills to full, so it can't flap.

**Health.** 0..100. Power out or air outside the band or no feed costs health; good conditions restore it slowly. Power out is the fastest, about a day from full to dead (the basin's "very quickly"). At 0 the stock dies and a message says why. Growth pauses while health is falling. Repeated power blips can't flap anything: no actor reacts to health, and a job is only offered for a stocked or ripe tank.

**Work.** `work.Handle` on the Animals skill, as the Fish job: `job.StockTank` fills an empty tank that has a stock chosen and power, `job.NetTank` nets a ripe one. A ripe tank waits; health still applies. Netting drops `item.Fish` beside the tank, allowed. Skill scales yield: unskilled fumbles part of it (original). The tank restocks by itself.

**Item.** `item.Fish`: raw meat in every way (`IsMeat`, nutrition 500 on the current scale, mass, rot and storage class as Meat). No fishing zone: Odyssey content, out of scope.

**State.** `game/aqua` sub-package (SOLID §10) with consumer-owned roles, one `*aqua.Tanks` per map on `mapState`, stepped hourly over its own list, so no scan. Saved, nil while no tank exists, so goldens hold. Pure rules in `internal/domain/building/aqua`.

**UI.** Architect production card after hydroponics, gated by research. Inspector: stock picker, growth, health and why it is falling, feed bar, power, and Harvest when ripe shows as a work state. Read model `BuildingView.Tank *TankView`. Render: tank sprite by facing, a water glow from `DrawPowerParts` like the basin, and fish visibly bigger per growth stage; no outlines are needed on fish but follow ART. Item icon for fish. Alert when a stock is dying.

## 4. Parallel workers

Areas: domain, application, renderer/UI, art, docs. **Contract, lead alone:** `command.SetTankStock`, `command.NewWanderers`, routed through `tick.go`; this file.

RAM: `make agent-budget` on 2026-11-02 allows 15 light agents, 9 heavy. Seven at once, tested with `GOFLAGS=-p=2`.

| id | model, reason | job | owns | forbidden | tests |
|---|---|---|---|---|---|
| P | opus: hash-proved extraction | Move at least 500 lines and one file out of package game | a cohesive chunk it picks, none of the files below | everything others own | `go test -race -count=1 ./internal/application/game/...` targeted, arch, hash |
| A | opus: jobs, reservations | AL-1..4 | `forbid.go`, `allow/*`, `animals.go`, `animal_needs.go`, `wildlife.go`, `animal_life.go`, `haul/*` | `raids.go`, `defense.go`, `fences.go` | new tests `-run` |
| B | opus: pathing | MAP-17, MAP-20 | `combat/*`, `defense.go`, `fences.go`, `bridge.go`, `orders/*`, `orders.go`, `raids.go`, `raid/*`, `regions.go`, `inspector/cmds_build.go`, `cmds_defence.go` | `visit/*`, `animals.go` | new tests `-run` |
| C | opus: state machine | MAP-18, MAP-19 simulation | `visit/*`, `visitors.go`, `trade_caravan.go`, `trade.go`, `world_stash.go`, `host_commands.go` (its case), `axview/*` | `raids.go`, `combat/*` | new tests `-run` |
| D | opus: saves, jobs | FISH simulation | `domain/building/aqua/*`, `game/aqua/*`, `construction.go`, `item/*`, `research.go`, `job.go`, `power.go`, `save.go`, `game.go`, `building_view.go`, `fuel.go`, `host_commands.go` (its case) | `visit/*`, `combat/*` | new tests `-run` |
| E | sonnet: UI on a contract | FISH and wanderers UI | `inspector/*` except C's two files, `architect/*`, `hud/*`, `render/world/*`, `wizard/*`, new `dialogs` or root compat, journeys `journey_fish_test.go`, `journey_wanderers_test.go` | `internal/application/**`, `assets/**` | targeted, no full ebiten package |
| F | opus: art | Tank and fish sprites, skill `rimworld-go-art-assets` | `assets/source/**`, generated output, metadata | Go code, palettes (ask the lead) | `make art FAMILIES=…`, `make art-check` |

Integration: the lead runs gates (build, fmt, vet, arch, full `-race`, determinism without `-race`, `make art-check`), re-pins goldens with a reason if behaviour moved them, updates checklists and handoffs, and makes the commits, then the tag and push the user asked for.
