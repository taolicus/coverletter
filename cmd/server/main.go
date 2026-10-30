package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/taolicus/coverletter/internal/players"
	"github.com/taolicus/coverletter/internal/server"
	"github.com/taolicus/coverletter/internal/storage"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "coverletter.db", "SQLite database file")
	open := flag.Bool("open", true, "open the page in a browser")
	flag.Parse()

	db, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}

	url := localURL(ln.Addr())
	fmt.Println(url)
	if *open {
		go openBrowser(url)
	}

	log.Fatal(http.Serve(ln, server.New(players.New(db))))
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
