---
area: foundation
---

# Run the server from `cmd/server` with the page embedded from `web/`

## Context

The server is a single root `main.go` that serves an inline HTML string.
`features/spec.md` §2 and §11 call for a single binary with frontend assets
embedded via `embed.FS`, and a layout that can grow into `internal/` packages.
This task establishes that layout without changing what a visitor sees. It also
updates `deploy.sh`, which currently builds `.`, and adds a test for the
existing `localURL` helper.

Scope is the move and the embedding. Do not add game features, a database, new
routes, or a frontend framework.

The spec suggests `internal/http` for the router. Avoid naming a package
`http`: it shadows the standard library `net/http` at every import site. Use a
clearer name such as `internal/server` or `internal/web`.

## Acceptance criteria

- `go build ./...` and `go build -o coverletter ./cmd/server` succeed.
- `go test ./...` passes; `localURL` has a unit test covering a concrete
  address and the wildcard cases (`0.0.0.0`, `[::]`).
- `go run ./cmd/server -open=false` serves the same "Coming up..." page at `/`
  with `Content-Type: text/html; charset=utf-8`.
- The page is served from `web/index.html` through `embed.FS`; editing that file
  and rebuilding changes the response, and the server does not read it from
  disk at runtime.
- The cross-compile command in `deploy.sh` still produces a linux/amd64 binary
  from `./cmd/server`.
- `features/coming-up-page.md` still describes the actual build and run
  commands.

## Verification

Run `go build ./...`, `go test ./...`, `go run ./cmd/server -open=false`, and
`curl -i http://127.0.0.1:8080/`. Then run
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /tmp/coverletter-linux ./cmd/server`.
Confirm the embedded page by changing `web/index.html`, rebuilding, and fetching
`/`; then revert the change. Run `sh workflow/lint.sh`.

## Outcome

Moved the server to `cmd/server`, put the HTTP handler in `internal/server`, and
moved the page to `web/index.html`, embedded through `embed.FS`. `deploy.sh`
now builds `./cmd/server`. Added tests for `localURL` and for the embedded root
page. `features/coming-up-page.md` documents the new layout and commands.

Verified with `go build ./...`, `go test ./...`, `go vet ./...`, a local fetch
of `/` (200, `text/html; charset=utf-8`), a rebuild after changing
`web/index.html` (the marker appeared while the source was restored), and the
linux/amd64 cross-compile from `./cmd/server`. Not deployed to production in
this task.
