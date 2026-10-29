# Pawn-to-pawn interaction gap analysis (SIM-061..SIM-066)

Scope: bring colonist-to-colonist interaction (player orders on a target pawn, rescue, bed choice, feeding and visiting the helpless, strip, mental breaks that involve others) to RimWorld-wiki fidelity. Load only for SIM-061..066 tasks. Phase handoffs go in `.cursor/handoffs/SIM-06N-*.md`.

## Status (2026-10-29)

SIM-061…066 are done (rule and care extraction; orders on a pawn; rescue fidelity and guests; feeding patients; visiting the sick and strip; mental breaks that target others). Open: feeding downed animals, tantrum and escape breaks, an event that brings strangers, and the out-of-scope rows. Every row in section 3 is `open` except the ones that section 3.5 lists as already at fidelity. Rows are ticked per phase by the lead, in the same commit that ships the phase.

## 1. Decision record

| Item | Decision |
|---|---|
| Goal | The user asked for pawn-to-pawn interaction "as close as possible with RimWorld". Their example: select a colonist, right-click a downed one, choose Rescue, and the rescuer carries the patient to a safe bed. |
| Numbers | ENG-003 in `AGENTS.md` was extended on 2026-10-29 to pawn-to-pawn care and mental breaks (SIM-061..066). Published RimWorld wiki numbers are used as tuning data. Names and prose stay original. |
| Clean room | No wiki text, code, art or sprite data is copied. Only numeric facts are taken. Each is a named constant with a `// wiki: <page>` comment, or `// original tuning` where the wiki gives none. Section 4 lists the constants. |
| New packages | `internal/application/game` is at its SOLID ceiling. New features therefore go into concise sub-packages: `internal/application/game/care` (SIM-061, rescue/tend/carry/beds/orders) and `internal/application/game/mind` (SIM-066, mental breaks). Rule: `.cursor/SOLID_GO.md` section 10. A sub-package is a consumer of the root and never imports it. |
| Orders | Pawn-targeted orders ride on `command.Prioritize` with the target in `Command.Target`. That reuses forced-job semantics, the Shift queue (max 3), failure re-queue and disabled reasons. |
| Hysteresis | Every new need- or threshold-driven behaviour (feeding patients, visiting the sick, aggressive breaks) starts and stops at different thresholds or has a cooldown, and ships a no-flap test modelled on `sleep_test.go`. |
| Determinism | No new global RNG and no wall clock. Care takes time from the host and draws on the existing medical stream. Break selection takes its roll from the caller. |
| Game budget | After SIM-061 lowers the game budget, no later phase may leave package game larger (lines, files, `Game` methods) than the previous commit. |

## 2. Reading the tables

Legend for the `State` column: `open`, `done`, `out of scope (needs X)`. `Today` evidence is `file:line` in `internal/application/game` unless a path is given. All references were checked against the code on 2026-10-29.

## 3. Gap tables

### 3.1 Player orders on a target pawn (right-click with a colonist selected)

