package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func TimeTrack(start time.Time) {
	elapsed := time.Since(start)

	// Skip this function, and fetch the PC and file for its parent.
	pc, _, _, _ := runtime.Caller(1)

	// Retrieve a function object this functions parent.
	funcObj := runtime.FuncForPC(pc)

	// Regex to extract just the function name (and not the module path).
	runtimeFunc := regexp.MustCompile(`^.*\.(.*)$`)
	name := runtimeFunc.ReplaceAllString(funcObj.Name(), "$1")

	log.Print(fmt.Sprintf("%s took %s", name, elapsed))
}

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
			log.Print("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Print("write:", err)
			break
		}
	}
}

func sessionDumper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys := sessionStore.getKeys()
		log.Printf("Found :%d sessions\n", len(keys))

		for _, sk := range keys {
			log.Printf("s: %s\n", sk)
			s, _ := sessionStore.get(sk)
			if s == nil {
				log.Print("session " + sk + " is nil")
				continue
			}
			ckeys := s.cursorStore.getKeys()
			for _, ck := range ckeys {
				c, _ := s.cursorStore.get(ck)
				if c == nil {
					log.Print("cursor " + ck + " is nil")
					continue
				}

				log.Printf("    c: %s\n", ck)
			}
		}
		next.ServeHTTP(w, r)
	})
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
