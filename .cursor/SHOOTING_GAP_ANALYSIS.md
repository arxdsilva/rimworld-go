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
| SHT-001 | Manipulation in accuracy | Weight 8, capped at 100 % | Only slows work, `body.go:97-99`; only Sight reaches `ShootAcc`, `:100-102` | open [SHT-1] |
| SHT-002 | How points apply | Summed into the score, then the curve | Flat after range, `attack/shot.go:37`; `10*ShootAcc`, `combat_host.go:136` | open [SHT-1] |
| SHT-003 | Sight | Weight 12, capped at 200 % | `sightAccPoints = 30` flat, `body.go:37` | open [SHT-1], with SHT-002 |
| SHT-004 | Curve points | The tables above | Only 0, 5, 10, 15, 20 and beyond, `attack/attack.go:92-93`: level 2 reads 91.2, wiki 93; level 1 90.1, wiki 91; score -5 84.5, wiki 84 | open [SHT-1] |
| SHT-005 | Interception by distance | "impossible at a distance of 5 or fewer tiles", scales to full "at a distance of 12 or more" | No distance term, `attack/shot.go:86-88`, `combat/shot.go:195`. Only allies are safe near the muzzle, `combat/shot.go:192` | open [SHT-1] |
| SHT-006 | Downed interceptors | "reduce the chance", no number | A downed pawn never intercepts: `standing`, `combat/shot.go:191` | open [SHT-1], own number |
| SHT-007 | Friendly Fire difficulty | 0..100 %, multiplies interception when both are colonists | No setting in `domain/difficulty` | open [SHT-1] |
| SHT-008 | Shooting a downed target | "Shooters beyond 4 tiles have half the chance... Within 3 tiles... +750%" | No term in `Strikes.report`, `combat/strike.go:254-280` | open [SHT-1] |
| SHT-101 | Trait numbers | Careful: +5, aim +25 %. Trigger-happy: −5, aim −50 % | `DeliberateAim` +12, ×1.2; `QuickDraw` −10, ×0.75, `trait/trait.go:160-165` | open [SHT-2] |
| SHT-102 | First aim on a new target | Aiming Time multiplies every warmup | `AimTicks(d)` ignores the pawn, `combat/strike.go:35`, used at `combat/engage.go:48`, `combat/defend.go:146`, `combat/hunt.go:70`, `orders.go:286`, `:292`, `mind_host.go:224`, `allies.go:98`. The action bar reads the same number, `action_view.go:246`. Repeat shots already scale, `combat/strike.go:395` | open [SHT-2] |
| SHT-103 | Melee cooldown and aim | Aiming Time names ranged warmup and psycasts only | `AimCooldown` also scales melee cooldown, `combat/strike.go:393`, so `DeliberateAim` slows a knife | open [SHT-2] |
| SHT-104 | Brawler effect | +4 Melee Hit Chance (published); −10 Shooting, +4 Melee, conflicts (summary) | `attack.Melee.Brawler` works and nothing sets it, `attack/melee.go:13`, `:44`; `skills.go:558` | open [SHT-2], original name |
| SHT-105 | Nimble effect | +15.0 Melee Dodge Chance | `attack.Melee.Nimble`, same state, `attack/melee.go:103` | open [SHT-2], original name |
| SHT-201 | Hit card adds up | Not published, ours | Factors omit the band and the offset, `combat_overlay_host.go:189-211`, so they don't sum to the headline | open [SHT-3] |
| SHT-202 | Hit card when undrafted | Not published, ours | Drafted only, `combat_overlay_host.go:60` | open [SHT-3] |
| SHT-203 | Bands and burst in the Gear tab | Weapons publish four bands and burst | `WeaponTip` shows damage, warmup, cooldown, range, `inspector/gear_ui.go:50-51` | open [SHT-3] |
| SHT-204 | Warmup ring on the shooter | Not published, ours | Action bar only | open [SHT-3] |
| SHT-301 | Cover of gates and small objects | Summary only: fence gate 75 %, fence 25 %, lamp 20 %, torch 15 %, plant pot 20 % | Gate counts as fence 25 %, `cover/of.go:20`; lamps none | open, re-read Cover first |
| SHT-302 | Skill decay and the daily cap | Summary only: soft cap then 20 % ([Clarify] on the page); decay from 30 XP a day at level 10 to 3,600 at 20 | Hard stop at 4,000, `skills.go:43`, `:414-416`; `500*(L-9)` above level 10, `skill/skill.go:167-173` | open, all skills, not only Shooting |
| SHT-303 | Sterile tiles | Summary only | None in `domain/difficulty` or `game/combat` | open, needs the building |
| SHT-304 | Passion XP | Summary only: 35 / 100 / 150 % | 100 and 150 confirmed, `roster/bio.go:187-193`; check the no-passion case | open, check first |

## Proposed phases

- **SHT-1, accuracy inputs and interception.** One score function in `domain/combat/attack`; `ShotAim` returns the score, not a per-tile value and an offset; `Report.Offset` goes away; the curve gets its missing points. Interception takes distance. The Friendly Fire setting goes in `domain/difficulty`. A golden re-pin is expected. Every raider and turret shot changes too.
- **SHT-2, aim and traits.** Trait numbers per the table; scale first aim through the existing role, so `AimTicks(d)` stops being a bare weapon read; separate the melee case; the Brawler and Nimble traits, wired into `attack.Melee` (`skills.go:558`).
- **SHT-3, shot information.** The card shows every factor in the order they apply, for any selected shooter; Gear tab bands and burst; a warmup ring. Read models only.

SHT-1 ships first: SHT-2's trait numbers are only right on the new score.

## Already matching

Miss radius table (wired: `attack/shot.go:52`, values match Combat). Stray 50 % (`attack.StrayHit`). Allies safe within 5 tiles of the shooter. Interception body-size rule, 4..80 %. Quality accuracy factors `0.80 .. 1.50` (`quality/quality.go:152`). Weather from the shooter and target's roofs (`combat/combat.go:179-184`). Gun cooldown not scaled by aim (`combat/strike.go:395`). Shooter skill points 0, 5, 10, 15, 20, 30, 40, 60.

## Checklist cross-reference

No checklist row names these gaps, apart from the general `Shooting` skill lines (vanilla checklist 257, 1410) and `Weapon range overlay` (UI checklist 816, already `[x]`). When a phase ships, add rows under the combat section rather than re-opening those.
