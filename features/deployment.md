# Deployment

The production binary lives at `/home/dev/coverletter/coverletter` on the
VPS reachable as `ssh tao`. Deployment is just copying that binary. The
host is not set up to serve it yet: there is no `coverletter.service`
and no nginx vhost.

## Deploying

`deploy.sh` cross-compiles for linux/amd64
(`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, so it works from the
android/arm64 dev device), creates the remote directory if needed,
copies the binary, and replaces it by rename (never writing a running
executable in place). It does not restart a service or check a public
URL, because neither exists yet.

```
sh deploy.sh
```

The script is POSIX `sh`. On the dev device the repository sits on a
filesystem without the executable bit, so use `sh deploy.sh` there.

Overridable: `DEPLOY_HOST` (default `tao`), `DEPLOY_DIR` (default
`/home/dev/coverletter`).

## Host setup

Not done. Asteroids, the model for this layout, serves
`/home/dev/asteroids/asteroids` through `asteroids.service` and nginx
at `asteroids.pifia.net`. Coverletter has only the directory and the
binary. Hostname and listen port are undecided.
