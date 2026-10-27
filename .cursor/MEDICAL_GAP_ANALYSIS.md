# Medical system gap analysis (MED-00..MED-13)

Scope: bring the medical system (wounds, bleeding, tending, infection, diseases, anatomy, bionics, surgery, drug health effects, aging, animals) to RimWorld-wiki parity of behaviour and numbers. Load only for MED-* tasks. Phase handoffs go in `.cursor/handoffs/MED-0N-*.md`.

## Status (2026-10-26)

All phases MED-00 to MED-09 are implemented. Each has a handoff in
`.cursor/handoffs/` (`MED-0N-*.md`); the art is in `ART-026-medical-items.md`.
Sections 2 and 3 below describe the system **before** the overhaul and are
kept as the analysis record. Section 4's constants are what the code uses,
with these deviations:
- Tend curve: the wiki's exact curve (lvl 18 = 1500, lvl 20 = 1550).
- Sight in tend quality: a linear 70 % weight.
- The surgery medicine factor and cleanliness curve are original tuning.
- The ridiculous-failure share is original tuning.

Follow-ups MED-10 to MED-13 (2026-10-27) closed the earlier leftovers:
- the capacity-breakdown tooltip and the vitals-monitor research gate (MED-10);
- food poisoning and disease on caravans (MED-11);
- downed animals (MED-12);
- per-archetype animal anatomy (MED-13).

Their numbers are original tuning. Section 7 lists what is still deferred, plus:
- toxic buildup;
- carcinoma part efficiency;
- animal scars and lost parts in butchering yield;
- feeding or euthanizing animals that stay down.

## 1. Purpose and decision record

| Item | Decision |
|---|---|
| Numbers | Published RimWorld wiki numbers are used as tuning data, extending the ENG-003 exception in `AGENTS.md` ("social/host-system feature set") to the medical system. User decision, 2026-10-26. |
| Names and prose | Original. RimWorld-coined item and condition names are replaced (see section 5). Real-world disease names (flu, plague, malaria, sleeping sickness) are kept. UI text, tooltips, letters, and doc prose are written fresh. |
| Clean room | No wiki text, code, art, or sprite data is copied. Only numeric facts (rates, thresholds, odds, curves) are taken, and every one is stored as an integer permille constant with a `// wiki: <page>` comment. Where the wiki gave no number the comment is `// original tuning` and the phase handoff lists it. |
| Units | Permille (‰) unless stated. 1 h = `clock.TicksPerHour`. RimWorld uses 2,500 ticks per hour; durations are converted by hours or days, never raw ticks. |
| Determinism | New RNG stream `streamMedical` (`internal/application/game/rng_streams.go`). Infection rolls move off `g.rng`. No wall clock. |
| Hysteresis | Every new threshold behaviour has enter/exit bands or a cooldown and a no-flap test on `kindChanges(g,id,n)` (`sleep_test.go:220`), modelled on `TestBedRestDoesNotFlap` (`bedrest_test.go:313`). |
| Golden hashes | `scale_golden_test.go` pins hashes over runs with injuries. P1 and P2 change behaviour on purpose, so those pins are re-pinned after two same-seed runs match (`TestStepIsDeterministicForSameSeed`, `TestBodiesAreDeterministic`). The handoff states why. |
| Agents | Domain and application agents: sonnet (opus for P0, P2, P6). Lead-only files: `save.go`, `command.go`, `tick.go`, `host_commands.go`, `views_social.go`, `phases.go`, `incident.go` defs, `item.go` kinds, `rng_streams.go`, checklists, `AGENTS.md`. |

## 2. Current system summary

