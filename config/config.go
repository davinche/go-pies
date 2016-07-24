package config

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/kardianos/osext"
)

type config struct {
	Redis string `json:"redis"`
}

// Config contains configuration to run the app
var Config config

func init() {
	extDir, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatalf("error: could not determine folder of binary")
	}

	data, err := ioutil.ReadFile(extDir + "/config.json")
	if err != nil {
		log.Fatalf("error: could not read config")
	}

	err = json.Unmarshal(data, &Config)
	if err != nil {
		log.Fatalf("error: could not parse config")
	}
}
