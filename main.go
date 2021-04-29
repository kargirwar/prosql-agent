package main

import (
	_ "fmt"
	"log"
	"net/http"

	_ "net/http/pprof"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func main() {
	r := mux.NewRouter()
	r.Use(mw)
	r.Use(sessionDumper)
	r.HandleFunc("/ping", ping).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/login", login).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/execute", execute).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/fetch", fetch).Methods(http.MethodGet, http.MethodOptions)

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":23890", nil))
}
