# Rescue gap analysis (RSC-001..)

Scope: who gets carried to a bed, which bed, what the carrier must have, and what happens after. Vanilla core only. Load only for RSC-* tasks. It starts from `PAWN_INTERACTION_GAP_ANALYSIS.md` (SIM-061..066), which holds the shipped rows; only what is still different is here.

## Decision record

| Item | Decision |
|---|---|
| Goal | Rescue already works for colonists and guests. What's missing is the event that brings strangers, rescuing animals, and a few rules. |
| Numbers | ENG-003 extended to rescue (2026-11-05, user approved). Names and prose stay original. |
| Goodwill for a rescue | Keep +12 on a healed guest leaving. The wiki's +16 row comes with its own note that rescues stopped improving goodwill in 1.4.3704, while Guest still says "relations with (faction) have improved by 12". |
| Strip penalty | Keep −40. Factions and Guest disagree ("Stripping a visitor doesn't have a negative impact on faction relations"). |
| Medical bed first | Keep. Downed says "the nearest available bed or sleeping spot"; Events (summary) sends friendly crash victims to a medical bed; a player who built a hospital expects it used. |
| Rescue before other Doctor work | Keep (`MEDICAL_GAP_ANALYSIS.md`, MED-03: rescue, bleeding, other tends, surgery). The wiki's Work page lists rescue after surgery, but a bleeding patient on the floor ranks highest for us by choice, and no page says the list is strict. |

## How we can use it

- **Rescue brings people.** The join letter, guest stay, medical beds and goodwill all exist, and the only way to reach the stranger path today is a dev command. One event connects them and gives a source of new colonists with a decision attached: spend a bed and medicine on someone who may walk away.
- **Animal rescue pays off bonds.** Beds for animals and tending already exist. Today a downed pet stays where it fell.
- **Capture gets a price (done).** Successful capture of a downed neutral surface-faction pawn now uses the existing arrest goodwill threshold once; capture admission alone does not charge.

## Sources

