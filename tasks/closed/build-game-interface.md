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

Built the start flow and game view into the existing embedded app. The lobby
shows `Start game` only to the creator of a ready room; other seated players
poll until the room becomes `in_progress`, then switch to the game view. The
start response is applied immediately, and a repeated start refreshes instead of
retrying.

The game view renders the round, turn, draw count, target, face-up cards, every
seat with name/status/callbacks/hand count/public discards, the viewer's hand,
the newest twelve history entries, and the previous round or match results.
Visitors without a seat get a spectator view. The active player sees `Your turn`
and can select a card, choose a target and, for Reference Check, a guess; other
players see `Waiting for {name}` and no move controls. Forced Non-Compete,
missing-target cases, double-submit prevention, and server errors are handled in
the UI. `stale_version`, `not_your_turn`, and a finished match clear the
selection and refresh the board.

`GET /api/rooms/{id}/game` is polled every four seconds while the tab is
visible, with an immediate fetch on visibility return and no requests while
hidden. The interval, lack of timestamps, and polling limits are recorded in
`features/game-interface.md`; `features/lobby.md` records the lobby-to-game
transition.

Added `e2e/game.test.cjs`, which drives two browser contexts from room creation
through a complete match. It verifies the creator-only start, both player views,
the waiting state, desktop and 390x844 layouts, spectator/private view
behaviour, a stale move error, polling updates, stopped polling in a hidden tab,
and a strict callback win with at least seven rounds. The last CI run played 50
turns and ended with `e2e: game flow passed in 50 turns`.

Verification: `go test ./...`, `go vet ./...`, `gofmt`, `node --check` on the
browser tests, and `sh workflow/lint.sh` pass locally. CI run `41232950723`
passed both the checks and browser jobs on commit `8bd19c1`; the browser job took
1m19s. The android/arm64 development device cannot run Playwright
(`Unsupported platform: android`), so responsive verification ran in CI Chromium
at a phone viewport rather than on a physical phone. The game interface has not
been deployed; production still serves the previous build until the deployment
task runs.
