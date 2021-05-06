package main

import (
	"flag"
	_ "fmt"
	"net/http"
	"os"
	"strconv"

	_ "net/http/pprof"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {

	//logging setup
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	//debug := flag.Bool("debug", false, "sets log level to debug")

	flag.Parse()

	//zerolog.SetGlobalLevel(zerolog.InfoLevel)
	//if *debug {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	//}

	r := mux.NewRouter()

	//middleware
	r.Use(mw)
	r.Use(requestIDSetter)
	r.Use(sessionDumper)

	//routes
	r.HandleFunc("/ping", ping).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/login", login).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/set-db", setDb).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/execute", execute).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/fetch", fetch).Methods(http.MethodGet, http.MethodOptions)

	http.Handle("/", r)

	log.Info().Msg("prosql-agent Listening at:" + strconv.Itoa(PORT))
	if err := http.ListenAndServe(":"+strconv.Itoa(PORT), nil); err != nil {
		log.Fatal().Msg(err.Error())
		os.Exit(-1)
	}
}
