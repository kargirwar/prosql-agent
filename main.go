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

package main

import (
	_ "fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"github.com/kargirwar/prosql-agent/utils"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

const LOG_FILE = "prosql.log"

var logger *lumberjack.Logger

func init() {
	log.SetFormatter(&log.JSONFormatter{
		TimestampFormat: time.StampMilli,
	})

	logger = &lumberjack.Logger{
		Filename:   getLogFileName(),
		MaxSize:    10, //megabytes
		MaxBackups: 2,
		MaxAge:     28, //days
		Compress:   true,
	}
	log.SetOutput(logger)

	log.SetLevel(log.DebugLevel)
}

func getLogFileName() string {

	home, err := os.UserHomeDir()
	if err != nil {
		return LOG_FILE
	}

	//Linux
	if runtime.GOOS == "linux" {
		path := home + "/.prosql-agent"
		err := os.MkdirAll(path, 0700)
		if err != nil {
			return LOG_FILE
		}
		return filepath.Join(path, LOG_FILE)
	}

	//OSX
	if runtime.GOOS == "darwin" {
		return home + "/Library/ProsqlAgent/" + LOG_FILE
	}

	return LOG_FILE
}

func main() {

	r := mux.NewRouter()

	//middleware
	r.Use(mw)
	r.Use(utils.RequestIDSetter)
	r.Use(sessionDumper)

	//routes
	r.HandleFunc("/about", about).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/ping", ping).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/login", login).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/query", query).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/execute", execute).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/fetch", fetch).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/fetch_ws", fetch_ws).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/cancel", cancel).Methods(http.MethodGet, http.MethodOptions)

	http.Handle("/", r)

	log.Info("prosql-agent Listening at:" + strconv.Itoa(PORT))

	if err := http.ListenAndServe(":"+strconv.Itoa(PORT), nil); err != nil {
		log.Fatal(err.Error())
		os.Exit(-1)
	}
}

func mw(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", ALLOW)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
		w.Header().Set("Access-Control-Allow-Headers", "X-Request-ID")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(w, r)
	})
}
