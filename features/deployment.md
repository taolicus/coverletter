# Deployment

Production is live at <https://coverletter.pifia.net/>. The repository's current
frontend is the lobby described in [lobby](lobby.md); the deployed instance
receives it on the next deploy.

## Host

The VPS is reachable as `ssh tao` (hostname `host.qnwt05-lwsites.com`).
The binary lives at `/home/dev/coverletter/coverletter` and runs as the
`dev` user.

`coverletter.service` (`/etc/systemd/system/coverletter.service`) runs:

```
ExecStart=/home/dev/coverletter/coverletter -addr 127.0.0.1:18084 -open=false
```

It restarts on failure (`Restart=always`, `RestartSec=3`). The drop-in
`/etc/systemd/system/coverletter.service.d/env.conf` loads
`/etc/coverletter.env`, which holds `LOBBY_KEY`. The unit also sets
`Environment=ADDR=127.0.0.1:18084`, but the current binary does not read
that variable; `-addr` is what sets the listen address.

The app listens on loopback only. nginx terminates TLS and proxies to it.
Its SQLite database is `/home/dev/coverletter/coverletter.db` by default,
created on first start. Deployment replaces only the binary, so the database
survives. See [identity and storage](identity-and-storage.md).

## nginx

The vhost is `/etc/nginx/sites-enabled/coverletter.pifia.net`, with
Let's Encrypt certificates under
`/etc/letsencrypt/live/coverletter.pifia.net/`.

- Port 80 redirects `coverletter.pifia.net` to HTTPS.
- Port 443 proxies `/` and `/events` to `http://127.0.0.1:18084`.
- Proxied requests carry `Host`, `X-Real-IP`, `X-Forwarded-For`, and
  `X-Forwarded-Proto`, so the app can mark identity cookies `Secure`.
  Share links are derived from `Host` and `X-Forwarded-Proto`; `-base-url`
  overrides them. Rate limiting trusts `X-Real-IP` because the app is only
  reachable through this proxy.
- `/events` disables proxy buffering and caching for streaming responses.
- Read and send timeouts are 3600 seconds for long-lived connections.

## Continuous integration

`.github/workflows/ci.yml` runs on every push to `main` and every pull request.
On an ubuntu/amd64 runner it checks `gofmt`, runs `go vet ./...`, `go test
./...`, `go test -race ./...`, `sh workflow/lint.sh`, and the
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64` deployment build. The race test runs
there because the android/arm64 dev device does not support the race detector.
CI does not deploy.

## Deploying

`deploy.sh` cross-compiles for linux/amd64
(`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, so it works from the
android/arm64 dev device), creates the remote directory if needed,
copies the binary, and replaces it by rename (never writing a running
executable in place).

```
sh deploy.sh
```

The script is POSIX `sh`. On the dev device the repository sits on a
filesystem without the executable bit, so use `sh deploy.sh` there.

Overridable: `DEPLOY_HOST` (default `tao`), `DEPLOY_DIR` (default
`/home/dev/coverletter`).

The script does not restart `coverletter.service` or check the public URL,
so a deploy leaves the old process running until the service is restarted
by hand. Automating the restart and the post-deploy check is tracked in
`tasks/open/restart-and-verify-deployment.md`.
