# Task Handoff

Task ID: SIM-035 (PLAY-010)
Status: complete

## Summary
Every animal hit by an attacker now reacts, whatever its size: fighters (territorial, predator, pack predator, legendary) fight back, everything else flees. An animal that can't fight back (flee type, tame and hit by its own colonist, badly hurt, or the attacker is out of reach) flees on every hit, to the farthest reachable spot when the straight-away one is blocked; a penned tame animal flees within its pen. Wild predators may charge colonists or raiders that come within 3 tiles: a roll every 60 ticks, 30%, then a 600-tick cooldown; never while asleep, fleeing, hurt, tame or enraged, and off on difficulties without "predators hunt colonists" (Wanderer, Gentle).

## Files
- `combat.go` `hurt` → `animal_needs.go` `animalHurt` (fight or flee on every hit).
- New `animal_flee.go`: `canFightBack`, `fleeHit`, `fleeFrom`, `fleeSpot` (straight away → farthest reachable within 8 tiles → a neighbour), `reachesFoe`, pen restriction (`penQuery`), exact reachability when regions are off or inside a pen.
- `animals.go`: `fleeCheck` (tame animals flee threats too; a predator that can't reach it across a fence or water is ignored), `fightCheck` (fights its own species; no revenge on an unreachable foe), new `chargeCheck`.
- `domain/animal`: `ChargeSince`, `NextCharge`, charge constants, `Fighter`, `Charger`.
- Event log: "Charged by a <species>" / "Charged <name>"; inspector action "Charging <name>".

## Notes
- Pets (dogs, cats) are a flee class: they run rather than fight.
- No slaughter path goes through `hurt`.
- `TestPredatorBlockedByFence` now designates the pen (an unpenned tame hare may wander out of the gate on its own).
- Goldens re-pinned (see ENG notes in scale_golden_test.go): in the six golden colonies no colonist dies; two take a hit from a charge.

## Tests
`animal_react_test.go` (incl. `TestPennedAnimalFleesInsidePen`), `animal_charge_test.go`.
