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
    //r.Use(mux.CORSMethodMiddleware(r))
    r.Use(mw)

	r.HandleFunc("/echo", echo)
	r.HandleFunc("/ping", ping).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":23890", nil))
}
