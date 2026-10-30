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

Filled in when the task is closed: what was done, what was verified, what was
not, and what remains.
