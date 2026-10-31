---
area: rooms
---

# Abandoned rooms have no trigger or cleanup policy

## What was observed

`features/spec.md` §4 lists **Abandoned** as a room state ("inactive or
explicitly abandoned") but defines no inactivity threshold, no abandon action,
and no endpoint for it. §5 also says the game waits indefinitely for the active
player, so a room can stall forever with no move.

Without a policy, every created room and its game state stays in SQLite. The
room and game schema will need a status and timestamps either way, but the
cleanup behaviour is undecided.

## Open questions

- What counts as inactive: no request, no move, or no seated player online?
- How long is the threshold, and is it configurable?
- Can the creator abandon a room, or only the server by inactivity?
- Does abandoning delete the room and game state, or archive it?
- Are seats released, and can players rejoin an abandoned room?
- Should players be told the room was abandoned when they return?

## What would resolve it

A documented lifecycle policy in `features/rooms.md` (or a decision recorded
here), including the threshold, who can trigger abandonment, and whether data is
deleted or archived. That turns cleanup into an actionable task.

## Affects

`tasks/closed/create-and-join-rooms.md`, `tasks/closed/expose-game-api.md`,
the storage schema, and any future maintenance job.