| Area | Summary (file refs) |
|---|---|
| Anatomy and wounds | `body.Body{Wounds []Wound; Missing uint32; Implants [NumParts]item.Kind}` (`domain/pawn/body/wound.go:37`). 20 parts (`anatomy.go:7-27`, defs `:46`), 7 capacities (`capacity.go:16`). A `Wound` carries `Dmg`, `Permanent` (scar), `Tended bool` (`wound.go:24-29`). `Heal` closes the smallest open wound first and may leave a scar (`wound.go:104`, `scarChance = 100` permille = 10 %, `:19`). |
| Capacities and pain | `Capacity()` folds part efficiency and implants (`capacity.go:54`). `Pain()` (`capacity.go:113`): open wound dmg x8 permille, tended x4, scar x1, capped at 1000. |
| Health and downed | `health.MaxHP = 100`, `DownedInjury = 70`, `RecoverInjury = 55` (`domain/pawn/health/health.go:86-87`). Cause enum `CauseNone..CauseInfection..CauseExecution/CauseSurgery` (`:24-35`). Downing is a scalar injury threshold, not capacity-driven. |
| Blood and bleeding | There is no blood-loss model. Blood is only ground stains: `colonistBleed = 15` / `raiderBleed = 30` injury drip thresholds (`stains.go:26-27`), `bleeding()` (`stains.go:188`). A downed raider dies on a fixed `bleedOut = 4 h` timer (`raids.go:75`, checked `:394`). |
| Tending | `assignDoctor` (`medical.go:97`) picks patients; `doTend` (`medical.go:255`) finishes a tend. Care is a 5-value enum `infection.Care` (Untreated, SelfTended, Tended, Medicine, IndustrialMedicine; `infection/infection.go:12-19`). `tendCare` picks it from the carried medicine (`game/infection.go:187`); `withMedicine` adds the fetch (`game/infection.go:127`). `tendHealBy` (`skills.go:622`) and `doctorInfectionPct` (`skills.go:637`) scale heal and infection by tier, hospital bed, and Medicine skill. Hospital bed constants `bedHealPct = 115`, `bedInfectionPct = 50` (`items_ng1.go:62-63`). `severeInjury = 50` picks industrial medicine (`items_ng1.go:56`). Wounds only have a `Tended bool`; there is no tend quality. |
| Infection | One infection per pawn: `wound{At, Infection, Care}` in `g.wounds` (`game/infection.go:23`). Window `DelayHours = 8`, `Onset = 15`, `Fatal = 100`, `Sick = 20`; chance by care tier (`infection/infection.go:25-60`, `Chance`, `Step`, `HealPct`). No immunity. Raiders and prisoners are skipped (`woundOpened`, `game/infection.go:40`). Hourly `phaseInfection` (`game/infection.go:236`, called `game.go:982`). |
| Bed rest | `bedrest.go`: sick, hurt, or untended colonists rest in bed with a minimum duration (hysteresis); `lieDownForCare`, `lyingForCare` (`game/infection.go:89,113`). |
| Surgery | Three `OpKind`s (install implant, remove, amputate); command bound `op > int(body.Amputate)` (`game/surgery.go:130`). Own chance curve `body.SurgeryChance(skill, hospitalBed, medicine, manip, sight)` (`domain/pawn/body/surgery.go:34`: 400..850 up to level 8, then to 980, x1.1 bed, x1.1 industrial, x0.7 no medicine). Outcomes Success/fail via `RollOutcome` (`surgery.go:571`). `assignSurgery` (`:426`) iterates `g.pawnIDs` only, so prisoners and guests are never operated. Anesthesia is a flat `anaesthTicks = 1 h` (`surgery.go:34`, `:601`). |
| Implants | `ImplantEfficiency` table (`capacity.go:21-46`): peg leg 600, prosthetic leg 850, prosthetic arm 850, wooden hand 500, denture 800, prosthetic foot 750. Item values in `domain/economy/item/value.go:15` (prosthetic leg/arm 120, peg leg 15, wooden hand 12, denture 10); ProstheticFoot, GlassEye, and WoodenNose have no row there. |
| Drugs | `drug.State` holds `Tolerance [numDrugs]int32` (`drug.go:91`) and `LastDose [numDrugs]uint64` (`:97`); definitions are beer, ashleaf tea, spark tablet (`drug.go:40-42`). Drugs have mood, joy, rest, and addiction effects but no health effects (no painkiller, no overdose). |
| Incidents and difficulty | `difficulty.DiseaseFreq` exists and is shown in the UI as inert: "No diseases yet" (`ebiten/screen_difficulty.go:125,129`, `story_tab.go:302,306`). No disease incidents in `domain/incident`. |
| Animals | Scalar `Injury` on animals; vets use `tendHealBy` (`vet_animals.go:96`). |
| Save | `SaveVersion = 5` (`save.go:72`), gob encoding. Gob rejects a resized array, so `Implants [20]` and `[numDrugs]` arrays cannot grow without a slice-plus-legacy conversion (`save.go:189` comment). |

## 3. Gap tables

Legend: Phase = MED-0N. "Ours today" refs verified against the code on 2026-10-26.

### 3.1 Bleeding, blood loss, pain, downed

| RimWorld behaviour | Ours today | Gap | Phase |
|---|---|---|---|
| Each fresh wound bleeds at a rate set by damage, injury kind, and part (heart much worse); tending stops it; burns and bruises do not bleed. | Stains only (`stains.go:26`, `:188`); no per-wound bleed. | Per-wound bleed rate, cached on the wound. | 1 |
| A blood-loss meter fills while bleeding, drains consciousness by stages, kills at full, and refills slowly when not bleeding. | None. Raiders bleed out on a 4 h timer (`raids.go:75,394`). | Blood-loss model, stage table with hysteresis, regen, `CauseBloodLoss`, raiders bleed like colonists. | 1 |
| Pain scales with open and tended wounds alike; heavy pain shock downs the pawn. | Open dmg x8, tended x4 (`capacity.go:113-125`). | Pain 12.5 permille per HP for open and tended; shock threshold. | 1 |
| A pawn is downed by low consciousness, low moving, or pain shock, and stands again with a margin. | Downed at injury 70, up at 55 (`health.go:86-87`). | Capacity-driven downing with exit band; `DownedAt`/`RecoverAt` kept for animals until P9. | 1 |
| Player sees bleeding and time to death. | No HUD. | Health tab state line, blood row, bleed tooltip, status line, `bleeding` alert. | 1 |

