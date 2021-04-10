package main

import (
	_ "fmt"
	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

func main() {
	r := mux.NewRouter()
	//r.Use(mw)
	//r.Use(sessionDumper)

	//r.HandleFunc("/echo", echo)
	//r.HandleFunc("/ping", ping).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	//r.HandleFunc("/login", login).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	//r.HandleFunc("/execute", execute).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	r.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		go test()
	})

	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":23890", nil))
}

func test() {
	for {
		log.Println("I am alive\n")
		time.Sleep(2 * time.Second)
	}
}
