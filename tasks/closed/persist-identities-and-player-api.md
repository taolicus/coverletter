---
area: identity
dependencies: [embed-frontend-and-restructure.md]
---

# Persist browser identities and expose the player API

## Context

`features/spec.md` §3 and §10 require pseudonymous browser identities with no
registration: a cryptographically random token in an HttpOnly cookie, persisted
server-side, used to recognise returning players, plus a display name. This is
the first persisted data and establishes the SQLite layer, migrations, and API
error shape the rest of the game will use.

Constraint: `deploy.sh` cross-compiles with `CGO_ENABLED=0` from an android/arm64
dev device. Use a pure-Go SQLite driver (`modernc.org/sqlite`) unless a concrete
reason to break that constraint appears, and record the choice in `features/`.

Cookie and storage decisions for this task:

- 32 random bytes from `crypto/rand`, encoded as base64url; `HttpOnly`,
  `SameSite=Lax`, `Path=/`, long `Max-Age`; `Secure` when the request is TLS or
  `X-Forwarded-Proto` is `https`.
- Store only a hash of the token, never the token itself.
- A cookie whose token is not in the database is replaced with a new identity,
  not trusted.
- The database file is created with restrictive permissions; open SQLite with
  WAL and a busy timeout.
- Identity tokens never appear in URLs, JSON, or logs.
- Display names are not unique; the UI distinguishes players by seat.

## Acceptance criteria

- `-db` selects the SQLite file (default `coverletter.db`); a missing file and
  parent directory are created, and `.gitignore` ignores the database plus its
  `-wal` and `-shm` files.
- `GET /api/player` with no cookie creates a player, sets the cookie, and
  returns the player id and display name.
- A second `GET /api/player` with the cookie returns the same player id and
  creates no duplicate row.
- `PATCH /api/player` with `{"displayName":"..."}` trims and validates 1–32
  Unicode characters, rejects control characters, persists the name, and
  returns the updated player. Invalid input returns 400 and changes nothing.
- Player JSON never contains the raw token or its hash.
- Migrations run on startup and are idempotent; starting twice against the same
  file succeeds and preserves rows.
- API errors use one JSON shape across the server, for example
  `{"error":{"code":"invalid_name","message":"..."}}`.
- Unit tests cover issuance, reuse, unknown-token replacement, display-name
  validation, migration idempotence, and the absence of secrets in responses.
- The cross-compile command in `deploy.sh` still succeeds with the new
  dependency.

## Verification

Run the server with `-db` pointing at a temp file. Use `curl -c` and `curl -b`
with a cookie jar to call `GET /api/player`, `PATCH /api/player`, and
`GET /api/player` again; inspect the `Set-Cookie` flags, the JSON, and the
SQLite file. Run `go test ./...`, start the server twice against the same file,
and run the cross-compile command.

## Outcome

Added the SQLite storage layer (`internal/storage`) with WAL, a busy timeout,
private file creation, and versioned idempotent migrations. Added the player
store, the identity cookie package, and `GET`/`PATCH /api/player`. The server
accepts `-db` and creates the database on first start. The cookie stores only a
SHA-256 token hash, and player JSON contains only the id and display name.
Recorded the decisions in `features/identity-and-storage.md` and the database
location in `features/deployment.md`.

Verified with `go test -race ./...`, `go vet ./...`, a curl flow covering
creation, reuse, rename, and an invalid name, a restart against the same
database, and the `CGO_ENABLED=0` linux/amd64 cross-compile. The database file
was created with mode `0600`. Not deployed to production in this task.