### 3.2 Tending, healing, scars

| RimWorld behaviour | Ours today | Gap | Phase |
|---|---|---|---|
| Tend quality is a number from doctor skill, hands, eyes, medicine potency, bed, self-tend, and a random factor, capped by the medicine. | `Tended bool`; heal and infection scaled by tier (`skills.go:622,637`). | `TendQuality` formula, `TendQ`/`TendAt` on wounds. | 2 |
| Wounds heal at a base daily rate, faster tended (by quality) and when lying (bed, hospital bed). | `HealTicks = 300` per injury point, bed x2 (`health.go:88-92`); tend heals once at completion (`medical.go:284`). | Per-wound wiki rates with tend and lying offsets. | 2 |
| Scar chance grows with wound size. | Flat 10 % (`wound.go:19`). | Scar odds follow damage. | 2 |
| Wounds need re-tending after a window. | One tend, no window. | `TendUntil` windows, re-tend eligibility, `untended` alert. | 2 |

### 3.3 Infection

| RimWorld behaviour | Ours today | Gap | Phase |
|---|---|---|---|
| Each wound may get its own infection a few hours after injury; chance depends on injury kind, tend quality, and room cleanliness. | One infection per pawn (`game/infection.go:23`); chance by care tier only (`items_ng1.go:97`). | Per-wound infection condition, `WoundChance(kind, tendQ, room)`, `InfectAt` roll 6-18 h. | 2 |
| Severity races immunity: immunity builds, severity grows, tending slows it; the loser decides death or cure. | Severity only, no immunity (`infection.go:51 Step`). | Shared `StepRace` engine, `Immunity` field. | 2 |
| Prisoners and raiders get infected too. | Skipped (`game/infection.go:40`). | Remove the skip. | 1-2 |
| Players read severity and immunity. | Infection shown as a single number. | Condition rows with severity and immunity bars, `infection` alert. | 2 |

### 3.4 Care policy and doctoring

| RimWorld behaviour | Ours today | Gap | Phase |
|---|---|---|---|
| Per-pawn medical care level caps the medicine used; group defaults; prisoners and guests default lower. | Doctor picks medicine by severity (`items_ng1.go:56`, `game/infection.go:127`). | 5-level policy, group defaults, `SetMedicalCare`/`SetDefaultCare`. | 3 |
| A pawn may refuse self-tend. | Colonist tends itself when alone (`game/infection.go:11-14` doc). | `SetSelfTend` toggle. | 3 |
| Doctor priority: rescue, bleeding, other tends, surgery. | `assignDoctor` order in `priorities.go:103-107`. | Explicit chain with deterministic tie-break; bleed-sorted candidates (`medical.go:97`). | 1, 3 |
| Drafted doctor can tend a patient on command. | None. | `EmergencyTend` command and job. | 3 |
| Queued operations can be reordered. | Fixed queue. | `ReorderOperation`. | 3 |

### 3.5 Diseases

| RimWorld behaviour | Ours today | Gap | Phase |
|---|---|---|---|
| Flu, plague, malaria, sleeping sickness: immunity race with lethal outcome, tend windows, biome limits. | None; `DiseaseFreq` inert. | Disease `CondKind`s, incidents, biome fit, bed rest. | 4 |
| Gut worms, muscle parasites: chronic, cured by cumulative tend quality. | None. | Parasite conditions with `CureTendSum`. | 4 |
| Frequency setting scales incidents. | Slider has no effect. | Wire `DiseaseFreq`; 0 disables. | 4 |

### 3.6 Anatomy, capacities, bionics

| RimWorld behaviour | Ours today | Gap | Phase |
|---|---|---|---|
| Kidneys, liver, spine exist; organ failure kills. | 20 parts, no kidney/liver/spine (`anatomy.go:7-27`). | Four new parts (24 total < 32-bit mask), coverage re-split to 1000. | 5 |
| Blood pumping, filtration, digestion capacities. | 7 capacities (`capacity.go:16`). | Three new capacities; capacity zero is death (`CauseOrganFailure`). | 5 |
| Bionic tier above natural (efficiency over 100 %). | Best implant is 850. | Servo tier, optic/cochlear, spinal brace, pump heart, synth stomach, valve heart. | 5 |
| Prosthetic arm 50 %, wooden hand 60 %. | Prosthetic arm 850 (`capacity.go:23`), wooden hand 500 (`:24`). | Fix both. | 5 |
| Item values reflect rarity. | Foot/eye/nose missing from `value.go`. | Add ProstheticFoot 90, GlassEye 40, WoodenNose 5 (original tuning). | 0 |
| Implants survive saves as the roster grows. | Fixed `[NumParts]` array (`wound.go:40`). | `Fitted []item.Kind` slice with legacy read. | 0 |

### 3.7 Surgery

