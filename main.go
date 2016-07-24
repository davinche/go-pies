package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/davinche/gpies/api"
	"github.com/davinche/gpies/ingest"
	"github.com/dimfeld/httptreemux"
)

func main() {
	shouldIngest := flag.Bool("i", false, "Ingestion: specify this boolean to repopulate Redis")
	ingestURL := flag.String("s", "", "Ingestion URL: specify the URL that contains the JSON to be ingested")
	flag.Parse()

	if *shouldIngest {
		if *ingestURL != "" {
			ingest.FromURL(*ingestURL)
		} else {
			ingest.FromFile()
		}
	}

	router := httptreemux.New()
	api.Handle("/", router)
	log.Fatal(http.ListenAndServe(":31415", router))
}
