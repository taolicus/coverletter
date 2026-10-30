package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
)

// page is a black viewport with the message centered on both axes.
const page = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Coming up...</title>
<style>
html, body {
  margin: 0;
  height: 100%;
  background: #000;
  color: #fff;
}
body {
  min-height: 100vh;
  min-height: 100dvh;
  display: grid;
  place-items: center;
  text-align: center;
  font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
p {
  margin: 0;
}
</style>
</head>
<body>
<p>Coming up...</p>
</body>
</html>
`

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	open := flag.Bool("open", true, "open the page in a browser")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}

	url := localURL(ln.Addr())
	fmt.Println(url)
	if *open {
		go openBrowser(url)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})
	log.Fatal(http.Serve(ln, mux))
}

// localURL is an address a browser on this machine can open. A wildcard
// listen address is not itself a destination.
func localURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String()
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("open browser: %v", err)
	}
}
