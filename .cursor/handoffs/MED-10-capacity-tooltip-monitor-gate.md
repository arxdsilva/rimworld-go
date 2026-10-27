# MED-10: capacity-breakdown tooltip, vitals monitor research gate

## Capacity breakdown
`body.CapacityFactors(c)` reuses the surgery `Factor` type and explains a capacity the way `capacity()` computes it:
- imperfect parts, naming fitted implants;
- "Capped by consciousness" for moving, manipulation and talking;
- the blood pumping and filtration weights, the pain share and the blood-loss stage, for consciousness;
- each condition stage's offset or cap;
- a final "= NN%" equal to `Capacity()`.

`CapacityView.Tip` carries it. Each Health-tab capacity row has a widget (`health.cap.%d`) with that tooltip, for example:

```
Moving
Left leg: peg leg 60%
Capped by consciousness 100%
= 80%
```

## Vitals monitor gate
`build:vitals monitor` is now unlocked by Fabrication, next to the hospital bed (`TestVitalsMonitorNeedsFabrication`, in the recipe package with the hospital-bed gating test).

## Verify
`TestCapacityFactorsExplainCapacity`, `TestCapacityTipPegLeg` and `TestCapacityRowTooltip` pass, along with the full suite under `-race`.