| RimWorld behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| "Rescue X" on a downed non-hostile pawn, drafted or not. | SIM-062: "Rescue <name>" (`care.PawnOrders`, `care/orders.go`) starts a forced rescue through `Care.Force` (`care/forced.go`); the doer stays drafted; disabled with "no free bed" or "carried by". Downed colonists only until SIM-063. | — | SIM-062 | done |
| "Capture X" on a downed hostile or neutral pawn, carried to a prison bed. | SIM-062: "Capture <name>" on any downed raider or visitor, the selected colonist as captor (`Care.StartCapture`, `care/capture.go`); disabled with "no free prisoner bed". The inspector button still picks the nearest downed hostile. | — | SIM-062 | done |
| "Arrest X" on a standing non-hostile (visitor, colonist in a break). | SIM-062: "Arrest <name>" with the selected colonist as arrester (`Care.StartArrest`). | — | SIM-062 | done |
| "Prioritize tending X" by an undrafted doctor. | SIM-062: "Prioritize tending <name>" (`OrderTendPatient`); Doctor priority never gates a forced order, incapability does. | — | SIM-062 | done |
| "Tend X" when drafted. | `EmergencyTend` works and, since SIM-062, Shift queues it behind the current order (`tick.go`, `order_queue.go` `execOrder`). | — | SIM-062 | done |
| "Feed X" for a downed or bedridden patient. | SIM-064: "Feed <name>" (`Feeding.judge/force`, `care/feed.go`), offered below 80 % food, disabled "no food". | — | SIM-064 | done |
| "Strip X" for a downed pawn or a prisoner. | SIM-065: "Strip <name>" on any downed pawn or a prisoner (`care/strip.go`); stripping a downed guest costs −40 goodwill. | — | SIM-065 | done |
| "Carry X to a cryptosleep casket". | No casket building. | - | - | out of scope (needs a casket building) |
| "Melee attack X" on a berserk colonist. | SIM-066: `standingRaider` admits a standing colonist in berserk or murderous rage; a drafted colonist gets "Melee attack/Shoot <name>" (no "raider" label), anyone "Arrest <name>". | — | SIM-066 | done |
| The order queue names each order's target. | SIM-062: `orderLabel` also matches `Command.Target` (`order_queue.go`; `TestOrderLabelMatchesTarget`). | — | SIM-062 | done |
| A drafted right-click on any pawn with options opens the menu. | SIM-061: `hasPawnOrder` (`internal/adapters/inbound/ebiten/orders/float.go`), true for `EmergencyTend` or `Prioritize` with a target. A drafted group still checks the first drafted pawn's options, which may differ from the primary pawn. | — | SIM-061, SIM-062 | done |

### 3.2 Rescue and bed choice

| RimWorld behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Any downed colonist out of bed is rescued, whatever the cause (injury, blood loss, hunger, disease, toxins, heat, cold). Exhaustion is a collapse into sleep, not a rescue. | SIM-063: `InjuredDowned` (`care/rescue.go`) admits every downed cause but `CauseNoBed`; in-bed anaesthesia stays (out of bed is required). | — | SIM-063 | done |
| Non-hostile downed humanlikes (guests, traders, allies, strangers) are rescued, tended under the guest care group, and leave when healed. Goodwill reacts. | SIM-063: guests (visitors, traders, allies, faction-less strangers) are rescued autonomously and by order, tended under the guest care group, leave when healed (+12 goodwill); a guest dying downed costs −5 (`care/guest.go`, `visitors.go`). | — | SIM-063 | done |
| A rescued faction-less stranger may ask to join (vanilla 19.5). | SIM-063: a recovered stranger asks to join (50 % safe / 100 % unsafe); the letter has Accept (`command.AcceptLetter`) and Decline. No event spawns strangers yet (`Game.DevJoinOffer` for tests). | — | SIM-063 | done |
| A downed prisoner outside a bed is carried back to a prison bed by a warden. | SIM-063: wardens carry a downed prisoner back to a prison bed, autonomously and by "Take <name> to bed" (`care/prisonbed.go`). | — | SIM-063 | done |
| Safe bed: reachable, not burning, not forbidden, not deadly in temperature. A patient that needs care gets a medical bed, else their own bed, else any free bed. | SIM-063: `SafeBedFor` filters fire within a tile and air beyond the 90 °C margin (`BedHazard`), ranks medical beds first for a patient who needs care or a guest, else its own bed, else any free bed; a rescue keeps the patient's own claim and holds another bed for the stay. | — | SIM-061 (merge), SIM-063 (rules) | done |
| A carrier that is downed, drafted or fleeing drops the patient. | SIM-063: draft, a new order, flight and a downed carrier drop the patient through `interrupt`; a patient dying mid-carry is dropped by `Doctor.PatientDied` (combat.go); one test each (`rescue_fidelity_test.go`). | — | SIM-063 | done |
| The carry relation is cheap to query both ways. | `care.Carries` (`care/carry.go`): a two-way index replaced the `g.carrying` map and its linear scan. | — | SIM-061 | done |
| Capture and rescue share one carry-to-bed flow. | SIM-062: `carryToBed` (`care/carrybed.go`) drives both `DoRescue` and `DoCapture`; the trip picks the bed (`SafeBedFor` or the prison bed) and the end action. | — | SIM-062 | done |

### 3.3 Needs of downed and bedridden pawns