| RimWorld behaviour | Ours today | Gap | Phase |
|---|---|---|---|
| Success chance from skill, hands, eyes, bed, cleanliness, light, vitals monitor, operation type. | Skill, bed, medicine, manip, sight (`body/surgery.go:34-60`). | Wiki table and factors with a tooltip breakdown. | 6 |
| Failure splits into minor, catastrophic, and absurd outcomes; some ops kill on failure. | Success/fail only (`surgery.go:571`). | Four outcomes; per-op death rate. | 6 |
| Prisoners and guests can be operated; organs harvested; euthanasia. | `assignSurgery` scans `g.pawnIDs` only (`surgery.go:426`); 3 ops (`surgery.go:130`). | New op kinds, prisoner/guest iteration, organ items, thoughts. | 0, 6 |
| Anesthetic is a staged condition. | Flat 1 h (`surgery.go:34,601`). | `CondAnesthetic` stages. | 6 |
| Vitals monitor next to a hospital bed. | None. | Powered 80 W building, adjacency lookup. | 6 |

### 3.8 Drugs, aging, animals

| RimWorld behaviour | Ours today | Gap | Phase |
|---|---|---|---|
| Painkiller halves pain for a while; overdose kills; a prophylactic blocks some diseases. | No health effects (`drug.go`). | Two new drugs, `Overdose` condition, policy auto-use with hysteresis. | 7 |
| Chronic age conditions and heart attack. | No age-driven health. | Seven chronic conditions plus heart attack. | 8 |
| Animals bleed, get tended, get infected. | Scalar `Injury` (`vet_animals.go:96`). | Lightweight body adapter with blood loss, tend, infection, care. | 9 |

## 4. Constants table

Conversion: permille unless a unit is shown. "Page" is the wiki page the number comes from (wiki: Ailments, Injury, Infection, Doctoring, Medical Tend Quality, Flu, Plague, Malaria, Sleeping sickness, Gut worms, Muscle parasites, Vitals monitor, Kidney, Heart, Bionic_* pages, Prosthetic_* pages, Peg leg, Wooden hand, Denture, Penoxycyline, Overdose, Human resources). Rows marked **original** are not from the wiki.

### 4.1 Blood, bleeding, pain, downed (MED-01)

| Constant | Value | Page |
|---|---|---|
| Blood-loss stage entry | 150 / 300 / 450 / 600 | Ailments |
| Exit margin (hysteresis) | 5 points below entry | **original** |
| Consciousness modifier by stage | -100 / -200 / -400 / -400 | Ailments |
| Consciousness cap | 100 at loss >= 600 | Ailments |
| Death | loss 1000 | Ailments |
| Regeneration | 333 per day, only while not bleeding | Ailments |
| Bleed per wound per day | dmg x kind factor x part factor (heart x5); tended 0; burn and bruise 0 | Injury, Ailments |
| Pain | 12.5 per HP of open or tended wound; tending does not reduce it | Injury |
| Pain shock | 800 | Injury |
| Downed enter | consciousness < 300, or moving < 150, or pain >= 800 | Ailments |
| Downed exit | consciousness >= 350 and moving >= 200 and pain <= 750 | **original** |

### 4.2 Tend, heal, scar (MED-02)

| Constant | Value | Page |
|---|---|---|
| Tend base quality | 200 + 100 x skill up to 8 (1000), then +46 per level to 1550 at 20 | Medical Tend Quality |
| Multipliers | manipulation (cap 1400) x sight^0.7 (cap 1400) x potency | Medical Tend Quality |
| Bed offset | hospital bed +100, vitals monitor +70 | Medical Tend Quality, Vitals monitor |
| Self-tend | x0.7 | Medical Tend Quality |
| Random factor | uniform 750-1250 | Medical Tend Quality |
| Potency / max quality | none 300/700; herbal 600/700; industrial 1000/1000; nanomed 1600/1300 | Medical Tend Quality |
| Base heal per wound | 8 HP per day | Injury |
| Tended heal | +40 + 0.08 x quality HP per day (+4 at 0, +12 at 1000) | Injury |
| Lying offset | +4 ground, +8 bed, +14 hospital bed | Injury |
| Scar odds | 20 x clamp((dmg-4)/10, 0, 1) | Injury |
| Re-tend window | per `CondDef.TendEvery` hours; wounds: tended until healed or new wound | Doctoring |

### 4.3 Infection (MED-02)

| Constant | Value | Page |
|---|---|---|
| Roll time | 6-18 h after the wound | Infection |
| Base chance | bite/burn 300; other bleeding 150; bruise/missing 0 | Infection |
| Tend factor | 850 down to 50, linear in quality | Infection |
| Room factor | roofed + floored + no filth 500; else 1000 | Infection |
| Severity per day | +840; tended down to -530 by quality; immune -700 | Infection |
| Immunity per day | +644 | Infection |
| Stages | 330 / 780 / 870; death 1000 | Infection |

### 4.4 Diseases (MED-04)

Columns: severity per day / tended-quality reduction / immune severity per day / immunity per day; tend interval; stages.

