# MED-06: surgery v2, organs, euthanasia, anesthetic, vitals monitor

Part of the RimWorld-style medical overhaul (`.cursor/MEDICAL_GAP_ANALYSIS.md`).
The numbers come from the wiki (Doctoring, Vitals monitor, Human resources)
under the ENG-003/MED exception.

## Behaviour
- **Success chance** (`body.SurgeryChance`, which returns factors for the tooltip). The base comes from the Medicine level, then is multiplied by the other factors and capped at 980:

  | Factor | Value |
  |---|---|
  | Base by Medicine level | 100 … 1100 |
  | Manipulation | × manipulation |
  | Sight | × min(sight, 1000) |
  | Bed | hospital 1150 |
  | Cleanliness | 600 … 1018; outdoors 850 |
  | Light | 750 in the dark, full from 50 % lit |
  | Vitals monitor | 1050 |
  | Medicine (original tuning) | none 700, herbal 1000, industrial 1100, nanomed 1200 |
  | Operation | install 1000, remove 1200, harvest 1200, anesthetize 2000, euthanize 5000, excise 1000 |

  The cleanliness curve and how filth maps to it are original tuning.
- **Outcomes**: Success, MinorFail, Catastrophic, Ridiculous.
  - Ridiculous means 2–4 random wounds. It is 10 % of failures under Medicine 6, otherwise 2 % (original tuning).
  - A failed heart install or carcinoma excision kills 25 % of the time.
  - A failed spinal brace install destroys the spine.
- **Anesthetic** (`CondAnesthetic`): sedated 0–6 h (consciousness max 10), woozy to 12 h, wearing off to 20 h. It replaces the old 1 h downed state; old-save entries still wake.
- **Medicine use**, always within the care level:
  - install, remove, harvest, amputate: 2 herbal or better;
  - anesthetize, euthanize: 1;
  - excise carcinoma: 4 industrial and Medicine 10.
- **Organ operations**:
  - Harvest drops the organ item. Taking a life organ kills (`CauseOrganFailure`).
  - Install organ goes into a missing or damaged inner part, or replaces an artificial heart.
  - Euthanasia always kills (`CauseEuthanasia`).
- **Mood and goodwill**: the donor gets −30 for 7 days; colonists get −5 (−6 if it was a colonist's organ), stacking 5 at 75 %. Faction goodwill: −80, or −100 for a heart (`faction.Adjust`).
- **Who is operated on**: colonists, then prisoners, then guests.
- **Vitals monitor** (`construction.VitalsMonitor`, sprite `furniture/vitals_monitor_<facing>`, icon `ui_icon/vitals_monitor`):
  - draws 80 W;
  - only when powered and touching a hospital bed: tend +70, surgery ×1050, immunity +20;
  - the bed-to-monitor lookup is cached on `structVer`;
  - no research gate yet.

## UI
- Queued operations show the chance (e.g. "72 %") with a factor tooltip.
- The picker adds harvest, install organ, euthanize and anesthetize, with blocked reasons.
- Prisoner and guest Health tabs can queue operations.
- A sedated patient's headline reads "Under anesthetic" (lead fix).
- The care-level buttons draw their ART-026 icons (lead fix).

## Verify
- `go test -race` over `internal/domain/...`, game and targeted ebiten tests passes; the lead's full suite ran too. `go vet` and gofmt are clean.
- New tests:
  - domain: `TestSurgeryChanceTable`, `TestCleanlinessFactor`, `TestAnestheticStages`
  - game: `TestMonitorNeedsPowerAndAdjacency`, `TestPrisonerGetsOperated`, `TestGuestGetsOperated`, `TestHarvestYieldsOrgan`, `TestHarvestLifeOrganKills`, `TestInstallOrgan`, `TestEuthanasiaCause`, `TestAnestheticWearsOffInGame`, `TestSurgeryConsumesMedicine`, `TestOldSurgerySaveLoads`, `TestHarvestMood`, `TestSurgeryCleanlinessFromRoom`, `TestSurgeryChanceView`, `TestHeadlineUnderAnesthetic`
  - UI: `TestHealthOperationChance`, `TestHealthPickerSurgeryOptions`, `TestHealthPrisonerQueuesOperations`, `TestArchVitalsMonitor`
- Goldens unchanged.

## Follow-ups
- Excise carcinoma stays hidden until MED-08 adds the condition.
- Guests are not tended by doctors (this predates MED-06).
- Implant medicine is taken from its reserved stack where it lies.
