package main

import (
	_ "fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

const LOG_FILE = "prosql.log"
const APP_NAME = "prosql-agent"

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

	//OSX
	if runtime.GOOS == "darwin" {
		return home + "/Library/Prosql/" + LOG_FILE
	}

	return LOG_FILE
}

func main() {

	r := mux.NewRouter()

	//middleware
	r.Use(mw)
	r.Use(requestIDSetter)
	r.Use(sessionDumper)

	//routes
	r.HandleFunc("/about", about).Methods(http.MethodGet, http.MethodOptions)
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
