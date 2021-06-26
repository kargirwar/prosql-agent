package main

import (
	_ "fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "net/http/pprof"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *lumberjack.Logger

func init() {
	log.SetFormatter(&log.JSONFormatter{
		TimestampFormat: time.StampMilli,
	})

	logger = &lumberjack.Logger{
		Filename:   "prosql.log",
		MaxSize:    10, // megabytes
		MaxBackups: 2,
		MaxAge:     28,   //days
		Compress:   true, // disabled by default
	}
	log.SetOutput(logger)

	log.SetLevel(log.DebugLevel)
}

func main() {

	r := mux.NewRouter()

	//middleware
	r.Use(mw)
	r.Use(requestIDSetter)
	r.Use(sessionDumper)

	//routes
	r.HandleFunc("/ping", ping).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/login", login).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/query", query).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/execute", execute).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/query_ws", query_ws).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/fetch", fetch).Methods(http.MethodGet, http.MethodOptions)

	http.Handle("/", r)

	log.Info("prosql-agent Listening at:" + strconv.Itoa(PORT))

	if err := http.ListenAndServe(":"+strconv.Itoa(PORT), nil); err != nil {
		log.Fatal(err.Error())
		os.Exit(-1)
	}
}
