# AX-5: fire foam and security (agent A5)

This handoff covers the AX programme's wave 1 foam and security rows. Under ENG-003 the numbers come from the RimWorld wiki and the names are original. The brief is the lead's `ax-brief.md`; the contract is `AX-0-contract.md`.

## What works
- **Foam** (`explosion.Foam`, combat/foam.go `Blasts.smother`): a foam burst puts out every fire it reaches (in sight of its centre), leaves `stain.Foam` (600 of 1000 per cell) and harms nobody.
  - Nothing burns on a foamed cell (game fire.go `cellFuel`) until the foam is cleaned up or washed away. Foam counts as filth, so colonists clean it in the home area and rain and snow wash it off.
- **Foam popper**: every 30 ticks it checks for fire within 3.9 tiles (wiki: Firefoam popper, 3 tiles). When it finds one, it sparks for 150 ticks, then bursts over 9.9 tiles.
  - Afterwards it is spent and a blueprint is laid in its place. This is the wiki's one-use device with its auto-rebuild option, so a constructor re-arms it by building it again.
  - A smashed popper bursts at once.
- **Foam charge**: a buried charge made of 2 foam shells (Only). It bursts in foam over 9.9 tiles (wiki: IED firefoam trap).
- **Warhead charge**: a buried charge made of 1 antimatter warhead (Only). It goes off as the warhead: 550 bomb at the centre, falling to 110 at the 14.9-tile edge, 130 % AP, and about 22 % of its cells catch fire (`explosion.Def.Rim`, `Def.Fire`; wiki: Antigrain warhead).
  - A loose warhead cooks off in a fire or blast (`cookOff`).
  - The wiki says the warhead is never crafted or sold; it comes from quest rewards and ancient shrines. This game has no quests, so vaults hold one as a rare find on 2 % of rolls (mapgen `vaultWarhead`).
- **Foam turret** (80 W, range 19.9): on its scan tick it looks for the fires it can see. Each spray costs one of its 10 charges and puts 41 blobs, 2 ticks apart, on those fires, nearest first and in turn; each blob is a 1.9-tile foam burst (projectile flight at 27 cells/s). It then waits 3600 ticks.
  - A new turret is full. At 2 charges or fewer a colonist hauls chemfuel: 7.2 a charge rounded up, /9.
  - A smashed foam turret bursts damageless foam over 4 tiles.
- **Rocket battery** (2x2, 150 W, range 29.9): after a 60-tick aim it fires 12 rockets, 10 ticks apart: 24 bomb, 36 % AP, 2.9-tile blast, 5.75 forced miss, 38 cells/s.
  - It fires from a full load only. A colonist reloads all 180 chemfuel (20 units) at once.
  - A smashed battery blows up (50 bomb over 6.9 tiles) on the turret roll.
- **Cooldown decision**: the wiki gives 210 ticks in the infobox and 7200 in the text. I chose the text's 7200-tick recharge, stored as `weapon.RocketRecharge` in `SwarmRocket.Cooldown` and in `Arm.Cool`. The recharge is the rule a player meets, and the 210-tick gun cooldown never matters for a weapon that empties itself in one volley.
- **Foam shells**: a machining-table recipe (`recipe.MakeFoamShells`: 3 steel, 80 work), unlocked by Fire foam. Mortars load them last (`Mortar.Foam`) and lob 5-tile foam bursts (`weapon.MortarFoam`).
- **Warhead as mortar ammo** (wiki: Antigrain warhead, a mortar shell): `weapon.Warhead` is a mortar row with the charge's blast, and `item.IsShell` and `weapon.ForShell` include the warhead. A mortar fires warheads after the ordinary shells and before foam (`Mortar.Warhead`). Rearm hauls fetch warheads last, only when nothing else is in stock; they keep the mortar's sound.
- **Burning creatures**: creatures here burn only by standing in a fire cell, so foam that douses the cell puts them out; a foam turret aiming at fire cells aims at burning pawns too.
- **Range checks are now explicit sets**: `construction.Kind.IsCharge` (combat `isIED`, raids.go `sapperTraps`), `item.IsShell`, `weapon.ForShell`, and the weapon `ToolsOf` carried range (`lastCarried`).
- **AI**: sappers step around the new charges they know of, and raiders target the new turrets (`Turrets.ForRaider` covers every turret). Colonists rearm and refuel through the existing rearm haul.

