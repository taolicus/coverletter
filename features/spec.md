# Cover Letter — Game Project Specification

## 1. Project Overview

**Cover Letter** is a multiplayer, browser-based adaptation of the tabletop game _Love Letter_, re-themed around applying for a job.

Players compete to get their job application noticed and ultimately secure the position. The game retains the simplicity and strategic interactions of Love Letter while introducing a job-application theme.

The game is designed to be:

- Multiplayer.
- Pseudo-anonymous.
- Asynchronous, allowing players to take turns at different times.
- Room-based, allowing separate games between different groups.
- Accessible through a web browser without requiring user registration.

## 2. Technology Stack

### Backend

- **Language:** Go (Golang).
- **Application:** Single compiled Go binary.
- **HTTP server:** Go standard library (`net/http`).
- **Static assets:** Embedded into the binary using `embed.FS`.
- **Database:** SQLite for persistent game state, rooms and player identities.
- **API:** JSON-based HTTP endpoints.
- **Real-time updates:** Server-Sent Events (SSE) or periodic polling for notifying players of game updates.

### Frontend

- HTML5.
- CSS3.
- Vanilla JavaScript.
- Responsive design for desktop and mobile browsers.
- Communicates with the backend through JSON APIs.

### Deployment

- Single executable binary.
- No separate frontend build server required at runtime.
- No external database server required.
- Configurable HTTP listening address and database location.
- Suitable for deployment on a small VPS or self-hosted server.

## 3. Player Identity

Players should not need to create accounts or provide personal information.

### Browser Identity

- On first visit, the application generates a cryptographically random browser identity token.
- The token is stored in a secure, persistent browser cookie.
- The backend uses this token to identify returning players.
- Player identities are pseudonymous rather than tied to real-world accounts.
- Players can choose a display name.

### Identity Requirements

- A browser identity persists across sessions.
- The same browser identity can participate in multiple rooms.
- A player must not be able to occupy multiple seats in the same game using the same identity.
- Browser identity tokens must not be exposed in public room information or URLs.
- Losing the browser token means losing access to the associated player identity unless recovery functionality is introduced.

## 4. Game Rooms

Players can create and join separate game rooms.

### Room Creation

- Any player can create a new room.
- Each room receives a unique identifier.
- Each room has its own shareable URL.
- The room creator becomes the first player and occupies the first seat.
- The creator can configure available game settings (the MVP uses fixed 2–4
  player settings).

Example:

`https://example.com/room/abc123`

### Joining Rooms

- Players can join using the room URL.
- Joining a room requires an available seat.
- Each player occupies one seat.
- Once a player takes a seat, that seat is permanently associated with their browser identity for that game.
- Returning players automatically reclaim their existing seat.
- Refreshing the page or closing the browser must not release a player's seat.

### Room States

A room can be in one of the following states:

- **Waiting:** Players are joining.
- **Ready:** Minimum player requirements have been met.
- **In Progress:** A game is underway.
- **Completed:** The game has finished.
- **Abandoned:** The game has been inactive or explicitly abandoned.

## 5. Multiplayer Gameplay

The game follows the adapted Love Letter rules in [rules](rules.md).

### Player Management

- Each game has a fixed set of player seats once started.
- Each seat is associated with a browser identity.
- Players can see other players' display names and public game information.
- Players cannot impersonate other seats.
- Players can only perform actions when it is their turn.

### Game Flow

1. A room is created.
2. Other players join through the room link.
3. Players occupy available seats.
4. The game starts once the minimum player requirement is satisfied and the game is started.
5. Players take turns drawing cards and playing cards.
6. The game advances when the current player submits a valid move.
7. The game ends when the round or game-winning condition is reached.

### Asynchronous Gameplay

Asynchronous gameplay is a core requirement.

- Players do not need to be online simultaneously.
- The game does not automatically advance through player turns.
- The game waits indefinitely for the current player to make their move, subject to any configured optional timeout rules.
- A player can close their browser and return later.
- On returning, the player sees the current game state and any pending action.
- Other players can view the game but cannot make moves on behalf of the active player.
- The game state is persisted after every successful move.

**Important:** A game must never skip a player's turn simply because they are offline.

## 6. Game State and Persistence

All authoritative game state is maintained by the backend.

### Persisted Data

- Browser player identities.
- Player display names.
- Room configuration.
- Room membership and seat assignments.
- Game status.
- Current turn.
- Player hands.
- Draw pile and discard pile.
- Played cards and their effects.
- Player elimination status.
- Round and game scores.
- Game history.

### State Integrity

- All moves must be validated server-side.
- Invalid moves must not change the game state.
- Moves must be processed atomically.
- Only the active player can submit a move.
- Duplicate move submissions must not result in duplicate actions.
- Private information, including other players' hands, must never be exposed through public API responses.

## 7. Web Interface

### Main Page

