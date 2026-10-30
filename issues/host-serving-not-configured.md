# The coverletter host does not serve the binary yet

## What was observed

`deploy.sh` copies a linux/amd64 binary to `/home/dev/coverletter/coverletter`
on `ssh tao`. `systemctl list-unit-files 'coverletter*'` returns nothing,
and no file under `/etc/nginx/sites-enabled` mentions coverletter. Other
apps on that host are served by a systemd unit running as `dev` plus an
nginx vhost; asteroids is the model (`asteroids.service`,
`asteroids.pifia.net`, binary in `/home/dev/asteroids`).

## Open questions

- Which hostname should serve it?
- Which localhost port should the unit bind?

## What would resolve it

A systemd unit and nginx vhost, in the same shape as asteroids, and
`deploy.sh` restarting the service and checking the public URL.

## Affects

`deploy.sh`, `features/deployment.md`