| Disease | Sev +/day, immune, imm gain | Tend | Tend every | Stages | Page |
|---|---|---|---|---|---|
| Flu | +249 / -495 / +239 | -77 max | 12 h | 666, 833; lethal | Flu |
| Plague | +666 / -333 / +522 | -363 | 15 h | 600, 800, 910; lethal | Plague |
| Malaria | +370 / -730 / +315 | -232 | 12 h | 780, 910; lethal | Malaria |
| Sleeping sickness | +120 / -176 / +110 | -70 | 32 h | 437, 625, 875, 937; lethal; tropical biomes only | Sleeping sickness |
| Gut worms | hunger +100 %, pain +200, vomit daily, mood -5; cured at cumulative tend quality >= 3000 | | 48 h | no immunity | Gut worms |
| Muscle parasites | pain +200, rest fall +100 %, manipulation and moving -300; same cure | | | no immunity | Muscle parasites |
| Incident weight, cooldown, MinDay 15 | | | | | **original** (wiki has none) |
| Malaria warm biomes only | | | | | **original** biome mapping |

### 4.5 Surgery (MED-06)

| Constant | Value | Page |
|---|---|---|
| Base by Medical level 0-20 | 100, 200, 300, 400, 500, 600, 700, 750, 800, 850, 900, 920, 940, 960, 980, 1000, 1020, 1040, 1060, 1080, 1100 | Doctoring |
| Factors | x manipulation x min(sight,1000) x bed (1000; hospital 1150) x cleanliness (600-1018; outdoors 850) x light (down to 750 in darkness when <50 % lit) x vitals monitor 1050 x op multiplier; cap 980 | Doctoring, Vitals monitor |
| Op multipliers | install 1000, remove 1200, harvest 1200, anesthetize 2000, euthanize 5000, excise carcinoma 1000 | Doctoring |
| Death on failure | install heart 250, excise carcinoma 250; spinal brace destroys part | Heart, Doctoring |
| Ridiculous-failure share | 10 % of failures when skill < 6, else 2 % | **original** |
| Anesthetic | sedated 6 h (consciousness max 10); woozy 6-12 h (max 700; moving, manipulation -200); wearing off to 20 h (max 900) | Doctoring |
| Medicine per op | install/harvest 2 herbal-or-better; anesthetize 1 | Doctoring (organ harvest medicine count: **conflict**, see 4.10) |
| Harvest mood | donor -30 for 7 d; colonists -5 (-6 if a colonist did it), stacking 5 x 0.75 | Human resources |
| Goodwill | -80 kidney / -160 heart, cap -100, only if a faction goodwill API exists | Human resources |
| Vitals monitor | adjacent to a hospital bed, one per bed, 80 W; tend +70, surgery x1050, immunity +20 | Vitals monitor |

### 4.6 Implants and organs (MED-05)

| Constant | Value | Page |
|---|---|---|
| Efficiency | peg leg 600, prosthetic leg 850, prosthetic arm 500 (was 850), wooden hand 600 (was 500), denture 800, valve heart 800, servo arm/leg/optic/pump/synth 1250, cochlear 1250, spinal brace 1000 | Peg leg, Prosthetic_*, Wooden hand, Denture, Bionic_* |
| Value (silver) | servo arm/leg/optic/brace/pump 1030; cochlear/synth 785; prosthetic leg/arm 260; valve heart 230; peg leg / wooden hand 1.2 -> 2; organs: kidney 900, lung 1000, heart 1200, liver 1200 | Bionic_*, Prosthetic_* |
| ProstheticFoot 90, GlassEye 40, WoodenNose 5 | | **original** |
| Kidney | 50 HP, 500 filtration each; one kidney: consciousness -50, immunity gain -250 | Kidney |
| Heart | 50 HP, 1000 pumping; destroyed means death | Heart |
| Death rule | filtration, pumping, breathing, or digestion at 0 | Ailments |
| New part HP / coverage | kidneys 50 HP / 17; liver 50 / 25; spine 40 / 25; torso coverage reduced to keep 1000 | **original** layout |
| Recipes | servo: Crafting 8; valve heart: steel 25 + 4 components, Crafting 5 | **original** (adapted to our economy) |

### 4.7 Drugs (MED-07)

| Constant | Value | Page |
|---|---|---|
| Hard drug overdose | +180-350 per dose; decay 1000 per day | Overdose |
| Stages | 500 (consciousness -500), 750 (max 100), 1000 death | Overdose |
| Prophyl tablet | +80-140 per dose, lasts 5 d | Penoxycyline |
| Numbroot tincture | pain x0.5 for 12 h; auto-use at pain >= 400 | **original** |

### 4.8 Aging (MED-08)