- Create a new game room.
- Join an existing game through a room link.
- Display the player's identity and display name.

### Room Lobby

- Display room information.
- Show occupied and available seats.
- Show player display names.
- Allow the room creator to start the game when requirements are met.
- Display the room's shareable URL.

### Game Interface

- Display all players and their status.
- Show the current player's hand.
- Display the active player's turn.
- Display public discard piles and game history.
- Provide appropriate actions for playing cards.
- Show the effects of played cards.
- Clearly indicate whose turn it is.
- Display a waiting state when another player needs to make a move.

### Responsive Design

The interface must work on:

- Desktop browsers.
- Mobile browsers.
- Tablet browsers.

## 8. Notifications and Updates

Players should be able to identify when action is required.

Potential features:

- In-browser notifications when it becomes a player's turn.
- Room activity indicators.
- Last-move timestamps.
- Unread game activity indicators.
- Optional email notifications in a future release.

Notifications must not be required for the game to function.

## 9. Security and Anti-Cheating

- Use cryptographically secure random tokens for browser identities.
- Use secure, HTTP-only cookies where supported.
- Validate all game actions on the server.
- Never trust client-submitted game state.
- Prevent players from accessing private information belonging to other players.
- Protect against replayed or duplicated move submissions.
- Apply rate limiting to room creation and joining endpoints.
- Use HTTPS in production.
- Avoid exposing browser identity tokens in URLs or logs.

## 10. Suggested API Structure

Example HTTP endpoints:

| Method | Endpoint                 | Purpose                      |
| ------ | ------------------------ | ---------------------------- |
| POST   | `/api/rooms`             | Create a room                |
| GET    | `/api/rooms/{id}`        | Get public room information  |
| POST   | `/api/rooms/{id}/join`   | Join a room                  |
| POST   | `/api/rooms/{id}/start`  | Start a game                 |
| GET    | `/api/rooms/{id}/game`   | Get current game state       |
| POST   | `/api/rooms/{id}/move`   | Submit a player move         |
| GET    | `/api/rooms/{id}/events` | Subscribe to room updates    |
| GET    | `/api/player`            | Get current browser identity |
| PATCH  | `/api/player`            | Update display name          |

All endpoints requiring player authentication must validate the browser identity token.

## 11. Suggested Go Project Structure

```text
cover-letter/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── auth/
│   │   └── identity.go
│   ├── rooms/
│   │   ├── handler.go
│   │   └── service.go
│   ├── game/
│   │   ├── engine.go
│   │   ├── cards.go
│   │   ├── rules.go
│   │   └── state.go
│   ├── players/
│   │   └── players.go
│   ├── storage/
│   │   ├── sqlite.go
│   │   └── migrations/
│   └── http/
│       └── router.go
├── web/
│   ├── index.html
│   ├── css/
│   │   └── style.css
│   ├── js/
│   │   ├── app.js
│   │   ├── lobby.js
│   │   └── game.js
│   └── assets/
├── go.mod
└── README.md
```

The frontend directory should be embedded into the Go executable using `embed.FS`.

## 12. Additional Features and Future Enhancements

Features to consider after the initial release:

### Game Experience

- Custom room names.
- Configurable player limits.
- Configurable match length and scoring variants.
- Game history and replay.
- Spectator mode.
- Customisable display names and avatars.

### Social Features

- Room passwords.
- Private and public rooms.
- Room discovery.
- Invite links.
- Optional player messaging.
- Player presence indicators.

### Asynchronous Enhancements

- Configurable turn reminders.
- Optional turn deadlines.
- Notifications when a game requires attention.
- Activity timeline showing previous moves.
- Resume games from any device using an explicit account or recovery mechanism.

### Administration

- Room owner controls.
- Ability to cancel a game.
- Administrative room management.
- Configurable server-wide game settings.

## 13. Initial MVP Scope

The first release should prioritise a complete, reliable multiplayer game over additional features.

### Required for MVP

- [ ] Single Go binary with embedded frontend.
- [ ] SQLite persistence.
- [ ] Automatic browser identity generation.
- [ ] Player display names.
- [ ] Room creation.
- [ ] Unique room URLs.
- [ ] Multiplayer room joining.
- [ ] Browser identity-bound player seats.
- [ ] Complete implementation of the [adapted Love Letter rules](rules.md).
- [ ] Server-authoritative game engine.
- [ ] Asynchronous turn-based gameplay.
- [ ] Persistent game state.
- [ ] Players can leave and return without losing their seats.
- [ ] Browser-based game interface.
- [ ] Basic game activity updates.
- [ ] Responsive mobile interface.

### MVP Design Principle

The game should be fully playable without requiring players to be online at the same time.

The backend is the source of truth for all game state, while the browser is simply the interface through which players view the game and submit their moves.

**The primary objective is to create a lightweight, self-hostable, pseudo-anonymous multiplayer game that can run from a single Go binary.**
