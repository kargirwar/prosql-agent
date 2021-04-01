package main

import (
	"log"
	"fmt"
	"net/http"
	"github.com/gorilla/websocket"
	"github.com/gorilla/mux"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
} 

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "https://dev.prosql.io")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	fmt.Fprintf(w, `{"status":"ok"}`)
}

func main() {
    r := mux.NewRouter()
    r.HandleFunc("/echo", echo)
    r.HandleFunc("/ping", ping)
    http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":23890", nil))
}