| RimWorld behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Downed pawns keep getting hungry. Doctors feed patients who cannot feed themselves. Wardens feed prisoners. | SIM-064: downed pawns' food falls at the awake rate (rest stays frozen); doctors feed a downed patient at or below Hungry (2,500), the warden's food run shares the driver (`Feeding`), a failed run retries after 1,000 ticks. | — | SIM-064 | done |
| Bedridden patients gain recreation from visitors ("visit sick"). | SIM-065: friends (opinion 20+) visit an awake bedridden colonist for 1,500 ticks, both gain 0.216 of the recreation bar; awake patients are social partners. The pair cooldown is a fixed time window (no saved state). | — | SIM-065 | done |
| Downed animals are fed. | Not done (MED gap section 7). | Optional extension of Feed. | SIM-064 | open |

### 3.4 Mental breaks that involve others (vanilla 10.1)

| RimWorld behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Insulting spree and targeted insulting spree. | SIM-066: insulting spree and targeted insulting spree insult through the social interaction path and can start a social fight (`internal/application/game/mind`). | — | SIM-066 | done |
| Berserk (attacks anyone near) and murderous rage (hunts one target). | SIM-066: berserk attacks the nearest creature until downed or timed out; murderous rage hunts one colonist or prisoner until it dies or the rager is downed or arrested. | — | SIM-066 | done |
| Catatonic breakdown (downed until recovered, needs rescue and feeding). | SIM-066: catatonic breakdown downs the pawn (`health.CauseCatatonic`) for 100,000–300,000 ticks; rescue and feeding apply. | — | SIM-066 | done |
| Break severity tiers (minor, major, extreme) gate which breaks can fire. | SIM-066: tiers at mood 35/20/5 shifted by break-threshold traits, wiki MTBs, `mentalbreak.Pick` with the caller's roll. | — | SIM-061 (domain), SIM-066 | done |
| Arrest a colonist in a break. | SIM-062: "Arrest <name>" in the right-click menu (the target may be in a break; the arrester may not). | — | SIM-062 | done |

### 3.5 Already at fidelity (no work)

Tending priority chain and quality, care policy, self-tend, surgery queue, social interaction catalogue and opinions, romance, marriage and breakups, social fights, prisoner modes, recruit, release and execute, warden feeding, kidnap, drafted move and attack, rescue alert, rescued-pawn gratitude. State: `done`.

### 3.6 Out of scope

| Item | Missing prerequisite | State |
|---|---|---|
| Cryptosleep casket carry | No casket building. | out of scope (needs a casket building) |
| Beat out fire on a burning pawn | Pawns have no burning state. | out of scope (needs a burning state) |
| Resurrection | No resurrection item or serum. | out of scope (needs a resurrection item) |
| Carry to an arbitrary spot | No free-carry command or drop target. | out of scope (needs a carry-to-cell command) |
| Downed-animal carry to animal beds | No animal bed building or animal carry job. | out of scope (needs animal beds) |

## 4. Wiki constants

Values from the W1 research (2026-10-29). Most wiki pages state no game version, and the Mental_break page is marked as needing a fact-check. Where the wiki publishes nothing, the row says so and records the decision. Every constant lands in code as a named constant with `// wiki: <page>` or `// original tuning`. Ticks: one in-game hour is 2,500 ticks, so a day is 60,000, the wiki's scale.

