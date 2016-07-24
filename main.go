package main

import (
	"log"
	"net/http"

	"github.com/davinche/gpies/api"
	"github.com/davinche/gpies/ingest"
	"github.com/dimfeld/httptreemux"
)

func main() {
	ingest.FromFile()
	router := httptreemux.New()
	api.Handle("/", router)
	log.Fatal(http.ListenAndServe(":31415", router))
}
