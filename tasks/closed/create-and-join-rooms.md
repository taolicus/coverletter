---
area: rooms
dependencies: [persist-identities-and-player-api.md]
---

# Create rooms, share their URLs, and bind seats to identities

## Context

`features/spec.md` §4 and §10. Any player can create a room; the creator takes
the first seat; the room has an unguessable id and a shareable URL; players join
into available seats; a seat is permanently bound to a browser identity for
that room; returning players reclaim their seat; refreshing or closing the
browser does not release it.

This task implements the room and membership model and the HTTP API. It stops
before a game starts: rooms are `waiting` or `ready`, and the start endpoint is
handled by the game API task. Fixed MVP settings: 2–4 seats (subject to the
rules decision), no room passwords or custom settings. The `abandoned` state is
out of scope; see `issues/abandoned-rooms-undefined.md`.

Decisions to apply:

- Visiting a room URL shows the lobby; taking a seat is an explicit
  `POST /api/rooms/{id}/join`, not a side effect of `GET`, so a shared link
  cannot silently consume a seat.
- Room ids are 10 characters from `crypto/rand` using an unambiguous alphabet,
  retried on collision.
- The creator is the only player who may start; the room view reports
  `canStart` for the viewer.
- The share URL is derived from the request scheme and host, with a `-base-url`
  override for a reverse proxy.
- An unset display name is rendered as "Seat N" in public room data.
- Rate-limit room creation and joining per client IP; use `X-Forwarded-For`
  only as documented for the proxy and record the trust assumption. Return 429
  with the standard error shape.

## Acceptance criteria

- `POST /api/rooms` creates a room, seats the creator, and returns the room id,
  state, seats, and share URL.
- `GET /api/rooms/{id}` returns public room data: id, state, seats with display
  names and occupied/empty, creator seat, the viewer's seat if any, and
  `canStart`. It never exposes identity tokens or private data.
- `POST /api/rooms/{id}/join` seats the current identity when a seat is free;
  calling it again as the same identity is idempotent; a full room returns 409;
  a room that has started returns 409 for a non-seated identity.
- A room with at least the minimum player count (2 unless the rules decision
  changes it) is `ready`; below that it is `waiting`.
- A database constraint (or equivalent) prevents one identity from holding two
  seats in the same room.
- Seats, room state, and membership survive a server restart; the same cookie
  reclaims the same seat.
- Creating or joining beyond the configured rate returns 429 and does not create
  a room or seat.
- Tests cover create, join, reclaim, full room, started room, creator-only
  `canStart`, token absence, id collision retry, and restart persistence.
- `features/rooms.md` records the room states, id/URL scheme, seat binding, and
  rate-limit policy.

## Verification

Run the server against a temp DB. Use two cookie jars (two identities) to
create, join, re-fetch, fill the room, attempt a fifth join, and attempt to join
after setting the room to started through a test fixture. Restart the server and
re-fetch with both cookies. Run `go test ./...` and `sh workflow/lint.sh`.

## Outcome

Added the `rooms` and `room_seats` migrations, the room store, and the room
HTTP API. Rooms have four seats, the creator takes seat 1, and two seated
players make the room `ready`. Joining is idempotent for seated identities; a
full or started room returns 409. `GET` does not create an identity. The view
reports four seats, `creatorSeat`, `viewerSeat`, `canStart`, and a share URL
derived from `Host`/`X-Forwarded-Proto` or `-base-url`. Creation and joining are
rate-limited per client IP with the proxy-header trust assumption documented in
`features/rooms.md`.

Verified with `go test -race ./...`, `go vet ./...`, a two-cookie curl flow
covering create, join, full-room rejection, restart persistence, and proxied
share URLs, plus the `CGO_ENABLED=0` linux/amd64 cross-compile. The started-room
case is covered by a test fixture. Not deployed to production in this task.
