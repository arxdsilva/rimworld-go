# MED-07: drug health effects (painkiller, prophylaxis, overdose)

Part of the RimWorld-style medical overhaul (`.cursor/MEDICAL_GAP_ANALYSIS.md`).

## Behaviour
- **Overdose** (`CondOverdose`, wiki: Overdose).
  - Each dose adds severity: spark tablet (the hard drug) +180..350, prophyl +80..140.
  - Severity decays 1000/day.
  - Stage 500: consciousness −500 and vomiting (mean 4.8 h).
  - Stage 750: consciousness capped at 100 (downed); kills with a mean of 72 h.
  - At 1000: death (`CauseOverdose`).
  - Rolls use `streamMedical`.
- **Numbroot tincture** (original painkiller; numbers are original tuning): pain ×0.5 for 12 h. No addiction, tolerance or overdose. Value 16. Made from 1 herbal medicine.
- **Prophyl tablet** (wiki: Penoxycyline): protects against plague, malaria and sleeping sickness for 5 days. MED-04's disease targeting skips protected pawns. Value 18. 2 tablets from 1 industrial medicine, Crafting 4, behind Pharmacology.
- **Auto-use** (every policy except "no drugs"):
  - Painkiller at pain ≥ 400 when no dose is still acting, so it is retaken only after the 12 h effect ends and pain is still ≥ 400 (hysteresis).
  - Prophyl once 5 days have passed since the last dose, checked once an hour.
  - Both run only on the idle path, so a colonist on bed rest won't take a painkiller.
- **Lead fix** (`tick.go` `phaseNeeds`): the health record is re-read after `drugTick`, so an overdose death can't be undone by the stale record.

## UI
- Drug lines: "numbroot: pain halved" and "protected by prophyl", each with a percent bar.
- Overdose and numbroot conditions appear as Health-tab rows.
- `overdose` alert: warning at stage 500, danger at 750.

## Art
- `item/numbroot_tincture`: an amber bottle with a root label.
- `item/prophyl_tablet`: a blister strip of pale-blue tablets.
- Their `ui_icon` cuts and normal maps. Rendered alone; no other sprite changed.

## Verify
- New tests:
  - `TestOverdoseStagesKill`
  - `TestOverdoseExtremeMayKill`
  - `TestOverdoseDecays`
  - `TestPainkillerHalvesPain`
  - `TestPainkillerUseDoesNotFlap`
  - `TestProphylBlocksMalaria`
  - `TestProphylRetakenEveryFiveDays`
  - `TestLegacyDrugSaveLoadsWithMedicalDrugs`
  - `TestOverdoseAlert`
- Domain and game `-race` runs pass, plus targeted ebiten tests (orphans and art coverage included). The validator reports 0 errors.
- Scale goldens are unchanged.
