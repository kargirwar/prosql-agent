package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		//todo: enforce some rules here
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

func mw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://dev.prosql.io")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sendError(w http.ResponseWriter, err error, code string) {
	res := &Response{
		Status:    "error",
		Msg:       err.Error(),
		ErrorCode: code,
	}
	str, _ := json.Marshal(res)
	fmt.Fprintf(w, string(str))
}

func sendSuccess(w http.ResponseWriter, data interface{}, eof bool) {
	res := &Response{
		Status: "ok",
		Data:   data,
		Eof:    eof,
	}
	str, err := json.Marshal(res)
	if err != nil {
		e := errors.New("Unrecoverable error")
		sendError(w, e, ERR_UNRECOVERABLE)
		return
	}
	fmt.Fprintf(w, string(str))
}
