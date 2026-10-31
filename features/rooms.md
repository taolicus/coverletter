# Rooms and seats

A room is a fixed set of four seats. The creator takes seat 1, and other
players join the lowest free seat. A room needs at least two seated players to
be ready; four is the maximum.

## States

- `waiting`: fewer than two seated players.
- `ready`: at least two seated players, not started.
- `in_progress`: the creator started the match.
- `completed`: the match ended; see [game-api.md](game-api.md).
- `abandoned` exists in the schema but has no trigger or cleanup policy yet; see
  `issues/abandoned-rooms-undefined.md`.

## IDs and share URLs

Room ids are 10 characters from the unambiguous alphabet
`23456789abcdefghjkmnpqrstuvwxyz`, generated with `crypto/rand`. Creation retries
on a collision. The share URL is `/room/{id}` under the request scheme and host;
`-base-url` overrides that base for a reverse proxy. nginx supplies `Host` and
`X-Forwarded-Proto`, so production links are `https://coverletter.pifia.net/room/...`.

## Seats

A seat is bound to a browser identity for the life of the room. Refreshing,
closing the browser, or restarting the server does not release it. A returning
cookie reclaims the same seat. Joining again as a seated player is idempotent.
The database enforces one seat per identity per room.

An occupied seat with no display name is rendered as `Seat N`. Empty seats have
`occupied: false` and an empty display name.

## API

- `POST /api/rooms`: create a room, seat the creator, return 201 with the room.
- `GET /api/rooms/{id}`: public room data. A request without a valid identity
  gets `viewerSeat: null` and does not create an identity.
- `POST /api/rooms/{id}/join`: seat the current identity, or return the existing
  seat. A full room or a started room returns 409.

`POST /api/rooms/{id}/start`, `GET /api/rooms/{id}/game`, and
`POST /api/rooms/{id}/move` are described in [game-api.md](game-api.md).

The room response contains `id`, `state`, four `seats` (`number`, `occupied`,
`displayName`), `creatorSeat`, `viewerSeat`, `canStart`, and `shareURL`.
`canStart` is true only for the creator of a `ready` room.

## Rate limits

Room creation is limited to 10 requests per minute per client IP; joining is
limited to 30. The limiter is an in-memory fixed window, so limits reset on
restart and are not shared between processes. Exceeding a limit returns 429 with
`code: "rate_limited"` and a `Retry-After` header.

The client IP is `X-Real-IP` when present, otherwise the last
`X-Forwarded-For` entry, otherwise the socket address. This trusts those headers
only because the application listens on loopback behind nginx, which overwrites
them. Do not expose the app directly while that assumption holds.
