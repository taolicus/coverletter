# Game interface

The game uses the same embedded page and route as the lobby. `/room/{id}`
shows the lobby while the room is `waiting` or `ready`, and the game board once
it is `in_progress` or `completed`. The API and rule details live in
[game-api.md](game-api.md) and [rules.md](rules.md); the lobby flow is in
[lobby.md](lobby.md).

## Start flow

The creator sees a `Start game` button in the lobby action panel when the room
response has `canStart: true`, which the server sets only for the creator of a
`ready` room. Starting posts `/api/rooms/{id}/start`, then fetches
`/api/rooms/{id}/game`. Other seated players keep polling the room; when it
becomes `in_progress`, the lobby switches to the game view and fetches the game
on the next poll. A second start returns `room_started`, and the client refreshes
into the game instead of retrying the start.

## Polling

`GET /api/rooms/{id}/game` is polled every four seconds while the tab is
visible. The lobby polls `GET /api/rooms/{id}` on the same interval until the
game starts. The visibility change listener fetches immediately when the tab
returns and the interval skips while the tab is hidden, so a hidden tab makes no
game requests. Polling stops when the room is not found.

There is no SSE or WebSocket transport and no push notification. A remote move
can take up to four seconds to appear. The player who submitted a move applies
the response immediately. The poll carries no version parameter; the next
response simply replaces the board. The engine history has no timestamps, so the
"last move seen" time is browser-local and resets on reload.

## Views

Every seated identity gets a player view with its own hand and private
snapshots. A visitor without a seat gets the public view and sees a spectator
panel instead of a hand. Both views show:

- the round, turn, draw count, target, face-up set-aside cards, and every seat;
- display names, active/eliminated/protected status, callbacks, hand counts, and
  public discards;
- the newest twelve public history entries, with card results rendered as plain
  language;
- the previous round's winners, revealed secret, and any hands revealed at deck
  exhaustion.

The active player sees "Your turn" and can select a card. Everyone else sees
"Waiting for {name}" naming the active player; eliminated players see that they
are out of the round. At match end both views show the winner and final
callbacks.

## Move controls

Selecting a hand card opens a form for its target and, for Reference Check, the
guessed card. Targets are limited to seats the server will accept: other live,
unprotected players for Reference Check, Informational Interview, Salary
Comparison, and Counter-Offer; any live player including the viewer for
Reassignment. If every other player is protected, a card that targets another
player is submitted without a target and resolves with no effect.

When Non-Compete is held with Reassignment or Counter-Offer, only Non-Compete is
selectable. The Play button stays disabled until every required choice is made,
the same button is disabled while a request is in flight, and a second submit is
ignored, so a double tap cannot send two moves. A rejected move leaves the
selection in place and shows the server error. `stale_version`,
`not_your_turn`, and a finished match clear the selection and refetch the board.

## Layout and accessibility

The board uses two columns above 860px and one column below. The seat grid uses
two columns above 560px and one below. Card controls and selects are at least
44px tall, long names and card text wrap, and the board never requires
horizontal scrolling at phone width. The page uses the existing panel, focus,
and reduced-motion styles.

## Verification

`e2e/game.test.cjs` runs against the real server binary in Chromium. It starts a
two-player game from the lobby, checks the creator-only start button, both
views, the waiting state, desktop and 390px layouts, a stale move error, polling
updates, that a hidden tab stops polling, and plays the match through to a strict
callback win. `e2e/lobby.test.cjs` covers the lobby-to-game transition. CI runs
both. The android/arm64 development device has no usable browser, so the browser
checks are CI-only rather than physical-phone checks.

## Not covered

There are no in-browser notifications, sound, unread indicators, turn reminders,
or server timestamps. The game does not show spectators' names or provide a
separate spectator link. Card effects that depend on hidden information are
resolved by the server and only their public results appear in history.
