package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/sevlyar/go-daemon"
)

func main() {
	ctx := &daemon.Context{
		PidFileName: "sample.pid",
		PidFilePerm: 0644,
		LogFileName: "sample.log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{"[go-daemon sample]"},
	}

	d, err := ctx.Reborn()
	if err != nil {
		log.Fatal("Unable to run: ", err)
	}

	if d != nil {
		return
	}

	defer ctx.Release()
	log.Print("- - - - - - - - - - - - - - -\n")
	log.Print("daemon started\n")

	go func() {

	}()

	serveHTTP()
}

func serveHTTP() {
	http.HandleFunc("/", httpHandler)
	http.ListenAndServe("127.0.0.1:8080", nil)
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("request from %s: %s %q", r.RemoteAddr, r.Method, r.URL)
	fmt.Fprintf(w, "go-daemon: %q", html.EscapeString(r.URL.Path))
}
