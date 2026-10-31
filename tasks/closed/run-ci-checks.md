---
area: ci
---

# Run the project checks in GitHub Actions

## Context

The repository has no `.github/workflows`, so formatting, vet, tests, workflow
lint, and the deployment build are only checked when someone remembers to run
them. The android/arm64 dev device cannot run `go test -race` (the Go toolchain
reports that `-race` is unsupported on that platform), so race coverage currently
depends on a different machine.

GitHub Actions provides an ubuntu/amd64 runner for every push to `main` and every
pull request. This task adds one workflow for the checks the project already
relies on; it does not add new test frameworks or change application behaviour.

## Acceptance criteria

- `.github/workflows/ci.yml` runs on pushes to `main` and on pull requests.
- The workflow uses the Go version in `go.mod` and caches module/build downloads.
- It checks `gofmt`, runs `go vet ./...`, `go test ./...`, and `go test -race
  ./...`.
- It runs `sh workflow/lint.sh` and the same `CGO_ENABLED=0` linux/amd64 build
  used by `deploy.sh`.
- A run on `main` finishes successfully and is visible in GitHub Actions.
- `features/deployment.md` records what CI checks and why the race test belongs
  there rather than on the dev device.

## Verification

Push the workflow to `main` and inspect the resulting run with `gh run list` and
`gh run view`. Run the same commands locally where the environment permits; the
race test is expected to be skipped locally because android/arm64 does not
support it.

## Outcome

Added `.github/workflows/ci.yml`. It runs on pushes to `main` and pull requests
on `ubuntu-latest`, uses `actions/checkout@v7` and `actions/setup-go@v7` with the
Go version from `go.mod` and module/build caching, and checks `gofmt`, `go vet
./...`, `go test ./...`, `go test -race ./...`, `sh workflow/lint.sh`, and the
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64` build used by `deploy.sh`. Concurrent
runs for the same ref are cancelled.

Run `41194472880` on `main` (commit `9269a26`) passed in 1m56s, including the
race test that this android/arm64 device cannot run. The only annotation was
GitHub's notice that the `ubuntu-latest` label migrates to Ubuntu 26 in October
2026; it does not affect the run. `features/deployment.md` now records what CI
checks and why the race test belongs there. CI does not deploy.
