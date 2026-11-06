# MED handoffs

Families: MED, AIL. Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## AIL-1-stuck-downed
No exposure state holds a colonist down forever · 2026-11-06 · 0e753dc06
- Changed: a heatstroke or hypothermia victim lying in a bed whose air does not heal them (a band where severity neither builds nor fades, wiki Ailments) is now rescued: carried to a bed whose air heals with 1 °C margin (`healMargin`), else to the nearest healing spot (`warmSpot`), else waits `ExposureRetry` (1 h). Every exposure move holds the patient 1 h. Prison beds ignore air. "Rescue" is not disabled for "no bed" when a spot exists.
- Changed: a downed pawn re-checks standing every climate interval (AIL-002). Before, standing was checked only when an exposure stage changed.
- Changed: `exposure.Held`/`Note`, `Bands.Inset`; `care.Reach` embeds a `Climate` role (`AirHeals`, `HealingSpot`, `ExposureWait`, `SetExposureWait`; wait is the saved `noComfort` map). No `Care` methods added (Care stays at 40). Root package +12 net lines.
- UI: `PawnView.ExposureNote` ("Not recovering: air 33.4 °C, needs 29.2 °C or cooler", via `inspector/temp_ui.go`) and a "Not recovering" danger alert (`hud/alerts_exposure.go`).
- Decisions: exposure rescue starts from the trivial stage (4 %). RSC-201 (rescuer's own temperature rule) unchanged. `TestGrownGolden` unchanged. No-flap: `TestExposureRescueDoesNotFlap`; scenario `TestHeatstrokeVictimInHotBedIsCarriedToACoolRoomAndStands`.
- Open: AIL-006 (permanent capacity inside the stand-up band) and AIL-007 (exposure off the map). Inspector temperature lines are Celsius only. A medical bed held for the stay stays held while the patient is on a spot (freed on standing). Prisoners get no `ExposureNote` (`prisoners.go` not owned).

## AIL-2-open-air
Heatstroke victims are carried into cool open air · 2026-11-06 · 545218548
- Changed: `thermo.Spot` also searches open-air cells when the caller finds `room.Outdoors` bearable (`outdoorSpots`: Chebyshev rings to `outReach` 64 cells, stops once the twelve nearest are certain; no whole-map scan). `Model.Air(r, out)` returns room or outdoor air. `warmSpot` reads it.
- Decisions: bed choice unchanged; a bed in a healing room still wins, open air is a spot not a bed. Reason: with every room above comfort max, AIL-1's fallback never saw the cool outdoors and victims lay 30+ h.
- Tests: `TestHeatstrokeVictimWithNoCoolRoomIsCarriedIntoTheOpenAir` (carried out once, not back for 2 h), `TestSpotMatchesFullScan`, `TestAirIsTheRoomsOrTheOpenAirs`.
- UI: none new; AIL-1's note and alert clear once outside.

## MED-00-contract
Medical overhaul contract and save groundwork · 2026-10-26 · acd15b559
- Changed: no behaviour change. Wiki constants in `.agents/gaps/MEDICAL_GAP_ANALYSIS.md`; ENG-003 wiki-numbers exception now covers MED-*.
- Changed: `Body.Implants [NumParts]` became `Fitted []item.Kind` (`Implant(p)`/`SetImplant`/`FittedFrom`); saves use `savedBody`, which reads legacy `Implants [20]` only when `Fitted` is nil. `Body` gains `BloodLoss`, `Conds`; `Wound` gains `TendQ`, `TendAt`, `InfectAt`, `Bleed`. Drug `Tolerance`/`LastDose` became slices (`savedDrug` keeps legacy arrays; `ToleranceOf`, `LastDoseOf`). All zero-safe.
- Changed: conditions (`body/condition.go`, `cond_kinds.go`): fixed `[NumCondKinds]CondDef` from per-phase `CondEntry` tables in `condTables`, no `init()`, no maps; `Capacity()`/`Pain()` fold stage modifiers. To add a kind: append before `NumCondKinds`, add `cond_<phase>.go`, list it in `condTables`.
- Changed: patients index (`game/patients.go`): sorted ID set from `bodyChanged`, cleared by the hourly sweep, rebuilt on load, not saved. RNG stream `streamMedical`.
- Changed: commands `SetMedicalCare`, `SetDefaultCare`, `SetSelfTend`, `EmergencyTend` (stubs until MED-03), `ReorderOperation` (real, ±1). New `OpKind`s `HarvestOrgan`, `InstallOrgan`, `Euthanize`, `Anesthetize`, `ExciseCarcinoma`, `NumOps`; `CanOperate` rejects them until MED-06. Views: `HealthView` (State, BloodLossPct, BleedPerDay, DeathInHours -1, Conditions, Care, SelfTend), `OperationView` (ChancePct, ChanceTip), `PawnView` (Bleeding, CareLevel).
- Decisions: value.go prices ProstheticFoot 90, GlassEye 40, WoodenNose 5 (original tuning). `SaveVersion` stays 5; all 6 `TestScaleGolden` pins, `TestStateHashStable`, `TestTemperateGenerationUnchanged` unchanged.
- UI: none yet; view fields are stubs filled from MED-01.

## MED-01-bleeding
Bleeding, blood loss, pain, downed · 2026-10-26 · ad3f6a49b
- Changed: bleeding in ‰ blood/day (`body/bleed.go`, wiki Injury/Ailments): cut/bite/scratch/surgical 60‰ per HP; bruise, burn, tended, scar 0; heart wound ×5; fresh stump 2× MaxHP until tended. Blood loss stepped hourly over the patients index; regenerates 333‰/day only while nothing bleeds. Stages enter 150/300/450/600, exit 50 lower; consciousness −100/−200/−400/−400, capped 100 from 600; death at 1000 (`CauseBloodLoss`, "of blood loss").
- Changed: pain 12.5‰ per HP (scars 1‰); shock at 800 scaled by pain tolerance. Downed via `health.BodyDowns`/`BodyRecovered`: down at consciousness < 300, moving < 150 or pain ≥ shock; up at consciousness ≥ 350, moving ≥ 200 and pain ≤ shock − 50 (hysteresis). Bodiless pawns (animals) kept old thresholds until MED-09.
- Changed: raiders and prisoners lose the 4 h bleed-out timer and bleed for real; a recovered downed raider flees. Doctor tends fastest bleeder, then worst hurt, then nearest. Stains drip only while bleeding. `phaseInfection` renamed `phaseMedical`.
- Changed: `Body.BloodStage`, `Body.Fresh` (saved via `savedBody`); `PartView.Bleed`, `PawnView.BleedDeathHours`.
- UI: Health tab state line, blood-loss bar, bleed rate in part tooltips; status "Bleeding (death in Nh)"; alert `bleeding` (danger at ≤ 12 h to death).
- Limits: golden re-pins (48x40/s1, 48x40/s42, 120x120/s7; others and biome pins unchanged). TickRaid/500 0.278 → 0.345 ms/tick (raiders live longer); ColonyHour 18.3 ms unchanged. No-flap: `TestBloodLossDoesNotFlap`, `TestDownedHysteresis`.
- Open: infection for raiders and prisoners (done MED-02); `Wound.Bleed` cache unused.

## MED-02-tend-immunity
Tend quality, healing, scars, infection vs immunity · 2026-10-26 · 4e51c06ad
- Changed: tend quality (`body/tend.go`): 200 + 100/Medicine level to 8 (1000), then wiki curve (18 → 1500, 20 → 1550); × manipulation (cap 1400), sight 70 %, medicine potency, hospital bed +100, self-tend ×0.7, roll 750–1250; clamped to medicine cap (none 300/700, herbal 600/700, industrial 1000/1000). Nanomed hook for MED-05.
- Changed: healing per wound/day: 8 HP base; tended +4 + 0.008×quality; lying +4 ground, +8 bed, +14 hospital bed (RimWorld pace, hourly over the patients index). Scar odds 20‰ × clamp((dmg−4)/10).
- Changed: infection is `CondInfection`, one per wound, on the shared race engine (`infection.StepRace`/`WoundChance`/`Losing`/`HoursToDeath`): rolled 6–18 h after the wound; base bite/burn 300‰, other bleeding 150‰, bruise 0; × tend factor (850 → 50‰ by quality) × room factor (500 clean roofed floored room, else 1000). Severity +840/day, tending cuts up to 530/day; immune −700/day; immunity +644/day; tend every 12 h; stages 330/780/870; death 1000. Raiders and prisoners can be infected. Removed `g.wounds`, `tendHealBy`, `doctorInfectionPct`; `story.go` uses permille `infectionChance`.
- Decisions (original tuning, no wiki number): infection onset severity 10; clean room from 750‰ roofed; 20-point stage-exit band; animal tend heal `TendHeal × (250+q)/500` until MED-09.
- UI: Health-tab conditions section (severity bar red, immunity bar green, "needs tend in Nh"/"now"), tend quality in part tooltips; status "Infection: losing/winning (N%)"; alerts `infection`, `untended` (≥ 4 h).
- Save: `SaveVersion = 6`; migration `v < 6` turns a legacy pawn infection into `CondInfection` on the worst open wound with 0 immunity; `Wounds` read-only. Golden re-pins for the three runs. No-flap: `TestOneTendPerWindow`.
- Open: UI row 702 (capacity tooltip) was open here, done in MED-10.

## MED-03-care-policy
Medical care policy, doctor work order, emergency tend · 2026-10-26 · 4f1c88af1
- Changed: care levels (`domain/pawn/medcare`): No care, Care with no medicine, Herbal or less, Industrial or less, Best available. Defaults (wiki Doctoring): colonists Best; prisoners, guests, animals Herbal or less. `withMedicine` caps tending and surgery medicine. No care: rescued, never tended; animals on No care skipped by the vet.
- Changed: self-tend per-colonist toggle, on by default (`selfTendDue` respects it). Doctor order: rescue > tend bleeding > other tends (then self-tend) > surgery > animals > bills; ties: bleeding, injury, distance, id; patient reserved until the tend ends (no flapping, `TestDoctorDoesNotFlapBetweenPatients`).
- Changed: `job.EmergencyTend`: a drafted pawn tends the target with the best allowed medicine within 15 tiles (original tuning); area-exempt; clears on undraft or target gone.
- Changed: `SetMedicalCare`, `SetDefaultCare`, `SetSelfTend`, `EmergencyTend` real. Save block `Care *careSave`, nil while default; no version bump. `TestCareStubsChangeNothing` retired.
- UI: Health header five care buttons + Self-tend checkbox; reorder arrows on queued operations; Assign screen Medical column + "Default medical care" row; drafted right-click "Tend <name>".
- Open: care icons (UI row 1019 `[-]`; drawn after ART-026). Golden hashes re-pinned together with MED-05.

## MED-04-diseases
Diseases and outbreaks · 2026-10-26 · 2feec5ec6
- Changed: disease conditions (`body/cond_disease.go`, `game/disease.go`, wiki numbers, hourly medical phase, `CauseDisease`): severity/day unimmune/immune, immunity/day, tend max, tend every, stages. Flu +249/−495, +239, 77, 12 h, 666/833. Plague +666/−333, +522, 363, 15 h, 600/800/910. Malaria +370/−730, +315, 232, 12 h, 780/910. Sleeping sickness +120/−176, +110, 70, 32 h, 437/625/875/937.
- Changed: gut worms (pain +200, hunger ×2, vomiting) and muscle parasites (pain +200, rest fall ×2, moving/manipulation −300) build no immunity; cured when tend quality sums to 3000, one tend per 48 h. Untended death: flu 99 h, plague 36 h, malaria 66 h, sleeping sickness 198 h; all beaten at 70 % tend quality.
- Changed: six Nature incidents from day 15; weight/cooldown original tuning: flu 6/20, plague 2/40, malaria 4/25 (warm), sleeping sickness 3/40 (tropical only), parasites 4/25 each (not ice). Scaled by Disease frequency, 0 = none (`PickAt`/`TotalAt`). Victims on `streamMedical`: one colonist, or 1..min(n,6) for flu/plague/malaria. Synth stomach blocks gut worms.
- Decisions: bed rest starts at major stage or when a tend is due, ends only when immune or cured (hysteresis, `TestDiseaseBedRestDoesNotFlap`). Vomit leaves a dirt splash; sick pawns get the existing "sick" thought.
- UI: headline "Flu: losing/winning" / "Gut worms: needs tending"; parasite tooltip tend progress to 300 %; caution letter naming victims; `disease` alert; difficulty screens describe Disease frequency.
- Limits: goldens unchanged (no day-15 roll in pinned runs).
- Open: parasites use the existing −8 "sick" thought, not the wiki −5; malaria extreme stage caps consciousness at 10 % (wiki unsure); rates step in whole permille per hour; food poisoning/toxic buildup (food poisoning done MED-11), disease on caravans (done MED-11).

## MED-05-anatomy-bionics
Organs, capacities, bionic tier · 2026-10-26 · 4f1c88af1
- Changed: parts KidneyL/R 50 HP (coverage 17 each), Liver 50 (25), Spine 40 (25); torso 380 → 296 so total stays 1000. v5/v6 bodies load with new organs at full HP. `body.Inner(p)` marks internal organs: never amputated, harvested in MED-06.
- Changed: capacities: blood pumping (heart) 0.2 on consciousness/moving, scales bleeding downward only; filtration (kidneys × liver) 0.1 on consciousness (one kidney −50), 0.5 on immunity gain (one kidney 750, feeds MED-02); digestion (stomach × liver); spine multiplies moving; servo limbs keep bonus above 100 %.
- Changed: pumping, filtration, breathing or digestion at 0 is organ failure (`CauseOrganFailure`, "of organ failure"). A hit never destroys the last life organ; hourly phase catches organ failure otherwise.
- Changed: items (wiki): nanomed kit potency 1600/max quality 1300 value 50; servo arm/leg, optic implant 1250 value 1030; cochlear, synth stomach 1250 value 785; spinal brace 1000 value 1030; pump heart 1250 value 1030; valve heart 800 value 230; kidney/lung/heart/liver 900/1000/1200/1200. Prosthetic arm 500, wooden hand 600. Helpers `HeartAttackImmune`, `GutWormImmune`, `SpineBraced`; artificial heart can't be removed.
- Changed: `ServoReplacements` research (6000, Prosthetics + Fabrication); servo recipes Crafting 8 (components); valve heart Crafting 5 (steel 25); nanomed kit only from vaults (4 % of rolls). Care "Best available" allows nanomed (`MaxMedicine(Best)`); severe/infected patients try nanomed, industrial, herbal; others herbal first. Nanomed kits and organs haul to goods stockpiles.
- UI: new capacity rows and organ part rows in the Health tab. Art: ART-026.
- Open: capacity column now 11 rows, drug lines may clip on short panels. Golden re-pins (48x40/s42, 120x120/s7, grown scenario) together with MED-03.

## MED-06-surgery
Surgery v2, organs, euthanasia, anesthetic, vitals monitor · 2026-10-26 · 2feec5ec6
- Changed: `body.SurgeryChance` returns factors for the tooltip; base by Medicine level 100…1100, × manipulation, × min(sight,1000), hospital bed 1150, cleanliness 600…1018 (outdoors 850), light 750 dark/full from 50 %, vitals monitor 1050, medicine (none 700, herbal 1000, industrial 1100, nanomed 1200), operation (install 1000, remove 1200, harvest 1200, anesthetize 2000, euthanize 5000, excise 1000); cap 980. Cleanliness curve, medicine factors, filth mapping are original tuning.
- Changed: outcomes Success, MinorFail, Catastrophic, Ridiculous (2–4 wounds; 10 % of failures under Medicine 6, else 2%, original). Failed heart install or carcinoma excision kills 25 %; failed spinal brace install destroys the spine.
- Changed: `CondAnesthetic`: sedated 0–6 h (consciousness max 10), woozy to 12 h, wearing off to 20 h; replaces the 1 h downed state (old-save entries still wake). Medicine use within care level: install/remove/harvest/amputate 2 herbal+; anesthetize/euthanize 1; excise 4 industrial and Medicine 10.
- Changed: harvest drops the organ item (life organ kills, `CauseOrganFailure`); install into a missing/damaged inner part or replaces an artificial heart; euthanasia always kills (`CauseEuthanasia`). Mood: donor −30 for 7 days; colonists −5 (−6 if a colonist's organ), stacking 5 at 75 %; faction goodwill −80 (−100 heart). Operated order: colonists, prisoners, guests.
- Changed: vitals monitor (`construction.VitalsMonitor`): 80 W; powered and touching a hospital bed: tend +70, surgery ×1050, immunity +20; bed lookup cached on `structVer`.
- UI: queued operations show chance with factor tooltip; picker adds harvest/install/euthanize/anesthetize with blocked reasons; prisoner and guest Health tabs queue operations; headline "Under anesthetic"; care buttons draw ART-026 icons. Goldens unchanged.
- Open: excise carcinoma hidden until MED-08 (done); guests are not tended by doctors (predates MED-06); implant medicine taken from its reserved stack where it lies.

## MED-07-drug-health
Drug health effects (painkiller, prophylaxis, overdose) · 2026-10-26 · f76419b43
- Changed: `CondOverdose` (wiki): dose adds severity (spark tablet +180..350, prophyl +80..140), decays 1000/day; stage 500: consciousness −500 + vomiting (mean 4.8 h); 750: consciousness capped 100, kills mean 72 h; 1000 death (`CauseOverdose`). Rolls on `streamMedical`.
- Changed: numbroot tincture (original painkiller): pain ×0.5 for 12 h, no addiction/tolerance/overdose, value 16, from 1 herbal medicine. Prophyl tablet (wiki Penoxycyline): protects against plague, malaria, sleeping sickness for 5 days (MED-04 targeting skips protected pawns), value 18, 2 tablets from 1 industrial medicine, Crafting 4, behind Pharmacology.
- Decisions: auto-use under every policy except "no drugs": painkiller at pain ≥ 400 only when no dose is acting (retaken after the 12 h ends and pain still ≥ 400: hysteresis, `TestPainkillerUseDoesNotFlap`); prophyl once 5 days since last dose, checked hourly; both only on the idle path (bed rest won't take a painkiller). Lead fix in `tick.go phaseNeeds`: health record re-read after `drugTick` so an overdose death isn't undone.
- UI: drug lines "numbroot: pain halved", "protected by prophyl" with bars; overdose/numbroot condition rows; `overdose` alert (warning at 500, danger 750). Art: `item/numbroot_tincture`, `item/prophyl_tablet` + icons.
- Limits: scale goldens unchanged.

## MED-08-aging
Aging and chronic conditions · 2026-10-26 · f76419b43
- Changed (`body/cond_aging.go`, wiki Ailments, onset age: effect): bad back 41: moving −300, manipulation −100 (cured by spinal brace); cataract per eye 49: sight −250 (eye replaced); frail 51: moving/manipulation −300; dementia 69: consciousness −150, talking/hearing −250, 4 h confused wander MTB 5 days; hearing loss per ear 49: −250 (ear replaced); artery blockage 21: +0.7‰/day ×0.5–3, 5 stages, pumping −50…−600, heart attack MTB 300/200/100/60/30 days (artificial or new heart cures); carcinoma 23: starts 300, grows/holds/remits, pain 100…600, at 1000 destroys its part or kills if vital (excision cures). A part's wiki −500 becomes −250 on the capacity (each capacity averages two parts).
- Changed: heart attack starts 400, ±(−400..+600)/h; painful stage consciousness −500; from 600 consciousness ≤ 100; 1000 death (`CauseHeartAttack`). Doctor tends hourly: 65 % × quality chance to cut 300; untended ~76 % die. Pump/valve heart blocks artery blockage and heart attacks.
- Decisions: daily roll at 00:00 over sorted living colonists on `streamMedical`; onset chance (original tuning) min(Cap, Base + Slope × years past onset) per year over 60 days. Growth rates and carcinoma grow/stable/remit split come from a hash of seed, pawn, condition: nothing new saved. Excise carcinoma now allowed; replacing a part cures its condition. Epitaphs "of a heart attack", "of a carcinoma", "of an overdose".
- UI: Health rows ("Bad back", "Cataract (left eye)", "Carcinoma (stomach), growing", "Heart attack (painful)") with how-it-ends tooltips; headlines "Heart attack: needs tending", "Carcinoma". Goldens unchanged (pinned runs never reach midnight).
- Open: carcinoma part-efficiency loss not modelled (pain is); wiki's 30 % painless carcinomas and carcinoma tending left out; "Biological aging" beyond these stays `[-]`.

## MED-09-animals
Animal medicine · 2026-10-26 · f76419b43
- Changed: body adapter (`game/animal_health.go`): a hurt animal gets the human body model with all wounds on Torso; created on first wound, dropped when nothing needs care. Reuses MED-01/02 bleeding, tend quality, `CondInfection`. Bleeding scales by species size (MaxHP vs 100, clamped 0.1×–10×, original); no scars (original). `Injury` stays the HP figure.
- Changed: blood loss and infection kill animals. Hourly step over a sorted animal patients index (rebuilt after load, not saved). Vet tends a tame animal with a body once per new wound or per 12 h infection window; tends stop bleeding, speed healing, lower infection odds; care level (per animal, else Animals default) caps medicine, No care skipped. Bodiless animals (old saves, starvation) keep old behaviour. Wild animals bleed and heal at wiki rates.
- UI: inspector shows bleeding with hours to death, blood loss, stage, infection vs immunity, "Needs tending"; Care command opens a five-level picker sending `SetMedicalCare`; Animals table "62%!" while bleeding/infected.
- Save: `animal.State.Body`, only when present. Scale goldens unchanged; grown scenario moved by the combat hook, re-pinned with MED-07/08.
- Open: per-species anatomy (MED-13); downed state (MED-12); stain drips for animals use half-HP rule; tame animals dying of wounds get the generic death message.

## MED-10-capacity-tooltip-monitor-gate
Capacity-breakdown tooltip, vitals monitor research gate · 2026-10-26 · 95b392308
- Changed: `body.CapacityFactors(c)` reuses surgery `Factor`: imperfect parts (naming implants), "Capped by consciousness", pump/filtration weights, pain share, blood-loss stage, condition stage offsets/caps, final "= NN%" equal to `Capacity()`. `CapacityView.Tip` carries it; each Health-tab capacity row has widget `health.cap.%d` with the tooltip (closes UI row 702).
- Changed: `build:vitals monitor` unlocked by Fabrication, next to the hospital bed (`TestVitalsMonitorNeedsFabrication`).

## MED-11-food-poisoning-caravans
Food poisoning, disease on caravans · 2026-10-26 · d6f64a943
- Changed (original tuning): `item.Stack.Poison` (‰, saved, zero-safe) set on cooked meals by `doBill`: base 3 + 47·(20−Cooking)²/400 ‰ (50 at 0, ~15 at 10, 3 at 20) × kitchen cleanliness (clean ×0.5, slightly dirty ×1, dirty ×1.8, filthy ×3, outdoors ×1.5); campfire meals ≥ 15 ‰; clamp 1–1000. Merged/split stacks keep a quantity-weighted chance (`MergePoison`; `splitLoad` copies). Eating rolls on `streamMedical` only when `Poison > 0`.
- Changed: `CondFoodPoisoning`, 24 h course: initial 4 h, major to 16 h (consciousness −200, moving/manipulation −100, pain +100, vomiting, "sick" thought, bed rest), recovering to 24 h; never lethal (`TestFoodPoisoningBedRestNoFlap`).
- Changed: outbreaks may pick caravan members (letter names the caravan); members keep stepping through the patients index; each hour the member with best Medicine (≥ 1) tends what is due (no bed, no medicine, self-tend ×0.7, care levels respected). All medical deaths go through `g.dieMedical` (uses `dieAway` on a caravan).
- UI: "Poison chance N%" on meals; Health-tab condition row (tooltip no longer claims immunity for race-free conditions); caravan rows list sick/hurt members with health tooltips. Goldens unchanged.
- Open: meals through a drying rack lose their poison chance.

## MED-12-downed-animals
Downed animals · 2026-10-26 · 0a1b591eb
- Decisions (original tuning): an animal goes down at 80 % HP lost or 450‰ blood loss; stands at ≤ 50 % and < 300‰ (`DownsNow`/`StandsUp`, hysteresis). The line sits past the 70 % break-off so a hurt animal flees first. `Downed`, `DownedTick` saved, zero-safe. Checked on hit (`animalHurt`), hourly body step, and the animal's own 60-tick decision (no all-animal scan).
- Changed: downed: `thinkAnimal` returns at once, `moveAnimal` refuses steps; food drains half, rest/healing at lying rate; downed manhunter stops hunting. Predators take a downed animal as prey whatever its size (`findPrey`); hunters take downed marked animals first (`assignHunt`). Taming chance doubled, cap 100 %. Vet tends it where it lies.
- UI: `AnimalView.Downed` (rest pose, grey tint); inspector "Downed"/"Wild | Downed"; table status; message "X the Y is down." Goldens unchanged. `TestHurtPredatorBreaksOff`, `TestTerritorialReactionDoesNotFlap` adjusted, intent kept.
- Open: rope jobs don't move a downed animal; a downed tame animal isn't fed.

## MED-13-animal-anatomy
Per-archetype animal anatomy · 2026-10-26 · afb74e18e
- Changed: replaces MED-09's one-part adapter. `Body` gains `Anatomy`, `Scale` (‰ of human HP; 0 = 1000), both zero for humans (human saves and hashes unchanged, `TestHumanlikeUnchanged`). Plans (`body/anatomy_plan.go`, original): Humanlike 24 parts, Quadruped 26, Bird 23 (wings serve no capacity), Snake 15 (vital spine moves it), Lizard 23, Insect 18; coverage sums 1000, ≤ 32 parts. Archetype map (`animal.AnatomyOf`): Quadruped/Primate/Brute → Quadruped (primates get no hands); Bird; Snake; Lizard/Tortoise → Lizard; Beetle/Crawler → Insect.
- Changed: part HP scales with species (`animalHealthScale`); capacities average limb chains and multiply `mul` parts; losing one leg: quadruped 75 %, bird 50 %, insect 83 %. Implants, replacements, operations and aging refuse non-humanlike bodies.
- Changed: `woundAnimalBody` picks a part by coverage on `streamMedical`; a part whose loss kills is never destroyed (damage spills on); limbs and eyes can be lost. Speed scales with Moving; Moving < 15 % downs, stands at 30 % (`TestAnimalMovingDownNoFlap`), on top of MED-12. Bodies with lost parts are kept; only bodies needing care are in the patient index. Load migration: old one-part bodies get their species plan, wounds/conditions move to the root part.
- UI: `AnimalView.Parts`, `AnimalView.MovingPct`; inspector "Moving 75%", "Torso: 12/28 HP, bleeding", "Front left leg: missing", up to 6 lines + "+N more".
- Limits: `TestGrownGolden` re-pinned `0x960cb818b7a41587` → `0xf2889f4a11cbdce0`; `TestScaleGolden` unchanged.
- Open: an animal that loses every leg stays down forever (no auto-euthanasia); no scars; butchering yield ignores lost parts; food poisoning/overdose/anaesthetic still name the human torso slot (no animal path creates them).
