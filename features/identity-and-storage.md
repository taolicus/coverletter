# Identity and storage

Players are pseudonymous. The server issues a browser identity on the first
`GET /api/player` and recognises returning browsers by that cookie. There are no
accounts or passwords.

## Database

The server uses SQLite through the pure-Go `modernc.org/sqlite` driver, so the
`CGO_ENABLED=0` cross-compile still works. `-db` selects the file (default
`coverletter.db`, relative to the process working directory). A missing parent
directory and database file are created; a new database file is mode `0600`.

The connection enables WAL, a 5000 ms busy timeout, and foreign keys. Migrations
run at startup, are recorded in `schema_migrations`, and are skipped on later
starts. The first migration creates `players`:

```sql
players(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  token_hash BLOB NOT NULL UNIQUE,
  display_name TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
)
```

## Browser identity

The cookie is `coverletter_identity`. Its value is 32 random bytes from
`crypto/rand`, encoded with base64url. The server stores only the SHA-256 hash of
the decoded token. A cookie that is malformed or whose hash is absent from the
database is replaced with a new identity; it is never trusted.

Cookie attributes: `HttpOnly`, `SameSite=Lax`, `Path=/`, `Max-Age` of ten years,
and `Secure` when the request is TLS or `X-Forwarded-Proto` is `https`. Tokens
never appear in URLs, JSON, or logs.

## Player API

`GET /api/player` returns the current player, creating an identity if needed.
`PATCH /api/player` accepts `{"displayName":"..."}` and returns the updated
player. The player JSON is only `{"id":1,"displayName":"..."}`; the token and
hash are never included.

Display names are trimmed, must be valid UTF-8, 1–32 Unicode code points, and
must not contain control characters. They are not unique; a room distinguishes
players by seat.

API failures use one JSON shape:

```json
{"error":{"code":"invalid_name","message":"display name must be 1-32 characters"}}
```
