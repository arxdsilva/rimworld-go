# Shooting gap analysis (SHT-001..)

Scope: how a colonist's shot is decided and shown: accuracy inputs, aim time, the people in the way, and the numbers the player sees. Vanilla core only. Load only for SHT-* tasks. The rest of combat (damage, armor, AI) is in `COMBAT_GAP_ANALYSIS.md`; weapons, turrets and traps are in `DEFENCE_GAP_ANALYSIS.md`.

## Decision record

| Item | Decision |
|---|---|
| Goal | A shot's chance comes from the wiki's inputs, combined the wiki's way, and the player can read why. |
| Numbers | ENG-003 extended to shooting (2026-11-05, user approved). The tables below are published wiki data. |
| Names | `DeliberateAim` and `QuickDraw` stay: they are the original-named counterparts of Careful shooter and Trigger-happy. Their numbers change, not their names. New traits for the Brawler and Nimble effects get original names too. |
| Light | Not a gap. The pages read give no core light rule for shooting; the wiki mentions only an Ideology precept. `attack.NoLightPenalty` stays. |
| Cooldown | Not a gap. Aiming Time says "the cooldown time of the weapon is unaffected", and the only published cooldown factor is a Biotech item. |

## How we can use it

- **Hurt shooters matter.** An arm wound changes who stands on the line, with no new system: one more term in the score.
- **Traits become a choice.** Today `DeliberateAim` is a flat +12 points, strongest where it matters least. On the curve it pays at long range, which gives each trait a place to stand.
- **The hit card can explain itself.** `attack.Report` already holds every number. Showing them all (ours goes beyond the wiki, which publishes no tooltip) teaches range and cover without a tutorial.
- **Shooting past your own people.** The distance rule makes close formation safe and a long line of fire risky, so where you put colonists starts to matter.

## Sources

