package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/kardianos/osext"

	"github.com/davinche/gpies/api"
	"github.com/davinche/gpies/config"
	"github.com/davinche/gpies/pie"
	"github.com/garyburd/redigo/redis"
)

// FromURL ingests data into redis via the data from the s3 bucket
func FromURL(url string) {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("error: could not ingest from pies.json: err=%q\n", err)
	}
	ingest(resp.Body)
}

// FromFile ingests data from the pies.json file from disk.
// Pies.json was obtained from the link in the bakeoff.
func FromFile() {

	execDir, err := osext.ExecutableFolder()

	if err != nil {
		log.Fatalf("error: could not determine path for pies.json: err=%q\n", err)
	}

	piesJSON := execDir + "/pies.json"

	file, err := os.Open(piesJSON)
	if err != nil {
		log.Fatalf("error: could not ingest pies.json: err=%q\n", err)
	}
	ingest(file)
}

func ingest(r io.ReadCloser) {
	defer r.Close()

	// Create the pie struct to deserialize into
	pStruct := struct {
		Pies pie.Pies `json:"pies"`
	}{}

	decoder := json.NewDecoder(r)
	err := decoder.Decode(&pStruct)
	if err != nil {
		log.Fatalf("error: could not decode pies.json: err=%q\n", err)
	}

	// Connect to redis
	conn, err := redis.Dial("tcp", config.Config.Redis)
	if err != nil {
		log.Fatalf("error: could not connect to redis: err=%q\n", err)
	}

	// Flush Redis
	_, err = conn.Do("flushall")
	if err != nil {
		log.Fatalf("error: could not flush redis: err=%q\n", err)
	}

	// Create the pies
	err = createPies(conn, pStruct.Pies)
	if err != nil {
		log.Fatalf("error: could not create pies in redis: err=%q\n", err)
	}
}

// createPies creates a hash entry for each
func createPies(conn redis.Conn, pies pie.Pies) error {

	// Serialize all the pies as json
	piesSerialized, err := json.Marshal(pies)
	if err != nil {
		return err
	}
	_, err = conn.Do("SET", api.PiesJSONKey, piesSerialized)
	if err != nil {
		return err
	}

	// Go through each pie and set the approriate pie information / indexes
	for _, p := range pies {

		pieIDString := strconv.FormatUint(p.ID, 10)
		key := fmt.Sprintf(api.PieKey, pieIDString)
		hkey := fmt.Sprintf(api.HPieKey, pieIDString)
		slicesKey := fmt.Sprintf(api.PieSlicesKey, pieIDString)

		// Marshal the pie
		serialized, err := json.Marshal(p)
		if err != nil {
			return err
		}

		// Add Pies to Redis
		conn.Send("MULTI")
		conn.Send("SET", key, serialized)
		conn.Send("SET", slicesKey, p.Slices)
		conn.Send("SADD", api.PiesAvailableKey, pieIDString)
		conn.Send("SADD", api.PiesTotalKey, pieIDString)

		// Set the labels
		for _, l := range p.Labels {
			lName := fmt.Sprintf(api.LabelKey, l)
			conn.Send("SADD", lName, pieIDString)
		}

		// Set the hash attributes of the pie
		conn.Send(
			"HMSET", hkey,
			"id", pieIDString,
			"name", p.Name,
			"imageURL", p.ImageURL,
			"price", strconv.FormatFloat(p.Price, 'f', -1, 64),
		)

		// Execute!
		_, err = conn.Do("EXEC")
		if err != nil {
			return err
		}
		log.Printf("activity: create pie: id=%d", p.ID)
	}
	return nil
}
