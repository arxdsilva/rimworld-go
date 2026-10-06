# Per-colonist order queue

## Controls
- Hold Shift while choosing an entry in the right-click order menu (any order: go here, equip, haul, build, harvest, hunt and so on). The order is queued behind the colonist's current order.
- For a drafted colonist, Shift + right-click queues a move.
- Without Shift an order runs at once and clears the queue, as before.
- The order menu footer shows "Shift: queue (max 3)".

## Game (`application/game/order_queue.go`)
- `command.Command.Queue` marks a queued MovePawn or Prioritize order.
- `g.orderQueue[id]` holds up to `MaxQueuedOrders` (3) orders, each with the menu label it had.
  - If the colonist isn't following an order (a forced, active job), a queued order runs immediately.
  - When the queue is full, the extra order is dropped with the message "Ada already has 3 orders queued".
- `phaseAI` starts the next queued order as soon as the colonist isn't following one. This happens before drafting and autonomy. Queued orders don't start during sleep hours unless the colonist is drafted, so they resume in the morning.
- `execOrder` re-validates each order through `forcedJob` and reachability. An order that's no longer possible is skipped with the message `Ada skipped "Equip rifle": no longer there`.
- `command.ClearOrders` empties the queue.
- `cmdMove` now marks move orders Forced, so a "go here" counts as a player order.
- The new `ReasonGone` ("no longer there") replaces the misleading "unknown building" for vanished targets in `forcedJob`.
- Read model:
  - `PawnView.QueuedOrders` is a list of {Label, X, Y}.
  - `HasOrder`/`OrderX`/`OrderY` give the current player order's target.

## UI
- A "Queued orders n/3" panel under the colonist inspector lists the queue, with a Clear button.
- On the map, the selected colonist's route is drawn as a dashed line from the colonist to the current order (">") and on through the numbered queued orders.
- The colonist's temperament now shows next to its name.

## Dev
- `-capture-ui queue`: Ada goes somewhere, then picks up the rifle and walks on.

## Tests
- `order_queue_test.go` covers three cases:
  - Orders run in sequence (move, then equip rifle, then move).
  - The limit holds, and a plain order clears the queue.
  - An impossible order is skipped with a message.
