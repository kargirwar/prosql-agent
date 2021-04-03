package main

import (
	_ "fmt"
	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"log"
	"net/http"
)

func main() {
	r := mux.NewRouter()
	r.Use(mw)
	r.Use(sessionDumper)

	r.HandleFunc("/echo", echo)
	r.HandleFunc("/ping", ping).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	r.HandleFunc("/login", login).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	r.HandleFunc("/check", check).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":23890", nil))
}