| Page | Used for |
|---|---|
| [Rescue](https://rimworldwiki.com/wiki/Rescue) | Targets, bed requirement, 90 °C rule, no move penalty (stub) |
| [Guest](https://rimworldwiki.com/wiki/Guest) | Join chance and "safe", no second chance, +12, strip (needs fact check) |
| [Downed](https://rimworldwiki.com/wiki/Downed) | Bed choice, crawling, shots at the downed, dying on downing (stub) |
| [Prisoner](https://rimworldwiki.com/wiki/Prisoner) | Valid prison, capture 100 % |
| [Work](https://rimworldwiki.com/wiki/Work) | Doctor giver order; Doctors "automatically rescue downed colonists" |
| [Events](https://rimworldwiki.com/wiki/Events), [Factions](https://rimworldwiki.com/wiki/Goodwill) | **Summary only**: transport pod crash, capture turning a faction hostile, +16 |

**Could not be read:** Doctor, Capture and Medical bed return 404.

**Not on any page read:** animals' bed requirement (Rescue says only that humans need one), the chance an enemy dies when downed, crawl speed and timing, how often pod crashes come, the numbers for a pod victim's wounds.

## Rows

State: `open` owed to a later change; `kept` differs on purpose. Phase in brackets.

| Id | Gap | Wiki | Today | State |
|---|---|---|---|---|
| RSC-001 | Transport pod crash | Events (summary): good, random; friendly or neutral goes to a medical bed, hostile to a prison bed; a non-hostile may join or leave. Guest: hostile victims can only be captured | TransportPodCrash incident chooses a surviving surface faction or stranger, lands a real pod shell and downed victim, posts a focused letter. Nonhostile victims enter rescue/recovery; hostile victims enter capture. | done (native) [RSC-1]; original event weights and injury tuning |
| RSC-002 | What "safe to leave" means | "the outdoor temperature is within the pawn's safe temperature range, there is no toxic fallout or acidic smog" | `careGuests.leavingSafe` now rejects active toxic fallout, including when the recovering pawn stands under a roof. Existing outdoor-temperature and threat checks remain. Acidic smog has no Core incident/model. | done for current Core hazards [RSC-1]; non-Core smog not implemented |
| RSC-101 | Rescuing animals | Rescue covers downed animals | Native Doctor rescue accepts downed tame animals, uses a safe animal bed/spot (BED-242: designated medical beds first), releases the temporary stay after recovery and survives save/replay; wild animals excluded. | done [RSC-2] |
| RSC-102 | Mechanoids | Rescue names them; "mechs always die" | No rescue of mechanoids | matches in effect |
| RSC-201 | Whose temperature | "within 90 °C of the rescuer's Comfortable Temperature range" | Bed admission and arrival use the actual carrier's Comfortable Temperature band with the 90 C margin; exposure treatment still prioritizes air that helps the patient. | done [RSC-3] |
| RSC-202 | Capture needs a room | "a valid prison, which is enclosed and has an open bed or sleeping spot" | Prison bed admission requires an enclosed room; rechecked at actual arrival. | done [RSC-3] |
| RSC-203 | Capturing a downed neutral | Summary only: turns its faction hostile, except spacers | Committed custody of a neutral surface-faction visitor/raider invokes the native faction hostility rule. Reuses the existing arrest threshold (-75); no charge on admission, no second charge after arrest, no charge for spacers, prisoners or own colonists. The faction transition propagates to other visitors. | done [RSC-3], shared existing native threshold; no claim of an independently sourced capture number |
| RSC-204 | Why there's no bed | Not published, ours | Distinct no-bed, burning, temperature and invalid prison enclosure reasons from the real destination search. | done [RSC-3] |
| RSC-205 | A carrier who can't reach the bed | Not published | Forced and autonomous rescue, capture, arrest and prisoner transport preflight the actual carrier and destination; reachable alternatives selected; stale unsafe beds refused; unreachable patient not skipped by ordinary tending. | done [RSC-3] |
| RSC-206 | Crawling | "will begin crawling after some time to the nearest bed or away from hostile threats" (1.5) | Injury-downed conscious humans crawl after 1200 ticks, one cell per 120 ticks; choose a reachable eligible bed or a safe step away from a threat; no movement while carried/reserved. | done (native; original cadence) [RSC-3] |
| RSC-207 | Enemy dies when downed | "Enemy pawns have a chance to instantly die when downed", no number | Configured storyteller colony, actual combat-caused enemy downing only: 500 permille roll on the saved body stream; environmental injury, legacy starts and prisoners excluded. | done (native; original probability) [RSC-3] |
| RSC-301 | Goodwill +16 | Factions (summary), with the 1.4.3704 note | +12, `care/guest.go:34` | kept |
| RSC-302 | Strip −40 | Pages disagree | `care/strip.go:21` | kept |
| RSC-303 | Medical first | "the nearest available bed or sleeping spot" | Ranks 5, 4, 3, 2, 0, `care/beds.go:81-93` | kept |
| RSC-304 | Rescue first in Doctor work | Work lists it after surgery | `care/rescue.go:35-37`, `priorities.go:76` | kept |

## Proposed phases

- **RSC-1, transport pod crash.** Event in `domain/incident` and the storyteller; a wounded stranger or faction member lands in a pod; letter; the existing guest path takes over. Hostile ones only by capture. Ships with the letter and the map marker, and closes checklist 1996 and the open half of 1597.
- **RSC-2, animal rescue.** Doctors carry downed tame animals to animal beds; the order on the map; tending already follows. Decide what happens with no animal bed, since the page gives only the humans' rule. Goes with the open row on feeding downed animals in `PAWN_INTERACTION_GAP_ANALYSIS.md`.
- **RSC-3, rules and reasons.** Rescuer's temperature; room check; capture cost; a specific refusal reason; the unreachable case.

## Already matching

Every downed cause but no-bed is rescuable. Sleeping spot, bedroll, bed and hospital bed all count (`Kind.Sleeps`). 90 °C margin. No movement penalty on a carrier. Join chance 50 % safe, 100 % unsafe; no second ask. Doctors rescue by themselves and any colonist by order. "Carried by" shows on the patient (`inspector/cmds_pawn.go:243`). Capture of a downed pawn always succeeds. Arrest turns a visitor's faction hostile.

## Checklist cross-reference

Vanilla: 1996 Transport pod crash `[ ]` (RSC-1); 1597 Rescue/recovery join chance `[-]` (RSC-1); 1020 and 1521 Rescue `[x]` (stay); 2281 Rescue downed/captured person `[ ]` is a quest row, not this. No row covers animal rescue, so RSC-2 adds one.
