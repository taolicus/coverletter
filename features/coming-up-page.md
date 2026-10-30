# Coming up page

`coverletter` serves one page: a black background with the white text
"Coming up..." centered in the viewport, horizontally and vertically.

The page lives in `web/index.html` and is embedded into the binary with
`embed.FS`. The command is `cmd/server`; the HTTP handler is in
`internal/server`.

Build and run:

```
go build -o coverletter ./cmd/server
./coverletter
```

It listens on `:8080`, prints a local URL, and opens that URL in the default
browser. `-addr` changes the listen address. `-open=false` skips the browser.

The production copy is described in [deployment](deployment.md).
