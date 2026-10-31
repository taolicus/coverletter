# Game API

The room API starts a match, serves each viewer an appropriate game view, and
accepts moves from the active player. The rules and state model live in
[game-engine.md](game-engine.md); rooms and seats live in [rooms.md](rooms.md).

## Endpoints

| Method | Endpoint                  | Purpose                                  |
| ------ | ------------------------- | ---------------------------------------- |
| POST   | `/api/rooms/{id}/start`   | Creator starts the match                 |
| GET    | `/api/rooms/{id}/game`    | Public game plus the viewer's private view |
| POST   | `/api/rooms/{id}/move`    | Active player submits one move           |

All three require a room id that exists. `start` and `move` create or resolve the
browser identity the same way as the player API; `game` never creates one.

### Start

Only the room creator may start, and only while the room is `ready`. A start
deals from a fresh cryptographic seed, stores one match for the room, and sets
the room to `in_progress`. The response is the room object with the new state.

Starting a second time never creates a second match. It returns 409
`room_started`; clients should fetch `/game` instead of retrying the start. A
non-creator gets 403 `not_creator`, and a room with fewer than two seated
players gets 409 `room_not_ready`.

### Game view

The response is:

```json
{
  "room": { "...": "the public room object" },
  "game": { "...": "a public or player view" },
  "canMove": true
}
```

A seated identity gets `game.PlayerView`: the public state plus `viewerSeat`, its
own `hand`, and its own `snapshots`. An identity that is not seated, and an
anonymous visitor, gets `game.PublicView`: seats, names, callbacks, hand counts,
discards, history, and round results, but no hand or snapshot. `canMove` is true
only for the active seat of a match still in progress.

Before the game starts the endpoint returns 409 `game_not_started`. Once the
match is over the room is `completed` and the final state, callbacks, and
`matchWinnerSeat` remain available.

### Move

The request body carries the version the client last saw and the chosen card.
The server derives the acting seat from the identity; a client cannot submit a
seat.

```json
{"version": 12, "card": 5, "target": 2, "guess": 0}
```

Cards are the ranks in [rules.md](rules.md): 1 Reference Check, 2 Informational
Interview, 3 Salary Comparison, 4 Personal Day, 5 Reassignment, 6 Counter-Offer,
7 Non-Compete, 8 Dream Offer. `target` is required for cards that target, and
`guess` is required for Reference Check. The response is the same shape as
`GET /game`, already showing the state after the move.

Errors:

| Status | Code                   | Meaning                                        |
| ------ | ---------------------- | ---------------------------------------------- |
| 400    | `invalid_json`         | Body is not one JSON object                    |
| 400    | `invalid_move`         | Card not in hand, illegal target/guess, or forced Non-Compete ignored |
| 403    | `not_seated`           | Identity has no seat in this room              |
| 404    | `room_not_found`       | Room does not exist                            |
| 409    | `game_not_started`     | Room has no match                              |
| 409    | `game_not_in_progress` | Match is not accepting moves                   |
| 409    | `stale_version`        | `version` is not the current state version     |
| 409    | `not_your_turn`        | Another seat is active                         |
| 409    | `match_over`           | Match has ended                                |

A rejected move changes nothing. The engine validates the version, phase, active
seat, hand membership, forced Non-Compete, targeting, and guess before applying
the effect.

## Persistence

Migration 4 adds `games`:

```sql
games(
  room_id TEXT PRIMARY KEY REFERENCES rooms(id) ON DELETE CASCADE,
  format_version INTEGER NOT NULL,
  state_version INTEGER NOT NULL,
  state TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
)
```

`state` is the JSON encoding of `game.State`, including the draw pile, hands,
discards, secret card, snapshots, history, and version. `state_version` mirrors
`State.Version` so an update can be a compare-and-swap. A restart loads the same
document and resumes the same active seat, hands, and version. `format_version`
is currently 1; a document with another version is an error rather than being
silently misread.

SQLite write transactions begin `IMMEDIATE`, so room joins and match updates
serialize. The move update additionally requires the stored `state_version` to
equal the version it read. Two concurrent moves therefore apply once: the loser
sees either the new state and fails the engine's version check, or fails the
conditional update, and receives 409 `stale_version`.

## Not covered

Moves are not rate-limited. There is no turn timeout, no SSE/WebSocket push, and
no listing of a player's rooms; the browser polls this API.