| Constant | Value | Source | Used by |
|---|---|---|---|
| Hungry threshold (patient feeding starts at or below it) | 2,500 of `needs.Full` 10,000 (wiki 0.25 saturation); equals the existing `needs.CriticalHunger` | https://rimworldwiki.com/wiki/Saturation | SIM-064 |
| Ravenous threshold | 1,250 of 10,000 (wiki 0.125) | https://rimworldwiki.com/wiki/Saturation | SIM-064 |
| Feeding trigger above Hungry | not published: original tuning, feed at Hungry | https://rimworldwiki.com/wiki/Work | SIM-064 |
| Downed food-need fall | not published: same rate as awake (original tuning) | https://rimworldwiki.com/wiki/Saturation | SIM-064 |
| Doctor work-giver order | urgent tend, tend, emergency self-tend, self-tend, feed patient, surgery, rescue ally, animal tend, feed animal patient, animal surgery, sick visit (entity/hemogen/bioferrite rows have no counterpart) | https://rimworldwiki.com/wiki/Work | SIM-063, SIM-064, SIM-065 |
| Goodwill: a downed faction member recovers and leaves | +12 | https://rimworldwiki.com/wiki/Guest | SIM-063 |
| Goodwill: return a downed member to its faction | +16 | https://rimworldwiki.com/wiki/Goodwill | SIM-063 |
| Goodwill: strip a downed guest | −40 (the Goodwill and Factions pages; the Guest page says none: decision −40) | https://rimworldwiki.com/wiki/Goodwill | SIM-065 |
| Goodwill: a guest dies downed on the map | not published: −5, the published 'non-hostile member killed' value | https://rimworldwiki.com/wiki/Goodwill | SIM-063 |
| Goodwill: arrest a visitor | relation set to −75 | https://rimworldwiki.com/wiki/Goodwill | SIM-062 (reference) |
| Rescued-stranger join chance | 0.5 when leaving is safe, 1.0 when not (page tagged Verify) | https://rimworldwiki.com/wiki/Guest | SIM-063 |
| Rescue safety margin | bed reachable within 90 °C beyond the rescuer's comfort range | https://rimworldwiki.com/wiki/Rescue | SIM-063 |
| Opinion of the rescuer | +15 (page also lists +25; decision +15) | https://rimworldwiki.com/wiki/Social | SIM-063 |
| Visit-sick recreation gain | 0.216 of the recreation bar for both, over 1500 ticks | https://rimworldwiki.com/wiki/Joy | SIM-065 |
| Visit-sick minimum opinion | not published: original tuning | https://rimworldwiki.com/wiki/Joy | SIM-065 |
| Mental-break threshold, minor / major / extreme | 35 / 20 / 5 (mood 0–100); major = 4/7 and extreme = 1/7 of minor; minor clamped 1–50 | https://rimworldwiki.com/wiki/Mental_break | SIM-066 |
| Mean time between breaks, minor / major / extreme | 4 / 0.8 / 0.5 days | https://rimworldwiki.com/wiki/Mental_break | SIM-066 |
| Trait threshold offsets | iron-willed −18, steadfast −9, nervous +8, neurotic +8, volatile +15 | https://rimworldwiki.com/wiki/Traits | SIM-066 |
| Insulting spree | minor tier; 25,000–45,000 ticks; weight 0.5; needs 2+ other colonists | https://rimworldwiki.com/wiki/Mental_break | SIM-066 |
| Targeted insulting spree | minor tier; 25,000–45,000 ticks; weight 0.5 | https://rimworldwiki.com/wiki/Mental_break | SIM-066 |
| Berserk | extreme tier; 40,000–60,000 ticks; weight 1.0 | https://rimworldwiki.com/wiki/Mental_break | SIM-066 |
| Murderous rage | extreme tier; 100,000 ticks; weight 1.0; ends when the target dies or the pawn is downed or arrested | https://rimworldwiki.com/wiki/Mental_break | SIM-066 |
| Catatonic breakdown | extreme tier; 100,000–300,000 ticks; weight 1.0; the pawn is incapacitated | https://rimworldwiki.com/wiki/Mental_break | SIM-066 |
| Sad wander / food binge / hide in room (existing) | minor; 40–60k, 25–45k, 40–60k ticks; weights 0.5, 0.8, 0.5 (adopted in SIM-066) | https://rimworldwiki.com/wiki/Mental_break | SIM-066 |
| Arrest base chance | not published: the existing SIM-045 formula stays | https://rimworldwiki.com/wiki/Arrest | — |

## 5. Phase plan

**SIM-061, rule, guard, gap doc, care extraction.** No behaviour change. The SOLID rule allows concise sub-packages, a guard forbids a game sub-package from importing the root, and this document is written. The care cluster (carry index, care policy, bed rest, doctor rescue and tend, bed ranking, order building) leaves package game for `internal/application/game/care`, behind consumer-owned roles. Golden hashes and old saves stay byte-identical. Domain groundwork lands inert: the new `Order` values, job kinds and mental-break kind constants, plus the pure tier tables. Moves: no checklist item; it prepares 5.5, 6.2 and 11.5.