| Constant | Value | Page |
|---|---|---|
| Bad back | age 41; moving -300, manipulation -100 | Ailments |
| Cataract | 49; eye -500 | Ailments |
| Frail | 51; moving and manipulation -300 | Ailments |
| Dementia | 69; consciousness -150, talking and hearing -250 | Ailments |
| Hearing loss | 49; ear -500 | Ailments |
| Artery blockage | 21; +0.7/day x 0.5-3; stages 200/400/600/900 at -50/-100/-150/-350/-600 heart efficiency; heart-attack MTB 300/200/100/60/30 d | Ailments |
| Carcinoma | 23; starts 300, +3/day x 0.45-1.65; excision needs 4 industrial medicine, Medical 10, 25 % death on failure | Ailments |
| Heart attack | starts 400; random -400..+600 per interval; 600 -> consciousness max 100; 1000 -> death | Ailments |
| Onset curve between onset age and cap | linear to a cap | **original** |

### 4.9 Care levels (MED-03)

| Level | Allows | Page |
|---|---|---|
| No care | rescue only | Doctoring |
| Care, no medicine | tend without medicine | Doctoring |
| Herbal or less | herbal and below | Doctoring |
| Industrial or less | industrial and below | Doctoring |
| Best available | any including nanomed | Doctoring |
| Defaults | colonists Best; prisoners and guests Herbal; animals decided in P3 | Doctoring |

### 4.10 Known conflicts and open checks

| Item | Conflict | Resolution |
|---|---|---|
| Infection chance by wound kind | The Injury page and the Infection page disagree on per-kind base chances. | The table above follows the Infection page. Re-check at the start of MED-02 and note the choice in the handoff. |
| Bleed kind factor | The Injury "bleed rate" table conflicts with the Ailments page. | Verify at the start of MED-01 and record the chosen per-kind factors. |
| Organ harvest medicine count | Pages disagree on how many medicine units a harvest consumes. | Use the 2-unit value above, flag it **original** if not resolved at MED-06 start. |
| Age field | Whether `pawngen.Pawn` exposes age. | Confirm at MED-08 start; add the field if missing. |

## 5. Naming map

| RimWorld term | Ours |
|---|---|
| glitterworld medicine | nanomed kit |
| bionic arm | servo arm |
| bionic leg | servo leg |
| bionic eye | optic implant |
| bionic ear | cochlear implant |
| bionic spine | spinal brace |
| bionic heart | pump heart |
| bionic stomach | synth stomach |
| prosthetic heart | valve heart |
| penoxycyline | prophyl tablet |
| pain drug (generic painkiller) | numbroot tincture |
| medical care level / "medical care" | care policy |
| vitals monitor | vitals monitor (generic term; kept) |

Real-world names kept: flu, plague, malaria, sleeping sickness, gut worms, muscle parasites, cataract, dementia, carcinoma, heart attack. Existing original items stay as they are (herbal and industrial medicine, peg leg, prosthetic leg/arm, wooden hand, denture).

## 6. Phase plan

Order: P0 alone, then (P1 || P3 || P5-domain), then P2, then P4. P6 after P5; P7 || P6. P8 after P4 + P5. P9 last. Run `bin/benchmark` after P1, P2, and P4.

| Phase | Depends on | Checklist items moved |
|---|---|---|
| MED-00 contract and saves | none | none (groundwork; handoff `MED-00-contract.md`) |
| MED-01 bleeding, blood, pain, downed | P0 | Vanilla 969 [x], 963 [x], 977 [x] (stale "no rescue" text fixed), 973 updated; UI 655 [x], 660 [x], 652 [-] |
| MED-02 tend quality, healing, scars, infection | P1 | Vanilla 1020 [x], 1002, 991, 1018 updated; UI 680 [x], 702/704 [x], 673 [-] |
| MED-03 care policy, doctor priority | P0 | Vanilla 996/2112 [x], 1378 [x]; UI 685-688 [x], 698 [x], 801 [x], 1016-1019 [x]/[-] |
| MED-04 diseases | P2 | Vanilla 1028/1034/1999/2198 [x], 2850 [-] until P7; UI 673 [x] |
| MED-05 anatomy, capacities, bionics | P0 (domain parts parallel with P1) | Vanilla 930 [-] (species deferred), 940/949 updated, 1070 [x] (archotech noted deferred) |
| MED-06 surgery v2 | P5 | Vanilla 1055/1057/1059/1065/1713 [x]/[-]; UI 693 [x], 1131 [x], 706 [x] |
| MED-07 drug health effects | P4 (parallel with P6) | Vanilla 2047 [x], 2055 updated, 2850 [x] |
| MED-08 aging, chronic conditions | P4 + P5 | Vanilla 1046/1049 [x] |
| MED-09 animals | all | UI 1076 [x]; vanilla updated |

(Line numbers are checklist-file line references from the plan; confirm against the current checklist text at phase start.)

### MED-00 contract and save groundwork (lead, sequential)

