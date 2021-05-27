package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		//todo: enforce some rules here
		return true
	},
}

func TimeTrackTest(start time.Time) {
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

func TimeTrack(ctx context.Context, start time.Time) {
	elapsed := time.Since(start)

	// Skip this function, and fetch the PC and file for its parent.
	pc, _, _, _ := runtime.Caller(1)

	// Retrieve a function object this functions parent.
	funcObj := runtime.FuncForPC(pc)

	// Regex to extract just the function name (and not the module path).
	runtimeFunc := regexp.MustCompile(`^.*\.(.*)$`)
	name := runtimeFunc.ReplaceAllString(funcObj.Name(), "$1")

	//log.Print(fmt.Sprintf("%s took %s", name, elapsed))
	Dbg(ctx, fmt.Sprintf("%s took %s", name, elapsed))
}

func Dbg(ctx context.Context, v string) {
	_, fl, line, _ := runtime.Caller(1)
	log.Debug().
		Str("req-id", reqId(ctx)).
		Str("fl", fl+":"+strconv.Itoa(line)).
		Str("m", v).Msg("")
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
			//log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			//log.Println("write:", err)
			break
		}
	}
}

func sessionDumper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys := sessionStore.getKeys()
		Dbg(r.Context(), fmt.Sprintf("Found :%d sessions", len(keys)))

		for _, sk := range keys {
			Dbg(r.Context(), fmt.Sprintf("s: %s\n", sk))
			s, _ := sessionStore.get(sk)
			if s == nil {
				Dbg(r.Context(), fmt.Sprintf("session "+sk+" is nil"))
				continue
			}
			ckeys := s.cursorStore.getKeys()
			for _, ck := range ckeys {
				c, _ := s.cursorStore.get(ck)
				if c == nil {
					Dbg(r.Context(), fmt.Sprintf("cursor "+ck+" is nil"))
					continue
				}

				Dbg(r.Context(), fmt.Sprintf("    c: %s query: %s\n", ck, c.query))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func mw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://dev.prosql.io")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "X-Request-ID")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sendError_ws(ctx context.Context, c *websocket.Conn, err error, code string) {
	defer TimeTrack(ctx, time.Now())

	res := &Response{
		Status:    "error",
		Msg:       err.Error(),
		ErrorCode: code,
	}
	str, _ := json.Marshal(res)
	c.WriteMessage(websocket.TextMessage, str)
}

func sendError(ctx context.Context, w http.ResponseWriter, err error, code string) {
	defer TimeTrack(ctx, time.Now())

	res := &Response{
		Status:    "error",
		Msg:       err.Error(),
		ErrorCode: code,
	}
	str, _ := json.Marshal(res)
	fmt.Fprintf(w, string(str))
}

func sendSuccess(ctx context.Context, w http.ResponseWriter, data interface{}, eof bool) {
	defer TimeTrack(ctx, time.Now())

	res := &Response{
		Status: "ok",
		Data:   data,
		Eof:    eof,
	}
	str, err := json.Marshal(res)
	if err != nil {
		e := errors.New("Unrecoverable error")
		sendError(ctx, w, e, ERR_UNRECOVERABLE)
		return
	}
	fmt.Fprint(w, string(str))
}

type requestIDKey struct{}

//https://stackoverflow.com/a/67388007/1926351
func requestIDSetter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		ctx := context.WithValue(r.Context(), requestIDKey{}, reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func reqId(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey{}).(string)
	if ok {
		return id
	}

	return "req-id"
}

func getContext(r *http.Request) context.Context {
	params := r.URL.Query()

	reqId, present := params["req-id"]
	if present && len(reqId) != 0 {
		return context.WithValue(r.Context(), requestIDKey{}, reqId[0])
	}

	return r.Context()
}
