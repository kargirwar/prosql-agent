/* Copyright (C) 2021 Pankaj Kargirwar <kargirwar@protonmail.com>

   This file is part of prosql-agent

   prosql-agent is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   prosql-agent is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with prosql-agent.  If not, see <http://www.gnu.org/licenses/>.
*/

package utils

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
	log "github.com/sirupsen/logrus"
)

const ERR_UNRECOVERABLE = "unrecoverable-error"

type Response struct {
	Status    string      `json:"status"`
	Msg       string      `json:"msg"`
	ErrorCode string      `json:"error-code"`
	Data      interface{} `json:"data"`
	Eof       bool        `json:"eof"`
}

var Upgrader = websocket.Upgrader{
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
	log.WithFields(log.Fields{
		"req-id": reqId(ctx),
		"fl":     fl + ":" + strconv.Itoa(line),
	}).Debug(v)
}

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := Upgrader.Upgrade(w, r, nil)
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

func SendError_ws(ctx context.Context, c *websocket.Conn, err error, code string) {
	defer TimeTrack(ctx, time.Now())

	res := &Response{
		Status:    "error",
		Msg:       err.Error(),
		ErrorCode: code,
	}
	str, _ := json.Marshal(res)
	c.WriteMessage(websocket.TextMessage, str)
}

func SendError(ctx context.Context, w http.ResponseWriter, err error, code string) {
	defer TimeTrack(ctx, time.Now())

	res := &Response{
		Status:    "error",
		Msg:       err.Error(),
		ErrorCode: code,
	}
	str, _ := json.Marshal(res)
	fmt.Fprintf(w, string(str))
}

func SendSuccess(ctx context.Context, w http.ResponseWriter, data interface{}, eof bool) {
	defer TimeTrack(ctx, time.Now())

	res := &Response{
		Status: "ok",
		Data:   data,
		Eof:    eof,
	}
	str, err := json.Marshal(res)
	if err != nil {
		e := errors.New("Unrecoverable error")
		SendError(ctx, w, e, ERR_UNRECOVERABLE)
		return
	}
	fmt.Fprint(w, string(str))
}

type requestIDKey struct{}

//https://stackoverflow.com/a/67388007/1926351
func RequestIDSetter(next http.Handler) http.Handler {
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

func GetContext(r *http.Request) context.Context {
	params := r.URL.Query()

	reqId, present := params["req-id"]
	if present && len(reqId) != 0 {
		return context.WithValue(r.Context(), requestIDKey{}, reqId[0])
	}

	return r.Context()
}