Extend the ENG-003 line in `AGENTS.md`. Change `Body` to `{Wounds; Missing; Fitted []item.Kind; BloodLoss int32; Conds []Condition}` with a lazily grown `Implant(p)` accessor; `savedBody` keeps `Implants [20]` as a legacy read used only when `Fitted == nil`. `Wound` gains zero-safe `TendQ int16`, `TendAt`, `InfectAt uint64`, `Bleed int32`. `drug.State.Tolerance/LastDose` become slices with a legacy conversion. Add `body/condition.go` (`CondKind`, `Condition`, `CondDef`, `Stage`, `AddCond`, `Cond`, `DefOfCond`); `Capacity()` and `Pain()` fold in stage mods. Add `game/patients.go` (sorted patients index, rebuilt on load). Append command stubs `SetMedicalCare`, `SetDefaultCare`, `SetSelfTend`, `EmergencyTend`, `ReorderOperation`; add `OpKind`s `HarvestOrgan`, `InstallOrgan`, `Euthanize`, `Anesthetize`, `ExciseCarcinoma`, `NumOps` and fix the bound at `surgery.go:130`. Add view stubs on `HealthView`, `OperationView`, `PawnView`. Fix item values (`value.go`). Add `streamMedical`. Tests: `TestLegacyBodySaveKeepsImplants`, `TestPatientsIndexMatchesScan`, same-seed hash parity. Commit `feat(med): body save slices, conditions, patients index (MED-00)` plus this doc.

### MED-01 bleeding, blood loss, pain, downed

Domain `body/bleed.go`: `BleedRate`, `BleedPerDay`, `StepBlood`, `BloodStage` with 5-point exit hysteresis. `capacity.go`: pain 12.5 per HP; consciousness = brain - pain/2 - blood stage, capped 100 at loss >= 600. `health.go`: append `CauseBloodLoss`; downed from capacities with the exit band. Application: new `game/bleed.go`; rename `phaseInfection` to `phaseMedical` (const, `Phases()`, `stepObserved` table, `game.go:982`); hourly loop over the patients index; delete the raider `bleedOut` timer (`raids.go:75,394`) and the raider early-return (`game/infection.go:40`); `stains.go:188` drips from `BleedPerDay() > 0`; `assignDoctor` sorts by bleed. UI: state line, blood-loss row, bleed tooltips, "Bleeding" status, `bleeding` alert. Tests: `TestBloodStageTable`, `TestBloodLossDoesNotFlap`, `TestUntendedCutBleedsOut`, `TestTendStopsBleeding`, `TestRaiderBleedsOutLikeColonist`, `TestDownedHysteresis`, `TestBleedingAlert`. Re-pin golden hashes. Handoff `MED-01-bleeding.md`.

### MED-02 tend quality, healing, scars, infection vs immunity

Domain `body/tend.go` (`TendInput`, `TendQuality`, `Potency`, `TendAll`; `Heal` uses wiki rates; new scar odds). Rewrite `infection/` as the shared race engine (`StepRace`, `WoundChance`); infection becomes `CondInfection` per part; delete `g.wounds` (save field stays as legacy read). `doTend` computes quality via `streamMedical`; remove `tendHealBy`/`doctorInfectionPct`; hospital-bed constants become bed/lying offsets; room factor via rooms index and filth; infection roll at `InfectAt`. Save: `SaveVersion = 6`, `migrateSave` converts legacy infections. UI: condition rows with severity and immunity bars and "needs tend in Xh", per-wound tend quality tooltip, `infection` and `untended` alerts. Tests: `TestTendQualityFormula`, `TestHigherTendQualityWinsMoreRaces`, `TestScarOddsFollowDamage`, `TestOldSaveInfectionMigrates`, `TestOneTendPerWindow`, `TestPrisonerWoundsCanInfect`; re-pin hashes. Handoff `MED-02-tend-immunity.md`.

### MED-03 care policy, doctor priority, emergency tend, reorder

Domain `pawn/medcare` (`Policy`, `Allows`, `MaxMedicine`, `Label`). Application `game/care.go` (`careState` with nil save when defaults); commands for policy, default, self-tend, reorder; `withMedicine` capped by policy; `NoCare` skips patients except rescue; doctor chain rescue > bleeding > other tend > surgery > animals, one `assignXxx` each; new job `job.EmergencyTend` (area-exempt, shown in `action_view.go`). UI: five care icons (art skill), self-tend checkbox, default-care selectors on the policies screen (load that spec section via `rimworld-go-ui-screens`), drafted right-click "Tend <name>", reorder buttons. Tests: `TestCarePolicyCapsMedicine`, `TestNoCareSkipsTend`, `TestDoctorPriorityOrder`, `TestEmergencyTendWhileDrafted`, `TestDoctorDoesNotFlapBetweenPatients`, `TestReorderOperation`, `TestCareSurvivesSave`, care-selector UI test. Handoff `MED-03-care-policy.md`.

### MED-04 diseases

