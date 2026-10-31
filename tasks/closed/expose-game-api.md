---
area: api
dependencies: [create-and-join-rooms.md, implement-game-engine.md, persist-identities-and-player-api.md]
---

# Expose the game through the room API

## Context

`features/spec.md` §5, §6, §10. This task connects the room/seat model and the
engine to HTTP: the creator starts a game, seated players fetch their view, and
the active player submits a move. The backend is authoritative and persists
after every successful move.

Endpoints:

- `POST /api/rooms/{id}/start`: creator only, requires `ready`, creates one
  match from the seated players, deals, and sets the room to `in_progress`. A
  second start must not create a second match; document whether it returns the
  current state or 409.
- `GET /api/rooms/{id}/game`: returns the viewer's private view (own hand) plus
  public state. Only seated players get a hand; a non-seated visitor gets the
  public room/game summary, not hands.
- `POST /api/rooms/{id}/move`: active player only, validates through the engine,
  persists atomically, and sets the room to `completed` when the match ends. The
  request includes the expected state version.

Concurrency and replay: serialize moves per room so two requests cannot both
apply, and reject a stale `expectedVersion` with 409. Do not rely on the client
to prevent double submission.

Persistence: store the engine state as a versioned JSON document keyed by room
id (or an equivalent schema that can resume exactly); a restart must resume the
same state. Keep the private hand out of public room responses.

## Acceptance criteria

- The creator can start a game at the minimum supported player count; each
  seated player fetches their own hand; a non-seated identity sees public state
  but no hand.
- A non-creator cannot start; starting a not-ready room returns 409; starting
  twice does not create a second match.
- Only the active player can move; out-of-turn and malformed moves return
  409/400 and do not change state.
- A valid move advances the turn, persists, and is visible to the other player
  on the next `GET`.
- Replaying a move with the same `expectedVersion` returns 409 and does not
  duplicate the effect.
- The game completes and the room becomes `completed`; the final scores and
  winner are returned.
- Restarting the server mid-game resumes the same state, including the active
  player, hands, and version.
- Tests cover start authorization, turn authorization, private views, duplicate
  moves, concurrent moves, and persistence across a reopen.
- `features/game-api.md` records the endpoints, error codes, authorization
  rules, and persistence format.

## Verification

Use two cookie jars against a temp DB: create, join, start, fetch both views,
make a move, replay it, and restart the server. Run `go test ./...`, including a
concurrency test, and `sh workflow/lint.sh`. Confirm no response contains
another player's hand or an identity token.

## Outcome

Added the `games` migration, the `internal/matches` store, and
`POST /api/rooms/{id}/start`, `GET /api/rooms/{id}/game`, and
`POST /api/rooms/{id}/move`. Start is creator-only and requires `ready`; a second
start returns 409 `room_started` and never creates another match. Views return a
`PlayerView` only for seated identities and a hand-free `PublicView` for
spectators and anonymous visitors, plus `canMove`. Moves take only the expected
version and card data; the server supplies the seat. Engine errors map to 400
`invalid_move` or 409 `stale_version`, `not_your_turn`, `match_over`, and the
room/game lifecycle codes documented in `features/game-api.md`.

The engine state is stored as JSON in `games`, keyed by room id, with a
`format_version` and a `state_version` that mirrors `State.Version`. The move
update is a compare-and-swap on `state_version`, and SQLite write transactions
now begin `IMMEDIATE`, so concurrent moves and room joins serialize. A completed
match updates the room to `completed` in the same transaction. A restart loads
the same document and resumes the active seat, hands, history, and version.

Verified with `go test ./...`, `go vet ./...`, the `CGO_ENABLED=0
GOOS=linux GOARCH=amd64` build, and a two-cookie smoke run against a real server
process: create, join, start, both private views, a spectator view without a
hand, one move, a replayed move rejected with 409, then a process restart that
resumed the same version, active seat, and hands. `go test -race` is unavailable
on this android/arm64 device, so the race detector was not run. Not deployed to
production.