**SIM-062, pawn-targeted orders.** The user's example. Right-click on a pawn with a colonist selected offers Rescue, Capture, Arrest, Prioritize tending and Tend, each with a disabled reason when it cannot run. Capture and arrest move into care and share one carry-to-bed driver with rescue. Queue labels name the target. The inspector shows carrying and carried-by lines. Moves: vanilla 5.5 and 6.2; UI 3.2 and 13.2.

**SIM-063, rescue fidelity.** Rescue admits every downed cause except collapse-asleep. `SafeBedFor` adds the hazard check and keeps the patient's own claim. Non-hostile downed humanlikes are rescued, tended, and leave on recovery, with goodwill changes. A rescued stranger may offer to join. Wardens carry downed prisoners back to a prison bed. Carrier interruptions drop the patient, one test each. Moves: vanilla 9.1, 11.3, 11.5, 19.1 and 19.5; UI 10.4.

**SIM-064, feeding patients and downed needs.** Food need decays again for downed and bedridden pawns, in the same commit as the Doctor Feed-patient giver. The giver has hysteresis, a per-patient reservation and a no-flap test. "Feed X" joins the menu. The warden food run becomes the one feeding driver. Moves: vanilla 11.5 (bed-rest note) and the MED gap feeding item if done.

**SIM-065, social care and strip.** Visit sick: a colonist walks to an awake bedridden friend and chats beside the bed; the patient gains recreation; a per-pair cooldown and a no-flap test guard it. "Strip X" works on a downed pawn or a prisoner; stripping a guest costs goodwill. Moves: vanilla 10.3 (note).

**SIM-066, mental breaks that target others.** `mental.go` moves into `internal/application/game/mind`. New kinds: insulting spree, targeted insulting spree, berserk, murderous rage, catatonic breakdown, with tiers, durations and weights from section 4. Insults feed the social-fight path; berserk and rage pick targets with a retarget cooldown; catatonic downs the pawn so rescue and feeding apply. The HUD names the break and its target. Moves: vanilla 10.1 (aggressive, catatonic, eligibility).

## 6. Checklist cross-reference

Each phase ticks these items when its acceptance criteria are met. Titles are quoted exactly. The lead edits the checklists; this file does not.

### 6.1 `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`

| Section | Item | Phase |
|---|---|---|
| 5.5 Forced orders | **Right-click/context forced work** (already `[x]`; add the pawn entries to its note) | SIM-062 |
| 6.2 Carrying/inventory | **Pawn carry tracker** (`[-]`; patient carry is a second tracker owned by care) | SIM-062 |
| 9.1 Rest | **Beds** (`[-]`; the note already lists medical beds; bed rest-effectiveness stays missing) | SIM-063 |
| 10.1 Mental breaks | **Mental break eligibility** (`[-]`) | SIM-066 |
| 10.1 Mental breaks | **Destructive/aggressive breaks** (`[ ]`) | SIM-066 |
| 10.1 Mental breaks | **Catatonic/extreme shutdown equivalent** (`[ ]`) | SIM-066 |
| 10.3 Social interactions | **Social fight** (`[x]`; its note says the insulting-spree break is not done) | SIM-066 |
| 10.3 Social interactions | **Periodic social interaction** (`[x]`; visit-sick note) | SIM-065 |
| 11.3 Injuries | **Wounds** (`[x]`; downed-cause breadth, note only) | SIM-063 |
| 11.5 Medical care | **Tending** and **Bed rest** (both `[x]`; add rescue and feeding notes) | SIM-063, SIM-064 |
| 19.1 Capture | **Prisoner state** (`[-]`) | SIM-063 |
| 19.5 Other recruitment | **Rescue/recovery join chance** (`[ ]`) | SIM-063 |

### 6.2 `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`

| Section | Item | Phase |
|---|---|---|
| 3.2 Context command bar | **Context-sensitive actions** (`[-]`; add the pawn-targeted entries to its note) | SIM-062 |
| 10.4 Medical settings | **Bed-rest/medical-bed state** (`[ ]`) | SIM-063 |
| 13.2 Tactical orders | **Rescue/capture contextual commands** (`[ ]`) | SIM-062 |

## 7. Out of scope

See section 3.6. Each item stays listed with its missing prerequisite. A phase never half-implements one of them.
