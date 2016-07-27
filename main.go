package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/davinche/gpies/api"
	"github.com/davinche/gpies/ingest"
	"github.com/dimfeld/httptreemux"
)

func main() {
	shouldIngest := flag.Bool("i", false, "Ingestion: specify this boolean to repopulate Redis")
	ingestURL := flag.String("s", "", "Ingestion URL: specify the URL that contains the JSON to be ingested")
	verbose := flag.Bool("v", false, "Verbose: specify to enable logging")
	flag.Parse()

	if !*verbose {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

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
