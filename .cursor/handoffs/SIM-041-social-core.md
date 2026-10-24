# Task Handoff

Task ID: SIM-041 (ENG-003 M3 + M7)
Status: complete (romance, fights, prisoners, caravans pending)

## Summary
People registry, family graph, directional opinions, social interactions, memories, smoothed mood and grief.

## Behaviour
- Registry includes off-map relatives and the dead; kin derived from parent links; starting families seeded.
- Sparse opinion edges: relation bonus + traits + beauty/disfigurement + memories, clamped +-100; compatibility from pair hash and age.
- Interactions every 60 ticks, staggered, among nearby same-room pawns: chitchat, deep talk, slight, insult (+mood), kind words (Gracious). Friend/rival with hysteresis. Memories stack x0.9.
- Smoothed mood (+12/h up, -8/h down). Social recreation at tables and campfires; chat partner mirroring.
- Grief runs through the relationship graph, not proximity, incl. off-map relatives and killer opinion; Cold-hearted exempt. Colonist died -3 for 6 days. Rescue, harm and botched-surgery memories; lost/returned memories API.
- History log capped at 500.
- New traits: Striking, Fair, Homely, Grotesque, Gracious, Cold-hearted, Bloodthirsty, Austere, Asexual, Gay, Bisexual, Grating voice, Rasping breath, Distrusts men/women, Idle, Abstainer, Indulgent, Hedonist, Augment fan, Natural purist.

## Invariants / Decisions
Numbers follow the published RimWorld wiki by explicit user decision (ENG-003 balance override).

## Screen / UI Impact
See UI-022.

## Known Limitations
Not done: romance, lovers, marriage, breakup, affairs, partner beds, parties (M5); social fights (M6, chance table is domain-only); prisoners and recruitment (M8); caravans, kidnap, visitors, raider relatives (M9); animal bonds (M10).

## Follow-ups
M5, M6, M8, M9, M10.
