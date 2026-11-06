# CSE-W7: joy and drugs (CSE-001, 002, 007, 008, 010, 011)

All six rows are finished. All numbers not in the mod's table are ours.

## What each row does

| Row | Where | Behaviour | Keeps it from flapping |
|---|---|---|---|
| CSE-001 | `recreate/seek.go` | `Seek` keeps the best outdoor pieces while it scores, tries them first, then the rest. | Saved craving, on below 40 %, off from 60 %. Bed rest keeps the craving and skips the outdoor pass. |
| CSE-002 | `sense/joy.go` `TopOff` | In a free hour, `Wants` false sends a colonist to `assignWorkByPriority` before idling. | `TopOffOn` set above 95 %, cleared below 80 %. |
| CSE-007 | `care/gift.go` `Giving` | After `Visits.Assign` picks a patient, a hash of tick and pair may turn the visit into `job.FeedPatient` with a dose. Pack first, else the nearest stack. Drug weight `4*(100-tolerance)`, visit `6*100`. | The visit slot, plus the dose gap in `Allows`. |
| CSE-008 | `sense/joy.go` `Relax` | `idleSocialRelax` asks `Relax.Assign` first. The `SocialRelax` job fetches a drink (`Target`), carries it (`Carry`), and finishes it at `Full*99% - dose joy`, or when the linger ends. | At least `relaxSit` (240 ticks) seated, plus the dose gap. |
| CSE-010 | `drug/policy.go`, `dose/drugs.go`, `windows/assign.go` | Opt-in `drug.MoodLift`: beer only, mood below 50, and with `DrugPotentialMood` on the target mood must be below 50 too. A craving ignores the gate, as on every other policy. | The 12-hour dose gap. |
| CSE-011 | `dose/habits.go` `fromPack` | `Assign` looks in the pack before `nearest`, medical and recreational alike. | `Retry` on a failed start, plus the dose gap. |

## Measured effect of CSE-002

From throwaway runs (default schedule, 48x40, three colonists, 5 days, plants and rock designated):

| Colony | Rule off | Rule on |
|---|---|---|
| Seed 1, chess table and horseshoe pin | 4.9 % of Recreation-hour colonist-ticks at work | 13.5 % |
| Seed 2, same pieces | 6.9 % | 6.9 % (joy peaks at 86.6 %, never topped off) |
| Seeds 1 and 2, no recreation piece | 4.4 %, 5.5 % | Unchanged |

What remains with the rule off is jobs carried over from a Work hour. In seed 1, total work fell 2.4 % and sleep rose 8.5 % with the rule on, so the extra work in Recreation hours didn't add up to more work overall. I didn't look into why; two seeds don't show a trend.

`TestRecreationBlockWorksWhenFull` forces the case the rule is for (colonists entering their first Recreation hours at 97 %): 23.9 % at work with the rule off, 78.1 % with it on. It logs these numbers. In that test the sleep-block count went from 28445 to 29789. With the rule off, seed 1's colonist 95 took a nap in a Recreation hour without being tired; with it on, that colonist kept cutting and entered the Sleep block at rest 2499 instead of 5522. It still went to bed at bedtime once its meal ended. The test counts sleep in Sleep hours only, since the nap gives way to work.

## Decisions to check

- **`TopOffOn` means "topped off".** The comment in `sense/sense.go` ("topping off, ends at the high mark") says the reverse, and that file wasn't mine to edit: please fix it.
- **The flag follows joy when `Wants` is read**, in free-time decisions, not at each job's end and start as the row says. That avoids a hook on every job. A mark passed during a Work hour counts from the next free hour.
- **The Sleep block:** the row includes it, and the same site serves a colonist who woke rested in a Sleep hour. It works until it tires. `bedtime` is checked earlier and still interrupts at once.
- **`SeatWanted`:** `Wants` returns true while a colonist is sitting down or could look for a seat, because `SitDown` lives in `assignIdle` and topped-off work would otherwise starve comfort. Not in the row.
- **Tolerance scales only the drug's weight**, not the visit's (the row scales both). The visit has no joy kind to scale by.
- **Gifts from the pack and `AnyDrugFromPack`:** with the rule off, a gift comes from a stack only.
- **`DrinkFor` skips drugs with an overdose limit**, so Unrestricted doesn't bring a hard drug to a table. Ours.
- **`TopOffOn` entries of the dead stay**, bounded by the colony's roster. `Craving` is cleared by `recreate.Forget`, which has no root caller for it today, so it stays bounded the same way.

## Known limit

A work giver whose `startJob` fails during a relax has already put the drink down. The relax is restored bare, and the drink stays on the map or goes back to the pack. Counts balance (`TestRelaxDrinkBalance/work_giver_fails`), but the colonist can lose a drink it was holding.

## Files outside the brief, each needed

- `windows/windows_test.go`: the preset count and the new label.
- `game/dose_host.go`, `game/game.go`: one role field each.
- `care/visit.go`: the `Giving.Try` hook, plus `wantsCompany`.
- `game/social.go` `doSocialRelax`: the `Relax.Do` call.
- `game/tick.go` `startJob`: a replaced `SocialRelax` drops its drink, as `Read` drops its book.

## Save

`recreate.Save.Craving` is the only new field, additive and sparse. `save_schema.txt` gained those two lines and nothing else, checked by diff.

`TopOffOn` already existed. With every toggle off, the hash matches f7631b72a at seed 1 and 2, 30000 ticks: `TestHabitsOffKeepTheHash` pins both. A change outside the habits moves it, as it does the scale goldens: re-pin with the reason.

## Visuals

- **MoodLift** is a fourth button on the Assign tab's drug policy, with a tip.
- **Toggles** are in the Habits tab (CSE-W1).
- **Gifts** show as the feed-patient job and the existing "fed by" log lines.
- **The rest** is plain behaviour with existing job labels: the carried drink, the outdoor choice and the extra work. No new screen is owed.

## Tests

`-race`, targeted, in `recreate/seek_outdoors_test.go`, `sense/joy_test.go`, `care/gift_test.go` and `game/sense_joy_test.go`. The row tests are all there by name.

Also: `TestTopOffWantsAllocatesNothing`, `TestTopOffKeepsBedtime`, `TestTopOffStillEats`, `TestRelaxDrinkBalance` (5 events by stack and pack), `TestPackDoseInterrupted`, `TestGiftFallsThrough`.

## Benchmark

`-benchtime 2000x`, no race:

| Benchmark | Result |
|---|---|
| `BenchmarkFreeTimeTopOff` (`Wants`) | 31 ns/op, 0 allocs |
| `BenchmarkFreeTimeStep/off` | 14640 ns/tick |
| `BenchmarkFreeTimeStep/on` | 14598 ns/tick |

The two `Step` figures are the same within noise.

## Lines added to `game`

`sense_joy_host.go` is 99, and the hooks add about 10.

## Goldens and other failures

`TestGrownGolden` moved (`0xba1495be71601c63`, was `0x7a22aca804422474`), not re-pinned. Other agents' default-on rows are in the same tree, so this isn't all from these six rows. `TestScaleGolden` still passes: nothing reached a recreation piece or the top-off mark there.

The whole `game` package under `-race` has two failures outside these rows:

- `TestUnstowIsAnOrdinaryCarriedStack`: expects a dropped pack stack on the ground, but `dropCarry` now calls `BackToPack` (CSE-W5).
- `TestBillGathersWoodFromSeveralStacks`: never sees `HaulIngredients` (`game/craft`, CSE-W4's area). It runs in a Work hour, which none of these hooks reach. I didn't confirm the cause.
