package main

import (
	"encoding/json"
	"fmt"
	"github.com/denisbrodbeck/machineid"
	log "github.com/sirupsen/logrus"
	"net/http"
)

const APP_NAME = "local-server"
const VERSION = "1.0"

type response struct {
	Status string `json:"status"`
	Id     string `json:"id"`
}

func main() {
	id, err := machineid.ProtectedID(APP_NAME)
	if err != nil {
		log.Fatal(err)
	}

	res := &response{
		Status: "ok",
		Id:     id,
	}
	str, _ := json.Marshal(res)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		fmt.Fprintf(w, string(str))
	})
	http.ListenAndServe(":23890", nil)
}
