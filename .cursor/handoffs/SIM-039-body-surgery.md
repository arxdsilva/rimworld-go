# Task Handoff

Task ID: SIM-039 (ENG-003 M1)
Status: complete (partial vs vanilla health scope)

## Summary
Humanlike body-part tree with per-part wounds, capacities, scars, prosthetics and surgery operation bills.

## Behaviour
- Parts: brain, eyes, ears, nose, jaw, torso, heart, lungs, stomach, arms, hands, legs, feet.
- Wounds per part: cut, bruise, bite, scratch, burn, surgical; tended flag; pain; 10% permanent scar on cut/bite/scratch/burn.
- Non-vital parts can be destroyed; vital damage spills so the downed (70) and death (100) thresholds are unchanged.
- Capacities (consciousness, moving, manipulation, sight, hearing, talking, breathing) feed MoveSpeed, WorkSpeed and ShootAcc; consciousness 0 downs.
- Disfigurement from a face scar or missing face part feeds opinion.
- Prosthetics (peg leg, prosthetic leg/arm, wooden hand, denture, hearing trumpet) are crafted at the workbench.
- Operation bills: install/remove implant, amputate; success curve from skill, bed and medicine; minor and catastrophic failure; anaesthetic lasts 1h; a botched operation leaves an opinion memory.
- Old saves migrate.

## Invariants / Decisions
Numbers follow the published RimWorld wiki by explicit user decision (ENG-003 balance override).

## Screen / UI Impact
Health tab: capacities, pain, parts, operations queue with add/remove (see UI-022).

## Known Limitations
No bleeding, damage types/armor, disease framework, organ transplant or bionics; infection is still one record per pawn.

## Follow-ups
Bleeding and blood loss, disease framework, medical care policy, operation reorder.