Domain `pawn/disease` with six `CondKind`s and `CondDef` rows from 4.4 (parasites: `CureTendSum 3000`, `NoImmunity`). Incident IDs appended (Nature category, biome limits, MinDay 15, original weights). Application `game/disease.go`: `incidentFits`/`fireIncident` cases, targets drawn with `streamMedical`, frequency x `DiseaseFreq` (0 disables), race stepping in `phaseMedical`, gut worms raise hunger, parasites raise rest fall; `needsBedRest` adds severe disease with the existing 2 h minimum. UI: reuse condition rows, outbreak letter, `disease` alert, remove "No diseases yet" strings (`screen_difficulty.go:125,129`, `story_tab.go:302,306`). Tests: `TestDiseaseOutcomeCurves`, `TestParasitesCureByTendSum`, `TestDiseaseFreqZeroNoIncidents`, `TestDiseaseBedRestDoesNotFlap`, `TestSleepingSicknessTropicalOnly`. Handoff `MED-04-diseases.md`.

### MED-05 anatomy, capacities, bionics

Append `KidneyL, KidneyR, Liver, Spine` (24 parts) with coverage re-split (`TestCoverageSumsTo1000`). Append capacities `BloodPumping`, `BloodFiltration`, `Digestion` (grow `capacityLabels`, `game/body.go:323`); zero pumping/filtration/digestion kills (`CauseOrganFailure`). New items: nanomed kit, servo arm/leg, optic and cochlear implants, spinal brace, pump heart, synth stomach, valve heart, and natural organs. Fix prosthetic arm to 500 and wooden hand to 600. Research `ServoReplacements`; recipes adapted to our economy; nanomed kit by trade/loot only. Art through `rimworld-go-art-assets`. Tests: `TestKidneyLossKillsWhenBoth`, `TestServoArmAbove100`, `TestProstheticArmEfficiencyFixed`, `TestLegacyBodyHasNewOrgans`, `TestCategoryOtherIsDeliberate`, recipe/research gating. Handoff `MED-05-anatomy-bionics.md`.

### MED-06 surgery v2

Domain `SurgeryInput`/`SurgeryChance` returning factor list for tooltips; four outcomes; per-op death on failure. Application: `assignSurgery` covers colonists, then prisoners, then visitors; ops consume medicine via the policy-capped fetch; `CondAnesthetic` replaces `anaesthTicks`; harvest spawns organs, install uses them; euthanize with new cause and execution-style thoughts; new mood entries; goodwill penalty only if the faction API exists. Vitals monitor building (power 80 W, adjacency via cached bed-to-monitor lookup). UI: chance with breakdown tooltip, new picker options with blocked reasons, prisoner/guest Health tab queues operations. Tests: `TestSurgeryChanceTable`, `TestCleanlinessFactor`, `TestMonitorNeedsPowerAndAdjacency`, `TestPrisonerGetsOperated`, `TestHarvestYieldsOrgan`, `TestEuthanasiaCause`, `TestAnestheticStages`, `TestSurgeryConsumesMedicine`, `TestOldSurgerySaveLoads`. Handoff `MED-06-surgery.md`.

### MED-07 drug health effects

Append `NumbrootTincture` and `ProphylTablet` to `drug.go`; `Overdose` condition with stages (spark tablet counts as hard); pain offset in `Pain()`; policy auto-use of painkiller at pain >= 400, re-eligible only after the effect ends and pain returns (hysteresis); prophyl every 5 d; items, recipes (Pharmacology), art. Tests: `TestOverdoseStagesKill`, `TestPainkillerHalvesPain`, `TestProphylBlocksMalaria`, `TestPainkillerUseDoesNotFlap`. Handoff `MED-07-drug-health.md`.

### MED-08 aging and chronic conditions

New `body/aging.go` using pawn age. Daily 00:00 roll over `g.pawnIDs` with `streamMedical`; onset zero before wiki age, then linear to a cap. Conditions per 4.8; carcinoma removed by `ExciseCarcinoma`; cataract cured by eye replacement; spinal brace cures bad back; pump/valve heart prevent heart attacks; dementia wander via the existing hook if present. UI rows; show age in bio/inspector if missing. Tests: `TestNoChronicBeforeOnsetAge`, `TestHeartAttackDeterministic`, `TestPumpHeartPreventsHeartAttack`, `TestExciseCarcinoma`. Handoff `MED-08-aging.md`.

### MED-09 animals

`vet_animals.go` gets an adapter with one virtual part: scalar wound list on a lightweight body with blood loss, tend quality, infection, and an animal care policy. Per-species anatomy stays deferred. Tests: `TestAnimalBleedsAndIsTended`, `TestAnimalCarePolicy`. Handoff `MED-09-animals.md`.

## 7. Deferred and out of scope

| Item | Reason / where noted |
|---|---|
| Archotech parts | Not in scope; noted as deferred in checklists (vanilla 1070). |
| Ribs and fingers | Part granularity kept at 24; noted in checklists. |
| Per-species animal anatomy | Animals get the scalar adapter in P9; checklist 930 stays [-]. |
| Pregnancy | Not part of this effort. |
| Faction goodwill penalty on harvest | Only if a faction goodwill API exists at P6; else noted deferred. |
| Dementia wander | Only if a mental-break/wander hook exists; else stat effects only. |
| Wiki prose, art, names | Never copied (section 1). |
