---
area: frontend
dependencies: [create-and-join-rooms.md]
---

# Build the lobby interface for creating and joining rooms

## Context

`features/spec.md` §7 (Main Page and Room Lobby). This is the first real
frontend: the embedded page becomes an app shell that uses the JSON API.
Vanilla HTML, CSS, and JavaScript; no build step; responsive on desktop and
phone.

Outcome: a player can set a display name, create a room, copy or share its URL,
open a room link, take a seat, and see seats fill. Starting the game is not part
of this task; the game API does not exist yet, so the lobby shows the waiting
state and the creator's start control arrives with the game interface task.

## Acceptance criteria

- `/` shows the player's display name (or a prompt to set one), a "Create room"
  action, and a way to open a room link.
- `/room/{id}` shows the room state, the share URL with a copy action, occupied
  and empty seats with display names, and a "Take a seat" action when the viewer
  is not seated and a seat is free.
- The page handles room not found, room full, and already-started responses
  with a clear message, using the API error shape.
- The lobby shows whether the room is waiting or ready; the start control is
  added with the game interface task.
- The layout is usable at phone width: no horizontal scrolling for the main
  content, readable text, and tappable controls.
- The page is served from the embedded assets and works with JavaScript
  enabled; no framework or bundler is introduced.
- `features/lobby.md` records the routes, API calls, and empty/error states.

## Verification

Run the server, open `/` in two browser profiles or a normal and private
window, set names, create a room, open the share URL in the second profile, and
take a seat. Check the full-room and bad-room messages. Use the browser's
responsive mode at phone width (and the dev device if available). Confirm the
assets are embedded by rebuilding after an edit. Run `sh workflow/lint.sh`.

## Outcome
