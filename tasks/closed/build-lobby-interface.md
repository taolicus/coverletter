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

Built the lobby as three embedded assets: `web/index.html`, `web/style.css`, and
`web/app.js`. The server serves the app shell for `/` and client room URLs, and
serves the CSS and JavaScript with `Cache-Control: no-cache`. The page supports
display-name setup/change, room creation, opening a room link, the share URL and
copy action, four seats with host/you markers, taking the lowest open seat, and
polling every four seconds while visible.

Room errors use the API envelope: a missing room shows a retryable room-
unavailable page, while `room_full` and `room_started` show notices and refresh
a stale lobby. The layout uses two columns on wide screens and one below 860px;
seats collapse to one column below 560px.

Added `e2e/lobby.test.cjs` and a Playwright CI job. It drives two live browser
contexts through creation, link opening, joining, polling, copy, full-room and
started-room races, the not-found page, and a 390x844 viewport. On the phone
viewport it asserts no horizontal overflow, at least 16px body text, 44px
minimum controls, and one-column seat and room layouts; on desktop it asserts
two columns. It also checks that the embedded page and assets are served from a
freshly built binary.

Verification: `go test ./...`, `go vet ./...`, `gofmt`, and `sh workflow/lint.sh`
pass locally. CI run `41195970540` passed both the checks and browser jobs on
commit `d9a841b`; the browser log ended with `e2e: lobby flow passed`. The local
device is android/arm64 and has no browser, so the responsive check ran in CI
Chromium at a phone viewport rather than on a physical phone. The lobby has not
been deployed; production still serves the previous build until the deployment
task runs.

