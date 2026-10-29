# SIM-063: rescue fidelity, safe beds, guests, prisoners back to bed

Date: 2026-10-29. Programme SIM-061…066 (`.cursor/PAWN_INTERACTION_GAP_ANALYSIS.md`). Committed together with SIM-064…066 (shared contract files); see `SIM-066-mental-breaks.md` for the gates and the budget.

## What changed
- **Who is rescued** (S3, opus): every downed cause except collapse-asleep (`CauseNoBed`), including malnutrition, disease, blood loss, heat and cold, and anaesthesia out of bed. Rescue covers colonists and guests: visitors, traders, allies and faction-less strangers (S4).
- **Safe bed** (`care.SafeBedFor`): filters out a bed with fire within a tile or air beyond the wiki's 90 °C margin (`Reach.BedHazard`), any bed the patient can't claim or that is held for someone else, and prison beds for non-prisoners. A patient who needs care, or a guest, gets a medical hospital bed, then any medical bed, then a non-medical hospital bed. Anyone else gets its own bed, then any free bed. The patient's own claim is kept; another bed is held for the stay and released when it gets up. Captives use the same ranking over prison beds (`care.PrisonBedFor`).
- **Interruptions:** a carrier drafted, ordered elsewhere, fleeing or downed drops the patient. A patient that dies mid-carry is dropped through `Doctor.PatientDied`, hooked in `combat.go`'s `dieBy`. Each path has a test (`rescue_fidelity_test.go`).
- **Prisoners** (`care.PrisonBeds`): a warden carries a downed prisoner lying outside a bed back to a prison bed, autonomously and by "Take <name> to bed".
- **Guests** (`care.Hospitality`, S4):
  - A rescued guest is tended under the guest care group. When healed it leaves with +12 goodwill. A guest dying downed costs −5.
  - A faction-less stranger asks to join on recovery: 50 % when leaving is safe, 100 % when a hostile is on the map or the air is unsafe.
  - The letter has Accept (`command.AcceptLetter`, new) and Decline (`DismissLetter`). Accepting joins the roster.
  - `Game.DevJoinOffer` places a stranger with an offer, for developer mode and the journeys.
- **Bug fixed** (found by S4): `careRoster.IsColonist` counted visitors as colonists. They could take orders, and a rescued guest was never tended.
- **UI:**
  - The inspector shows "Resting in a medical bed", "On bed rest", and "Guest of <faction>" or "Stranger" (`PawnView.OnMedicalBed`, `GuestFaction`).
  - Join letters show Accept and Decline.
  - Journeys: `TestJourneyOrdersRescueVisitor`, `TestJourneyOrdersTakePrisonerToBed`, `TestJourneyHudJoinOfferAccept`.

## Decisions and gaps
- No incident brings faction-less strangers yet (RimWorld's transport pod crash). The join path is complete, and its only source today is the dev hook.
- The wiki's guest-death goodwill is not published, so −5 (the published "non-hostile member killed" value) is used.
- A resting guest's job line reads "Sleeping"; "Resting (injured)" would read better.

## Checklists
Vanilla 11.3 Downed (note), 11.5 Rescue `[x]`, 19.1 Prisoner state (note), 19.5 Rescue/recovery join chance `[-]` (no stranger event); UI 10.4 Bed-rest/medical-bed state `[x]`.
