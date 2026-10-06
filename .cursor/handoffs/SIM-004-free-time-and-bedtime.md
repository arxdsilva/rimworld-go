# Free time, bedtime priority, stack counts

## Free time (`application/game/idle.go`)
- A colonist with nothing to do, during work or recreation hours, starts a free-time job:
  - `job.Wander`: a random reachable tile within 5 tiles (9 for wanderers).
  - `job.Chat`: walks beside the nearest free colonist within 16 tiles. The game draws a speech bubble while they chat. The chat ends if the partner walks off.
  - `job.Relax`: sits on the nearest free chair, reserving its tile.
  - `job.FireSide`: stands beside the nearest burning campfire.
- Each lingers 240-720 ticks. A failed pick falls back to wandering.
- Temperament comes from a hash of the name, so it's stable across sessions:

  | Temperament | Wander | Chat | Relax | Fireside |
  |---|---|---|---|---|
  | Wanderer | 6 | 2 | 1 | 1 |
  | Social | 2 | 6 | 1 | 2 |
  | Homebody | 1 | 2 | 4 | 4 |

  `PawnView.Temperament` shows it in the inspector.
- Free-time jobs never block work:
  - An idling colonist re-checks for real work every `idleRecheck` (30) ticks.
  - It re-checks immediately when hungry or tired.

## Bedtime priority (`phaseAI`)
- When the schedule says Sleep, a colonist drops any job, including a player-prioritized one, and goes to bed. If it's hungry it eats first.
- Exceptions:
  - Drafted colonists stay under player control.
  - Seeking warmth comes first.

## Stack counts
- `labels.go`: every ground stack with more than 1 item shows its quantity on a dark tag in its tile's bottom-right corner. Counts are hidden when tiles are smaller than 36 px on screen.

## Tests
- `idle_test.go`: colonists wander, idling gives way to a designated tree, and bedtime overrides a move order.
- Two older tests assumed colonists stand still after arriving or near a wolf. They were loosened.
