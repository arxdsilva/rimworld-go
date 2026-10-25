# Task Handoff

Task ID: SIM-059 (PLAY-016 §3)
Status: complete

## Summary
**Damage rule** (`plant/damage.go`): cold or heat damage builds over hours instead of an instant kill.
- 25%/h at the kill threshold, +5%/h per °C beyond it, capped at 60%/h.
- The thresholds: crops' FrostKill, wild plants' ColdKill/HeatKill.
- Rates accumulate exactly across the staggered chunk windows.

**Hysteresis** (state in `Plant.Damaging`):
- damage stops only 2 °C back on the safe side;
- below 60% the plant heals 10%/h;
- from 60% it keeps dying at 25%/h.

**Effects:**
- at 100% the plant is removed, with the batched notice;
- a warning per species once a day ("Frost is damaging the corn" / "Heat is scorching…"; deduped against recent messages, no save field);
- harvest yield and the straw by-product scale by `(100 − Damage)%`.

**Read model:** `PlantView.Damage` and `DamageKind`.

## Notes
- The berry bush has no kill temperature, so it only goes dormant (unchanged).
- A field dying across a climate-interval boundary reads as two notices.
- No golden moved (the scale and grown runs never reach frost).
