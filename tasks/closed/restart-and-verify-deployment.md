---
area: deployment
---

# Restart and verify the service after a deploy

## Context

Production is live at <https://coverletter.pifia.net/> through
`coverletter.service` and nginx, but `deploy.sh` only copies the binary.
Because the running process keeps its old image after the file is replaced,
a deploy has no effect until someone restarts the unit by hand, and nothing
checks that the new binary actually serves the public URL.

`ssh tao` logs in as root, so the script can restart the unit directly.
`features/deployment.md` describes the current host layout.

## Acceptance criteria

- `sh deploy.sh` restarts `coverletter.service` after replacing the binary.
- The script fails with a nonzero exit status if the service is not active
  after the restart, and prints enough output to diagnose it.
- The script fetches the public URL after the restart and fails if it does
  not return the expected success response.
- `features/deployment.md` describes the new deploy behaviour, and the
  follow-up note about the missing restart is removed.

## Verification

Run `sh deploy.sh` from the dev device with `ssh tao` available. Confirm
that `systemctl is-active coverletter` reports `active` and that the public
URL returns HTTP 200. Confirm the failure path by running the post-deploy
check against an unreachable URL and observing a nonzero exit.

## Outcome

`deploy.sh` now uploads with `rsync` (compressed, checksummed, 60-second I/O
timeout) instead of `scp`, so a slow or interrupted transfer cannot leave a
truncated binary without a clear failure. It replaces the binary by rename,
restarts `coverletter.service`, waits up to 15 seconds for the unit to become
active, and retries the public URL for up to 10 seconds. A failed restart prints
`systemctl status`; a failed URL check prints the status and the request error or
response body, then exits nonzero. The script also ensures the remote directory
is owned by `dev:dev`, because the service must be able to create its SQLite
database there. `DEPLOY_URL` overrides the public check.

Updated `features/deployment.md` with the rsync upload, restart and health-check
behaviour, the directory ownership requirement, and the new `DEPLOY_OWNER` and
`DEPLOY_URL` overrides.

Verification: `sh -n deploy.sh` and `sh workflow/lint.sh` pass. A successful
`sh deploy.sh` run left `coverletter.service` active and
<https://coverletter.pifia.net/> returned HTTP 200; the service also created
`/home/dev/coverletter/coverletter.db` and served the game-interface assets. The
failure path was exercised with
`DEPLOY_URL=http://127.0.0.1:9/ sh deploy.sh`: it restarted the service, reported
`HTTP 000` with `curl: (7) Failed to connect`, and exited 1. A final normal
deploy restored the verified state. The inactive-service failure branch was not
separately simulated; it shares the same status-report and nonzero-exit path.
Nothing remains open for this task.
