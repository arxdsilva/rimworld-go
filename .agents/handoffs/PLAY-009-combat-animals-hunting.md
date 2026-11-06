# Task Handoff

Task ID: PLAY-009
Status: complete (phase 5)

## Summary

Health, wild animals, weapons, combat and hunting: the fourth food route.

- **Health.** Pawns take injury (100 HP), go down at 70 injury, die at 100, and heal over time. Healing is faster asleep in a bed and slower while downed. Downed pawns stand up again at 55 injury.
- **Animals.** Two original species, with their own AI, movement pace and art in four facings:
  - Dustbuck: a skittish grazer that wanders in herds and flees people and predators. 12 meat.
  - Ridgewolf: a predator that hunts dustbucks when hungry and bites colonists within 5 tiles only when hungry with no prey nearby. It retaliates against attackers and drops a downed colonist to look for other prey. 6 meat. A wolf kill leaves half the meat to scavenge.
  - Wildlife spawns away from the start; a dustbuck herd migrates in at dawn when fewer than 4 remain.
- **Weapons.**
  - Profiles: fists, club, rifle, plus the animals' hooves and bite.
  - Pawns wield one weapon. The start kit has a rifle on the ground, and the workbench crafts clubs.
  - "Equip" order; hunters fetch a free rifle on their own.
- **Combat.**
  - Instant deterministic rolls; rifle accuracy falls with distance, and rock, walls and doors block shots.
  - Attack cooldowns plus a short aim warm-up on a new target.
  - Hunt and Attack jobs chase and re-path to moving targets.
  - Drafted pawns attack on right-click ("Shoot ..." / "Melee attack ...") and fire at hostiles in range when idle.
- **Hunting.**
  - Hunt designation: architect Orders tool, animal inspector toggle, or right-click "Hunt ..." order.
  - Hunt work type (needs a ranged weapon).
  - Kills drop meat. Haulers store it, the stove cooks it, and pawns eat it raw if they must.
- **Presentation.**
  - Animal sprites (new `animal` Blender family, fur palette materials) with hunt crosshairs and health bars.
  - Held rifle or club at the body's hand attachment, turned to face the target.
  - Rifle tracers with a muzzle flash, misses flying past, hit puffs and melee swing arcs, drawn after the night veil.
  - Pawn inspector shows health and weapon. Clicking an animal opens its inspector with a Hunt toggle.
  - Red alert banner while an animal attacks a colonist.

## Public Contracts Changed

- New domain packages:
  - `animal`: species, defs, state, `FleeGoal`.
  - `combat/weapon`: defs, `HitChance`, `InRange`, `Distance`.
  - `combat/projectile`: `Trace`, `Prune`.
- `health.Record.Injury`, `CauseInjury`, `MaxHP/DownedInjury/RecoverInjury/HealTicks`, `Record.HP()`.
- Commands, jobs and orders:
  - Command `DesignateHunt`; orders `OrderHunt/OrderAttack/OrderEquip`.
  - Jobs `Hunt/Attack/Equip`. Attack jobs do not reserve their target.
- Read model:
  - `PawnView.Health/Weapon`, `WorldView.Animals/Traces`, `AnimalView`, `TraceView`.
  - `Game.SpawnAnimal`, `Game.Equipped`.
- Start kit: rifle on the ground.
- `NewColony` spawns wildlife; `New` does not, so fixtures stay animal-free.
- Capture modes `-capture-ui pawn|combat|melee`. Captures settle step lerps before the shot.

## Verification

- `go test -race ./...` green; gofmt, vet and staticcheck clean, apart from the 3 known SA4000s in tests.
- Asset validator: 210 sprites, 0 errors.
- New tests:
  - Domain: weapon ranges and falloff; animal rules and flee goal.
  - Game: hunter fetches the rifle and kills a dustbuck (12 meat); hungry wolf bites, drafted rifleman auto-engages and kills it (6 meat); injury downs, heals and kills; walls block sight; combat order menu (hunt disabled without a gun, equip club, drafted melee attack); colony starts with wildlife and a rifle.
  - Adapter: animal inspector hunt toggle; architect Hunt tool.
- Six-day probes on seeds 1-3:
  - Unprovoked wolves never hurt colonists.
  - Wolves thin the herds and leave meat.
  - Migration refills the herds.
  - Marking everything for hunting yields 84 meat on day one with no injuries.
- In-game: `assets/generated/ingame_combat.png`, `ingame_wolf.png`.

## Known Limitations

- No body parts, bleeding, medicine, rescue or doctoring: downed pawns heal where they fall.
- Animals do not need rest; no taming, manhunter packs or corpses (meat drops directly).
- Undrafted colonists do not flee or fight back.
- Shots resolve instantly; tracers are presentation only.
- One shared hostile banner, with no alert list or pause-on-threat.
