package main

import (
	_ "fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"

	"flag"

	_ "net/http/pprof"

	"github.com/gorilla/mux"
	_ "github.com/gorilla/websocket"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
var mp *os.File

func main() {
	//profiling
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	// ... rest of the program ...

	if *memprofile != "" {
		mp, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer mp.Close() // error handling omitted for example
		runtime.GC()     // get up-to-date statistics
		//if err := pprof.WriteHeapProfile(f); err != nil {
		//log.Fatal("could not write memory profile: ", err)
		//}
	}

	r := mux.NewRouter()
	r.Use(mw)
	r.Use(sessionDumper)
	r.HandleFunc("/ping", ping).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/login", login).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/set-db", setDb).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/execute", execute).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/fetch", fetch).Methods(http.MethodGet, http.MethodOptions)

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":23890", nil))
}
