---
area: frontend
dependencies: [expose-game-api.md, build-lobby-interface.md]
---

# Build the start flow and game interface

## Context

`features/spec.md` §7, §8, §13. This task turns the lobby into a playable game:
the creator can start, and every seated player sees the board, their hand, the
active player, discards, history, and the controls for the current move. It adds
the basic activity signals the MVP asks for: whose turn it is, when the last move
happened, and a polling refresh so remote players see changes without reloading.

Polling is the MVP transport. `GET /api/rooms/{id}/game` is fetched every few
seconds while the tab is visible. `features/spec.md` allows SSE or polling;
polling is simpler, works through nginx, and avoids long-lived connections.
Record the interval and the limits in `features/`.

## Acceptance criteria

- The creator sees a Start button only when the room is ready; starting
  transitions every seated player to the game view on the next refresh.
- The game view shows all seats, display names, elimination/status, the active
  player, the viewer's hand, public discards, and recent history.
- The viewer can play each legal card, choose a target where required, and
  submit; the UI prevents double submission and shows the server's error.
- A non-active player sees a waiting state naming the active player and cannot
  submit a move.
- The view refreshes by polling while visible; another player's move appears
  without a manual reload; polling stops while the tab is hidden.
- The layout is usable at phone width: the main board does not scroll
  horizontally, text is readable, and tap targets are adequate.
- `features/game-interface.md` records the polling interval, views, responsive
  approach, and what is not covered (for example, in-browser notifications).

## Verification

Open two browser profiles or a normal and private window against a local
server, create/join/start a game at the minimum supported player count, and play
until the documented end condition. Confirm both views, the waiting state, error display, and polling.
Use responsive mode at phone width and the dev device if available. Confirm
polling stops on a hidden tab. Record any device limitation in the outcome.

## Outcome