## UI and art
- **Architect**: the cards `ied_foam`, `ied_warhead`, `foam_turret` and `rocket_battery` (Security) and `foam_popper` (Misc) replace the placeholders, using AR2's icons.
- **World art (AR2)**: `defense/foam_popper`, `ied_foam`, `ied_warhead`, and `foam_turret_base` and `rocket_battery_base` with their 16 gun frames (gun_keys.go). Glow comes from the shared power look.
  - The projectiles `effect/rocket_projectile` and `effect/foam_projectile` are drawn rotated to their heading (combat_draw.go `drawProjectileSprite`, with `projectileKind` in combat_overlay_host.go).
  - Foam stains use the `overlay/foam_0..2` decals (fire_world_draw.go).
  - The foam burst flash uses `effect/smoke_cloud`, because AR2 drew no foam burst.
- **Inspector** (inspector/ax5.go): foam charges and rockets out of their full load, with refill notes, and "Armed" or "Sparking" on the popper and every charge. Hold fire also covers both new turrets. The read model is `BuildingView.AX.Charges`, `ChargeCap` and `Armed`, filled from `combat.Facts`.
- **Sounds**: existing ones only. The foam turret uses `sfx.smoke_launcher`, the rocket battery `sfx.rocket_launch`, foam bursts `sfx.smokepop_burst`, and charges the arming click. No new sound was generated.

## Files
- **Domain**: explosion.go, weapon.go and tool.go (kinds MortarFoam, FoamSpray, SwarmRocket and Warhead, appended), item.go IsShell, the research.go Fire foam unlock, the recipe.go append, construction/ax_a5.go (plannedA5 is now always false; `IsCharge`, `BurstsWhenDestroyed`), mapgen vaults.go, and fire/roll.go (new).
- **Combat**: foam.go (new), deps.go (role `Foam`), turret.go, ied.go, blast.go, combat.go, orders.go, rearm.go, mortar.go and AGENTS.md.
- **Game**: fire.go (the `combatFoam` adapter and the foam check), lightning.go, combat_host.go, raids.go, defense.go and building_view.go. The net line change in the game package is -1: I moved fireMix and spreadRoll to `fire.Mix` and `fire.SpreadRoll`.
- **Adapters**: build_keys.go, gun_keys.go, turret_draw.go, blast_draw.go, creature/burst.go, cue combat_sounds.go and structure_sounds.go, blood_draw.go, fire_world_draw.go, combat_draw.go, combat_overlay_host.go, inspector ax5.go and cmds_defence.go, and architect catalog.go and planned.go.

## Tests
- combat/foam_test.go: the burst, both charges, the popper's no-flap, the foam turret's spray and cooldown no-flap, refuelling, the rocket volley and recharge, smashed defences, and foam mortar shells.
- game/ax5_test.go: the kinds are live; the foam turret puts out a fire, leaves foam that nothing burns under, and spends a charge; a popper bursts and is rebuilt; the warhead charge; the rocket battery volley.
- journey_ax5_test.go: placement from the Architect, a popper smothering a fire, a colonist building a foam charge from foam shells, the pane lines, and hold fire.
- vaults_test.go now allows the warhead.
- The determinism gate passes with no golden change.

## Open
- **Rocket battery**: vanilla has a colonist activate it manually and spreads its rockets over target tiles; here it fires on its own like the other turrets.
- **Foam popper**: there is no manual trigger, and the HP ≤ 8 rule for bursting when destroyed is not modelled; it always bursts.
- **Foam coating**: the "covered in foam" coating on pawns is not modelled. Pawns burn only by standing in fire, and foam puts that fire out.
- **Warhead**: quest rewards wait for a quest system.
- **Lead**: journey_arch_test.go `TestJourneyArchitectVanillaCategoriesAndPlaceholders` (the lead is relaxing it).
