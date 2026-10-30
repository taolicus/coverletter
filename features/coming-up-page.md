# Coming up page

`coverletter` serves one page: a black background with the white text
"Coming up..." centered in the viewport, horizontally and vertically.

Build and run:

```
go build -o coverletter .
./coverletter
```

It listens on `:8080`, prints a local URL, and opens that URL in the default
browser. `-addr` changes the listen address. `-open=false` skips the browser.
