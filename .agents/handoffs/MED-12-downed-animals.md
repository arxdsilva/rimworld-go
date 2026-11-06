# MED-12: downed animals

Original tuning.

## Rule (`animal.State`)
- An animal goes down at 80 % of its hit points lost, or at 450 ‰ blood loss.
- It stands again only at ≤ 50 % and < 300 ‰ (`DownsNow` / `StandsUp`, hysteresis).
- The down line sits past the 70 % break-off, so a hurt animal flees first.
- `Downed` and `DownedTick` are saved, zero-safe.

## Checked
- On a hit (`animalHurt`).
- On the hourly body step.
- On the animal's own 60-tick decision. No all-animal scan.

## While downed
- `thinkAnimal` returns at once and `moveAnimal` refuses steps.
- Food drains at half rate; rest and healing go at the lying rate. A downed manhunter stops hunting.
- Predators take a downed animal as prey whatever its size (`findPrey` looks for downed prey first). Hunters take downed marked animals first (`assignHunt`).
- Taming chance is doubled, capped at 100 %.
- The vet tends a downed tame animal where it lies.

## UI
- `AnimalView.Downed`: rest pose with a grey tint.
- The inspector line reads "Downed" ("Wild | Downed" for wild animals); the Animals table status says "Downed".
- A tame animal going down posts "X the Y is down."

## Verify
- `animal_downed_test.go` (including the no-flap test), `TestDownedThresholds` and `TestAnimalInspectorDownedLine`.
- `TestHurtPredatorBreaksOff` and `TestTerritorialReactionDoesNotFlap` were adjusted with their intent kept.
- Goldens unchanged.

## Left open
- Rope jobs don't move a downed animal.
- A downed tame animal isn't fed.