| Page | Used for |
|---|---|
| [Shooting Accuracy](https://rimworldwiki.com/wiki/Shooting_Accuracy) | Per-tile table, curve, Sight and Manipulation weights, trait offsets, 99.9 % cap |
| [Aiming Time](https://rimworldwiki.com/wiki/Aiming_Time) | Multiplier on warmup, trait factors, "cooldown unaffected" (stub, verified for 1.3.3101) |
| [Combat](https://rimworldwiki.com/wiki/Combat) | Interception, friendly fire (stub) |
| [Downed](https://rimworldwiki.com/wiki/Downed) | Half chance past 4 tiles, +750 % within 3 (stub) |
| [Ranged Cooldown Multiplier](https://rimworldwiki.com/wiki/Ranged_Cooldown_Multiplier) | Base 100 %; only factor read is a Biotech bandolier |
| [Melee Hit Chance](https://rimworldwiki.com/wiki/Melee_Hit_Chance), [Melee Dodge Chance](https://rimworldwiki.com/wiki/Melee_Dodge_Chance) | Brawler +4, Nimble +15, as published |
| [Traits](https://rimworldwiki.com/wiki/Traits) | **Summary only**, the rows fell past the verbatim cutoff: Careful and Trigger-happy descriptions; Brawler -10 Shooting, +4 Melee, conflicts; Nimble trap rule (garbled, re-read before use) |

**Not on any page read:** warmup cancelled by moving or being hit, XP per shot, stance, shots past 40 tiles, a hit tooltip, the scale between distance 5 and 12 in interception ("scales up"), and two of interception's "four factors".

## Numbers

Score = Shooting level + (Sight − 100 %) × 12, capped at Sight 200 % + (Manipulation − 100 %) × 8, capped at Manipulation 100 % + trait (Careful +5, Trigger-happy −5). The published examples are "+12 from 200% Sight", "+0 from 100% Manipulation". Accuracy per tile comes from the curve, capped at 99.9 %. Chance at range = accuracy ^ distance before band and cover.

| Score | -20 | -10 | -6 | -4 | -2 | 0 | 2 | 4 | 6 | 8 | 10 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| % per tile | 70 | 80 | 83 | 85 | 87 | 89 | 93 | 94 | 95 | 96 | 97 |

| Score | 12 | 14 | 16 | 18 | 20 | 22 | 26 | 30 | 40 | 60 |
|---|---|---|---|---|---|---|---|---|---|---|
| % per tile | 97.5 | 98 | 98.333 | 98.666 | 99 | 99.25 | 99.5 | 99.65 | 99.8 | 99.9 |

Odd levels sit between these in the published table: 1 is 91, 3 is 93.5, 5 is 94.5, 7 is 95.5, 11 is 97.25.

**What the flat offset costs.** A level 10 shooter, chance before band and cover:

| Distance | Plain | Ours, `DeliberateAim` (+12) | Wiki, Careful (+5 score) | Ours, `QuickDraw` (-10) | Wiki, Trigger-happy (-5 score) |
|---|---|---|---|---|---|
| 3 | 91.3 | 100 (clamped) | 94.6 | 81.3 | 84.4 |
| 12 | 69.4 | 81.4 | 80.1 | 59.4 | 50.7 |
| 32 | 37.7 | 49.7 | 55.3 | 27.7 | 16.4 |

So ours overpays at close range and underpays at long range, and `QuickDraw` costs far too little at long range. A fully blind shooter of level 10: ours 61.3 % at 3 tiles and 7.7 % at 32 (flat −30 points); wiki 65.9 % and 1.2 % (score −2).

## Rows

State: `done` shipped; `open` owed to a later change; `kept` differs on purpose. Phase in brackets.

| Id | Gap | Wiki | Today | State |
|---|---|---|---|---|
| SHT-001 | Manipulation in accuracy | Weight 8, capped at 100 % | Shipped: `attack.ManipPoints` (`attack/contract.go:75`, weight 8, no bonus above 100 %) added to `stat.ShootAcc` in `bodyMods` (`body.go:102`); sight through `attack.SightPoints` (`contract.go:80`) | done [SHT-1] |
| SHT-002 | How points apply | Summed into the score, then the curve | Shipped: `attack.ShootingScore` (`attack/attack.go:105`), then `attack.ShooterPerTile`; `ShotAim` at `combat_host.go:136` | done [SHT-1] |
| SHT-003 | Sight | Weight 12, capped at 200 % | Shipped: `sightAccPoints = 12` per 100 % of sight, capped at 200 % (`body.go:40`, `:104`); Nearsighted −6 (`pawngen/stats.go:24-25`, original tuning) | done [SHT-1] |
| SHT-004 | Curve points | The tables above | Shipped: all 21 published points from −20 to 60 (`attack/attack.go:95`); level 1 = 91, level 2 = 93, score −5 = 84 (interpolated between −6 and −4) | done [SHT-1] |
| SHT-005 | Interception by distance | "impossible at a distance of 5 or fewer tiles", scales to full "at a distance of 12 or more" | Shipped: `attack.InterceptScale` (`attack/intercept.go:17`) of `weapon.Range` (`weapon/weapon.go:438`, Euclidean, rounded), applied per creature in `combat/shot.go:200`; none within 5 tiles, full from 12 | done [SHT-1] |
| SHT-006 | Downed interceptors | "reduce the chance", no number | Shipped: a downed creature (humanlike lying or animal `State.Downed`) intercepts at `attack.DownedIntercept` 250 ‰ of the standing chance (`attack/contract.go:28`, original tuning); a standing one in the same cell wins (`combat/shot.go:339`, `reachOf` :371). A downed creature never takes a landing stray | done [SHT-1], own number |
| SHT-007 | Friendly Fire difficulty | 0..100 %, multiplies interception when both are colonists | Shipped: `difficulty.Settings.FriendlyFirePct` (`difficulty/difficulty.go:93`, 100 in every preset, codec key `ff`), clamped at use (`combat/shot.go:377`); scales an aimed colonist shot crossing a colonist (not visitors or leaving raiders, `combat_host.go:76`), interception and landing stray; save v20, older saves 100 (`save.go:124`, `:993`); sliders in the new-game and story options screens | done [SHT-1] |
| SHT-008 | Shooting a downed target | "Shooters beyond 4 tiles have half the chance... Within 3 tiles... +750%" | Shipped: `attack.DownedFactor` (`attack/contract.go:85`: ×8.5 within 3 tiles, ×1 at 4, ×0.5 from 5) in the pawn report (`combat/strike.go:305`) and the turret report (`combat/shot.go:115`), before the clamp; the card shows a "downed target" line | done [SHT-1] |
| SHT-101 | Trait numbers | Careful: +5, aim +25 %. Trigger-happy: −5, aim −50 % | Shipped: `DeliberateAim` +5 score and aim ×1.25; `QuickDraw` −5 score and aim ×0.5 (`trait/trait.go:166-169`) | done [SHT-2] |
| SHT-102 | First aim on a new target | Aiming Time multiplies every warmup | Shipped: `combat.AimTicks(a, id, d)` (`combat/strike.go:45`) scales every first aim by the shooter's aiming time at every combat, raid and order call site; 1.0 for a creature without the stat; melee unscaled; the action bar reads the same number | done [SHT-2] |
| SHT-103 | Melee cooldown and aim | Aiming Time names ranged warmup and psycasts only | Shipped: `Strikes.next` (`combat/strike.go:394-404`) scales only a gun's aim; `PunchTicks` (`mind_host.go:270`) is never scaled | done [SHT-2] |
| SHT-104 | Brawler effect | +4 Melee Hit Chance (published); −10 Shooting, +4 Melee, conflicts (summary) | Shipped: original-named trait `Scrapper` (`trait/trait.go:73`, Melee +4, Shooting −10, needs Violence, trigger group) sets `attack.Melee.Brawler` in `meleeOf` (`skills.go:561`) | done [SHT-2], original name |
| SHT-105 | Nimble effect | +15.0 Melee Dodge Chance | `attack.Melee.Nimble`, same state, `attack/melee.go:103` | open [SHT-2], original name, #273 |
| SHT-201 | Hit card adds up | Not published, ours | Shipped: card lines are successive differences of `attack.Report.Chain()` (`attack/contract.go:33`, `overlay/shot_card.go:27`), so band + factors equal the headline exactly | done [SHT-3] |
| SHT-202 | Hit card when undrafted | Not published, ours | Shipped: `shotCard` (`combat_overlay_host.go:80`) for any selected colonist; the range ring stays drafted-only | done [SHT-3] |
| SHT-203 | Bands and burst in the Gear tab | Weapons publish four bands and burst | Already shown: `inspector/gear/facts.go:89-105` (bands) and `:99` (burst); proven by `TestSHT203WeaponTooltip` | done [SHT-3] |
| SHT-204 | Warmup ring on the shooter | Not published, ours | Shipped: `creature.DrawAimRings` (`render/creature/aim_ring.go:70`) sweeps `AttackCycle` on each in-view pawn aiming a ranged weapon; procedural, no assets, no per-frame allocation. Repeat shots show cooldown + aim as one sweep | done [SHT-3] |
| SHT-301 | Cover of gates and small objects | Summary only: fence gate 75 %, fence 25 %, lamp 20 %, torch 15 %, plant pot 20 % | Shipped: `cover.Gate` 750, `Lamp` 200, `Torch` 150 (`cover/cover.go:60-61`), plant pot as `Stool` 200 (`cover/of.go:27`) | done |
| SHT-302 | Skill decay and the daily cap | Summary only: soft cap then 20 % ([Clarify] on the page); decay from 30 XP a day at level 10 to 3,600 at 20 | Shipped, all skills: `skill.LearnCapped` (`skill/skill.go:171`): full up to 4000 a day (`SoftCapDay`, :45), then 20 %, lossless through `Carry`; decay table `decayXP` (`:216`, none at level 9 and below); the skill sheet's saturated cell now means past the soft cap | done |
| SHT-303 | Sterile tiles | Summary only | Already shipped (AX-3): sterile tile floor raises cleanliness, and surgery reads it (`body/surgery.go:73-75`, ×1.018 at +0.6); proven by `TestSHT303SterileSurgery` | done |
| SHT-304 | Passion XP | Summary only: 35 / 100 / 150 % | 100 and 150 confirmed, `roster/bio.go:187-193`; check the no-passion case | done (2026-11-20 audit: game/roster/bio.go PassionPct: 35/100/150) |

## Proposed phases

- **SHT-1, accuracy inputs and interception.** One score function in `domain/combat/attack`; `ShotAim` returns the score, not a per-tile value and an offset; `Report.Offset` goes away; the curve gets its missing points. Interception takes distance. The Friendly Fire setting goes in `domain/difficulty`. A golden re-pin is expected. Every raider and turret shot changes too.
- **SHT-2, aim and traits.** Trait numbers per the table; scale first aim through the existing role, so `AimTicks(d)` stops being a bare weapon read; separate the melee case; the Brawler and Nimble traits, wired into `attack.Melee` (`skills.go:558`).
- **SHT-3, shot information.** The card shows every factor in the order they apply, for any selected shooter; Gear tab bands and burst; a warmup ring. Read models only.

SHT-1 ships first: SHT-2's trait numbers are only right on the new score.

## Already matching

Miss radius table (wired: `attack/shot.go:52`, values match Combat). Stray 50 % (`attack.StrayHit`). Nothing within 5 tiles of the shooter is intercepted, allies included (SHT-005). Interception body-size rule, 4..80 %. Quality accuracy factors `0.80 .. 1.50` (`quality/quality.go:152`). Weather from the shooter and target's roofs (`combat/combat.go:179-184`). Gun cooldown not scaled by aim (`combat/strike.go:404`). Shooter skill points 0, 5, 10, 15, 20, 30, 40, 60.

## Checklist cross-reference

No checklist row names these gaps, apart from the general `Shooting` skill lines (vanilla checklist 257, 1410) and `Weapon range overlay` (UI checklist 816, already `[x]`). When a phase ships, add rows under the combat section rather than re-opening those.
