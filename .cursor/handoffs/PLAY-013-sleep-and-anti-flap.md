# PLAY-013: Sleep rules, anti-flapping pass, zones over floors

## Why
A colonist on a free schedule bounced between bed and work. It woke as soon as it was a hair over the 20% tired line, after a 40-tick nap. An audit then found the same pattern elsewhere: behaviour that starts and stops at the same threshold, two interrupts that cancel each other, and unreachable targets retried every tick. Separately, zones on a built floor were drawn under the floor.

## Sleep (user-specified)
- **Rate:** `needs.SleepGain=2` every `SleepPeriod=5` ticks, so 20%→100% takes exactly 8 h.
- **Start rules** (`tick.go` `phaseAI`):
  - **Work hours:** a tired colonist keeps working (`needs.SleepAllowed`) until it collapses at 0.
  - **Anything / Recreation:** it sleeps once tired.
  - **Sleep block:** bed, unless `restedFree` is set (then free time until rest falls to 20%).
- **Wake rules** (`sleepDone`):
  - collapsed: stays down until `WakeRest` (80%), and hunger doesn't wake it;
  - hungry with reachable food: wakes to eat;
  - Sleep block: wakes at 100% and sets `restedFree`;
  - Work hours: wakes above 20% (the user chose to keep this);
  - otherwise: wakes at 80%.
- **Collapse:** the colonist sleeps where it dropped, in the same tick. The log says "Collapsed from exhaustion" (new `logCollapsed`).

## Anti-flap fixes
| ID | What | Fix |
|---|---|---|
| W1/W2 | Warm and Sleep (or patient rest) interrupted each other every tick | Sleep/patient interrupts skip `job.Warm`. Warmth only gets a sleeper up at `exposure.Recover` (`exposureDanger`). |
| W3 | Warmth check cancelled the job even when there was no warm spot | `warmSpot` search first; interrupt only if a spot is found |
| F1 | Unreachable food retried every tick | `noFood` cooldown `foodRetry=500`; `seekFood` |
| B1 | Unreachable bed retried every tick | falls back to `assignSleepInPlace` |
| I1 | A failed work giver overwrote free time every 30 ticks | `keepIdling` restores the running activity |
| D1 | Dead `Exhausted()` checks | removed (`phaseNeeds` collapses first) |
| H1 | Campfire thermostat flickered | `banked` band: banks at 20 C, relights below 16 C |
| E1 | "Until you have X" restarted for one item; cooks overshot | `Bill.Waiting` latch, resumes at `ResumeAt` (10→8); `billPending` counts meals in progress |
| A1/A2 | Fleeing animals stopped one tile out and turned back | release radius `FleeRadius+animal.FleeRelease` (9); finish the flee path |
| A3 | Enraged animals logged "Fled" on every hit | no flee reaction while enraged |
| C1 | Unarmed colonist: work↔flee loop | one "Fled from X" log; `calm` 600 ticks after the threat was last seen; release radii 8/18; holds position while calm |
| C2 | Armed colonist re-targeted every tick (aim reset) | sticky target unless the new threat is 3 tiles closer |

The new state is `restedFree`, `noFood`, `calm` and `banked`. Each is an optional save field; older saves load without it. `Bill.Waiting` is saved with the bill.

## Zones over floors
- **Draw order:** `drawArt` now runs terrain, ground cover, floors, zones, blueprints. The fallback `drawWorld` draws floors before zones.
- **Test hook:** `Renderer.layerTrace` lets `TestZoneDrawsOverFloor` check the layer order.
- **Capture:** `captureZones` puts a stockpile on a wood floor.

## HUD
No new widget was needed:
- **Collapse:** the existing alert, plus the new log line.
- **Free time at night:** the job line.
- **Sleep length:** the inspector's Rest bar.
- **Colonist flee:** "Fled from X" in the log.
- **Bills:** the bill row shows "(Have enough - resumes at N)".

## Verification
- `go vet ./...` is clean; `go test -race ./...` passes; the determinism run (`-count=3`) passes.
- **Regression tests:** every new no-flap test fails on the old code. Old counts: cold+tired 142 kind changes in 500 ticks, cold patient 300 in 300, unreachable food 1000 failed ticks in 1000.
- **Benchmarks:** `ColonyHour` 28.2 → 22.3 ms; `ColonyRaid` 30.3 → 23.6 ms; `DrawArt` 5.3 → 3.7–4.5 ms with the same alloc count.
- **Capture:** checked visually.

## Default-schedule side effect
With the default schedule (Sleep 20–05), colonists tend to tire in late Work or Recreation hours. They go to bed early and wake at 100% around 01–03, then have free time until morning. A shorter default Sleep block would balance this; that is for the user to decide.

## Follow-ups (audited, not changed)
- Siege re-acquire and breach re-plan (unclear).
- Indoor sowing season uses live room temperature.
- Preferring beds in warm rooms.
- Predators never give up a chase.
- A tame grazer whose flee path fails re-plans every tick.
- A colonist in its calm window also delays its bedtime and warmth checks (up to 600 ticks).
