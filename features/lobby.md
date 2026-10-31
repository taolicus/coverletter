# Lobby interface

The browser app is still one embedded page with no framework or bundler. The
server sends `web/index.html` for `/` and for client-side room URLs such as
`/room/abc123`, and serves `web/style.css` and `web/app.js` as embedded assets.
`web/embed.go` includes all three files, so a rebuild is enough to ship a
change.

## Routes

- `/` is the landing page: identity and display name, create a room, or open a
  room link.
- `/room/{id}` is the room lobby: state, share URL, four seats, and the action
  to take the lowest open seat. The page also shows the game state if the room
  has already started or finished.
- Any other path renders a not-found page in the browser.

Navigation uses `history.pushState`; direct visits are handled by the server's
fallback to `index.html`.

## API calls

- `GET /api/player` on page load creates or reclaims the browser identity.
  The name panel uses `PATCH /api/player` with `{"displayName":"..."}`.
- `POST /api/rooms` creates a room, then the page navigates to the returned
  `id`.
- `GET /api/rooms/{id}` loads the lobby and is polled every four seconds while
  the tab is visible. Polling stops on navigation and for fatal room errors.
- `POST /api/rooms/{id}/join` takes the lowest open seat. The response replaces
  the current room view.

The page never receives identity tokens; those stay in the HttpOnly cookie.

## States and controls

The room response drives the view: `waiting`, `ready`, `in_progress`, and
`completed` (plus the reserved `abandoned` label). `waiting` means fewer than
two players; `ready` means at least two. A seated player sees their seat and a
waiting message. A viewer with no seat sees "Take seat N" for the lowest open
seat while the room is waiting or ready. Started and finished rooms do not
offer a seat.

The share field contains the server's absolute `shareURL`. The copy action uses
the Clipboard API on secure contexts and falls back to selecting the field and
`document.execCommand("copy")` on plain HTTP. The creator's start control is
not part of this page yet; it arrives with the game interface.

## Errors

API failures are read from the JSON error envelope and mapped to plain
messages:

- `room_not_found` shows a room-unavailable page with a retry action.
- `room_full` and `room_started` show a notice and refresh the room, so a stale
  lobby corrects itself.
- `rate_limited`, `invalid_name`, `network`, and unknown errors use their
  message or a generic fallback.

The room not-found state is fatal and stops polling. A polling network failure
keeps the last room on screen, shows "Can't reach the server. Retrying…", and
continues until the tab is hidden or navigated away.

## Layout and accessibility

The layout switches from two columns to one below 860px, and the seat grid
switches to one column below 560px. Inputs and buttons are at least 44px tall,
long room codes and names wrap, and the share row wraps instead of forcing
horizontal scrolling. The page uses system fonts and only embedded assets, so
it works offline after the binary is served.

## Verification

`e2e/lobby.test.cjs` uses Playwright against the real server binary. It creates
two browser contexts, takes seats, checks polling, exercises the full and
started errors from stale pages, copies the share URL, and checks a 390px-wide
viewport for horizontal overflow and undersized controls. CI installs Chromium
and runs it; the android/arm64 development device has no browser for this check.
