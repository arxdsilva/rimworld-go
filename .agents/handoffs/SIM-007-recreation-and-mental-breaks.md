# Recreation need and mental breaks

## Recreation (`domain/pawn/recreation`)
- A 0-10000 need, shown as "fun" in the inspector.
  - New colonists start at 7000.
  - It drains 1 point per 14 awake ticks, about 4300 a day. It doesn't drain during sleep.
- Free-time jobs refill it each tick they run (`doIdle`):

  | Activity | Gain per 10 ticks |
  |---|---|
  | Wander | 4 |
  | Relax (chair) | 6 |
  | Fireside | 6 |
  | Chat | 8 |
  | Horseshoes | 13 |

- Mood thoughts:

  | Level | Thought |
  |---|---|
  | below 1500 | recreation-starved (-12) |
  | below 3500 | bored (-5) |
  | 8500 and up | entertained (+4) |

- Horseshoe pin (`construction.HorseshoePin`): 1x1, 3 wood, under Architect > Furniture.
  - Colonists who aren't entertained usually pick it when one is within 30 tiles. Wanderers pick it half as often.
  - The player throws from a free tile 3 tiles off the pin (`job.Horseshoes`), with a dust burst at the pin.

## Mental breaks (`domain/pawn/mentalbreak`, `application/game/mental.go`)
- Once an hour (staggered per colonist), a colonist with mood under 20 may break. The chance is 2% per mood point under 20, so 40% an hour at mood 0.
- Break kinds (picked at random among those that make sense):

  | Break | Lasts | What happens |
  |---|---|---|
  | Sad wander | 8 h | wanders in a daze |
  | Food binge | 4 h | eats past full: at least 3 portions per sitting, `bingeEat` flag on the Eat job |
  | Hiding in room | 10 h | goes to its own bed, else the nearest room (`job.Hide`) |

- During a break:
  - The colonist ignores work, schedule and orders. MovePawn, Prioritize and DraftPawn commands are dropped.
  - It is undrafted and its order queue is cleared.
  - It still eats when starving and sleeps when exhausted.
- When the break ends, the colonist gets catharsis (+30 for a day) and a message.
- Messages: "Mental break: Bo is wandering around in a daze." and "Bo has calmed down and feels a little better."
- UI:
  - The inspector status reads "MENTAL BREAK: sad wander".
  - A danger alert names the break.
  - A storm cloud with lightning floats over the colonist.
- `PawnView` gains Recreation (percent) and MentalBreak.

## UI fixes
- Event messages are centred under the top bar. Before, they overlapped the inspector at top-left.
- The inspector's room label is right-aligned on the title row. Before, it collided with long status lines.

## Dev
- `Game.ForceBreak(id, kind)`.
- `-scenario fun -capture-ui fun`: a horseshoe pin, with Bo in a sad wander.

## Tests
- `mental_test.go`:
  - Recreation refills at the pin and drains while drafted.
  - Boredom thoughts appear.
  - A miserable colonist breaks, refuses drafting and orders, then recovers with catharsis.
  - A binge goes through a lot of food.
