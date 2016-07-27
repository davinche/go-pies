package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/davinche/gpies/config"
	"github.com/davinche/gpies/pie"
	"github.com/dimfeld/httptreemux"
	"github.com/garyburd/redigo/redis"
)

// Redis Connection Pool
var pool *redis.Pool

// Create a redis pool connection on API init
func init() {
	pool = &redis.Pool{
		MaxIdle:   80,
		MaxActive: 1000,
		Dial: func() (redis.Conn, error) {
			redisOpts := []redis.DialOption{}
			if config.Config.RedisPassword != "" {
				redisOpts = append(redisOpts, redis.DialPassword(config.Config.RedisPassword))
			}
			c, err := redis.Dial("tcp", config.Config.Redis, redisOpts...)
			if err != nil {
				log.Fatalf("error: could not create redis connection pool: err=%q\n", err)
			}
			return c, err
		},
	}
}

// Handle takes a prefix (the prefix route for the API) and registers
// functions that will handle the API requests
func Handle(prefix string, r *httptreemux.TreeMux) {
	api := r.NewGroup(prefix)
	api.GET("/pies", getPies)
	api.GET("/pies/:id", getPie)
	api.GET("/pies/recommend", getRecommended)
	api.POST("/pies/:id/purchases", purchasePie)
}

// getPies returns the list of all pies
func getPies(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	conn := pool.Get()
	defer conn.Close()

	// Get all the pies
	piesBytes, err := redis.Bytes(conn.Do("GET", PiesJSONKey))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("error: could not get pies from Redis: err=%q\n", err)
		return
	}

	// Unmarshall
	pies := pie.Pies{}
	err = json.Unmarshal(piesBytes, &pies)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("error: could not unmarshall pies: err=%q\n", err)
		return
	}

	for _, p := range pies {
		// Grab remainig slices for the pie
		slicesKey := fmt.Sprintf(PieSlicesKey, strconv.FormatUint(p.ID, 10))
		slices, err := redis.Int(conn.Do("GET", slicesKey))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("error: could not get pie slices: err=%q\n", err)
			return
		}
		p.Slices = slices
		p.Permalink = "http://" + r.Host + "/pies/" + strconv.FormatUint(p.ID, 10)
	}

	PiesList.Execute(w, pies)
}

// getPie returns the information for a single pie
func getPie(w http.ResponseWriter, r *http.Request, params map[string]string) {
	conn := pool.Get()
	defer conn.Close()
	pieID := params["id"]
	isJSON := false
	if strings.HasSuffix(pieID, ".json") {
		isJSON = true
		pieID = pieID[:strings.LastIndex(pieID, ".")]
	}

	// Redis Keys that we need
	key := fmt.Sprintf(PieKey, pieID)
	slicesKey := fmt.Sprintf(PieSlicesKey, pieID)
	piePurchasersKey := fmt.Sprintf(PiePurchasersKey, pieID)
	log.Printf("debug: pieKey=%q, slicesKey=%q, purchasersKey=%q\n", key, slicesKey, piePurchasersKey)

	// Pie to eventually serialize
	details := pie.Details{
		Purchases: []*pie.Purchases{},
	}

	conn.Send("MULTI")
	conn.Send("GET", key)
	conn.Send("GET", slicesKey)
	conn.Send("SMEMBERS", piePurchasersKey)
	resp, err := redis.Values(conn.Do("EXEC"))
	if err != nil {
		redisError(w, err)
		return
	}

	// Get the pie data from redis
	pieBytes, err := redis.Bytes(resp[0], nil)
	if err != nil {
		redisError(w, err)
		return
	}

	// Unmarshall into pie object
	err = json.Unmarshal(pieBytes, &details)
	if err != nil {
		redisError(w, err)
		return
	}

	// Get the number of slices
	slices, err := redis.Int(resp[1], nil)
	if err != nil {
		redisError(w, err)
		return
	}

	// get purchaser IDs
	members, err := redis.Values(resp[2], nil)
	if err != nil {
		redisError(w, err)
		return
	}

	// Get number of slices by each purchaser
	for _, member := range members {
		memberName, err := redis.String(member, nil)
		if err != nil {
			redisError(w, err)
			return
		}
		purchasesKey := fmt.Sprintf(PurchaseKey, pieID, memberName)
		numSlices, err := redis.Int(conn.Do("GET", purchasesKey))
		if err != nil {
			redisError(w, err)
			return
		}
		details.Purchases = append(details.Purchases, &pie.Purchases{
			Username: memberName,
			Slices:   numSlices,
		})
	}

	// serializes
	details.Pie.Slices = 0
	details.RemainingSlices = slices

	// showing json? Or rendering template
	if isJSON {
		encodeJSON(w, details, nil)
		return
	}
	PiesSingle.Execute(w, details)
}

// getRecommended gets a recommended pie for a given user
func getRecommended(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	conn := pool.Get()
	defer conn.Close()

	// Get the from data
	username := r.FormValue("username")
	budget := r.FormValue("budget")
	labelsStr := r.FormValue("labels")

	// Split labels by delimiter ","
	var labels []string
	if labelsStr != "" {
		labels = strings.Split(labelsStr, ",")
	}

	log.Printf("debug: username=%q, budget=%q, labels=%q\n", username, budget, labelsStr)

	// List of sets we are going to intersect with to narrow down the pies
	// we can recommend to the user
	query := []interface{}{
		PiesAvailableKey,
	}

	// Check to see if there is a list of pies available to a user
	userAvailableKey := fmt.Sprintf(UserAvailableKey, username)
	exists, err := redis.Bool(conn.Do("EXISTS", userAvailableKey))
	if err != nil {
		redisError(w, err)
		return
	}

	// Filter by pies available to current user if possible
	log.Printf("debug: userAvailableKey=%v\n", userAvailableKey)
	if exists {
		query = append(query, userAvailableKey)
	}

	// Figure out which labels to filter by
	if len(labels) > 0 {
		for _, label := range labels {
			query = append(query, fmt.Sprintf(LabelKey, label))
		}
	}

	log.Printf("debug: query=%v\n", query)

	// Query redis for the intersecting pies
	recommendedPieIDs, err := redis.Values(conn.Do("SINTER", query...))
	if err != nil {
		redisError(w, err)
		return
	}

	// Are there any pies to recommend
	if len(recommendedPieIDs) == 0 {
		noRecommended(w)
		return
	}

	// Get all the pies to recommend
	listOfPies := make(pie.RecommendPies, len(recommendedPieIDs))

	// Get the pie details
	for index, id := range recommendedPieIDs {
		pKey := fmt.Sprintf(HPieKey, id)
		values, err := redis.Values(conn.Do("HMGET", pKey, "id", "price"))
		if err != nil {
			redisError(w, err)
			return
		}

		id, err := redis.Uint64(values[0], nil)
		if err != nil {
			errMsg := fmt.Sprintf("error: could not get pie id: err=%q", err)
			log.Println(errMsg)
			encodeError(w, errMsg)
			return
		}

		price, err := redis.Float64(values[1], nil)
		if err != nil {
			errMsg := fmt.Sprintf("error: could not get pie price: err=%q", err)
			log.Println(errMsg)
			encodeError(w, errMsg)
			return
		}

		pieObj := &pie.RecommendPie{
			ID:    id,
			Price: price,
		}

		listOfPies[index] = pieObj
	}

	// Sort by budget
	if budget == "cheap" {
		sort.Sort(listOfPies)
	} else {
		sort.Sort(sort.Reverse(listOfPies))
	}

	recommend(w, r, listOfPies[0])
}

// getPurchaseParams validates an incoming request and ensures that all
// required data is provided
func getPurchaseParams(r *http.Request) (username string, amount float64, slices int, errors []string) {
	// Get purchase information
	username = r.PostFormValue("username")
	amountStr := r.PostFormValue("amount")
	slicesStr := r.PostFormValue("slices")

	// make sure all data is there
	if username == "" || amountStr == "" {
		errorFmt := "error: missing information: %s"
		if username == "" {
			errors = append(errors, fmt.Sprintf(errorFmt, "missing username"))
		}

		if amountStr == "" {
			errors = append(errors, fmt.Sprintf(errorFmt, "missing amount"))
		}

		return "", 0, 0, errors
	}

	if slicesStr == "" {
		slicesStr = "1"
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		errors = append(errors, "amount is not a decimal")
	}

	slices, err = strconv.Atoi(slicesStr)
	if err != nil {
		errors = append(errors, "error: slices is not an integer")
	}

	if errors != nil {
		return "", 0, 0, errors
	}
	return username, amount, slices, nil
}

// purchasePie is the endpoint that allows users to purchase the pie
func purchasePie(w http.ResponseWriter, r *http.Request, params map[string]string) {
	conn := pool.Get()
	defer conn.Close()

	pieID := params["id"]
	key := fmt.Sprintf(PieKey, pieID)
	hkey := fmt.Sprintf(HPieKey, pieID)
	slicesKey := fmt.Sprintf(PieSlicesKey, pieID)
	piePurchasersKey := fmt.Sprintf(PiePurchasersKey, pieID)

	// Make sure the pie exists
	exists, err := redis.Bool(conn.Do("EXISTS", key))
	if err != nil {
		redisError(w, err)
		return
	}

	// Return not found if the requested pie does not exist
	if !exists {
		http.NotFound(w, r)
		return
	}

	// get the parameters
	username, amount, wantedSlices, errors := getPurchaseParams(r)
	if errors != nil {
		encodeBadRequest(w, errors...)
		return
	}

	// Simple check for gluttony if slices > 3
	if wantedSlices > 3 {
		log.Printf("debug: gluttony: wanted=%d\n", wantedSlices)
		gluttony(w)
		return
	}

	// Check purchases to see how many slices the user already bought
	purchasesKey := fmt.Sprintf(PurchaseKey, pieID, username)
	purchases, err := conn.Do("GET", purchasesKey)
	if err != nil {
		redisError(w, err)
		return
	}

	// Make sure number of existing purchases + new purchases does not exceed 3
	if purchases != nil {
		numPurchases, err := redis.Int(purchases, nil)
		if err != nil {
			errMsg := fmt.Sprintf("error: could not determine number of purchases: user=%q, value=%v", username, purchases)
			log.Println(errMsg)
			encodeError(w, errMsg)
			return
		}

		if numPurchases+wantedSlices > 3 {
			log.Printf("debug: gluttony: wanted=%d, numPurchasedExisting=%d\n\n", wantedSlices, numPurchases)
			gluttony(w)
			return
		}
	}

	// Check for remaining slices
	remainingSlices, err := redis.Int(conn.Do("GET", slicesKey))
	if err != nil {
		redisError(w, err)
		return
	}

	// see if we have enough slices to sell to the user
	if remainingSlices == 0 {
		gone(w, nil)
		return
	}

	if wantedSlices > remainingSlices {
		log.Printf("debug: gone: remaining=%d, wanted=%d\n", remainingSlices, wantedSlices)
		gone(w, "not enough remaining slices")
		return
	}

	// Check the maths
	pricePerSlice, err := redis.Float64(conn.Do("HGET", hkey, "price"))
	if err != nil {
		redisError(w, err)
		return
	}

	if int(pricePerSlice*float64(wantedSlices)*100) != int(amount*100) {
		log.Printf("debug: wrong math: amount=%f, calculated=%f\n", amount, pricePerSlice*float64(wantedSlices))
		wrongMaths(w)
		return
	}

	// ------------------------------------------------------------------------
	// Attempt to PURCHASE
	// ------------------------------------------------------------------------
	// 1. Lock (watch)
	// 2. Check all params to make sure the user can still update
	// 3. If all goes well, update
	// ------------------------------------------------------------------------
	userAvailableKey := fmt.Sprintf(UserAvailableKey, username)
	userUnavailableKey := fmt.Sprintf(UserUnavailableKey, username)
	var transactionError error
	for i := 0; i < 5; i++ {
		// TODO: sleep maybe for exponential backoff?
		_, err := conn.Do("WATCH", slicesKey, piePurchasersKey, purchasesKey, userAvailableKey, PiesAvailableKey)
		if err != nil {
			transactionError = err
			continue
		}

		remainingSlices, err := redis.Int(conn.Do("GET", slicesKey))
		if err != nil {
			transactionError = err
			continue
		}

		purchasedSlices := 0
		nSlices, err := conn.Do("GET", purchasesKey)
		if err != nil {
			transactionError = err
			continue
		}

		if nSlices != nil {
			purchasedSlices, err = redis.Int(nSlices, nil)
			if err != nil {
				log.Printf("error: could not type assert num purchased slices: err=%q\n", err)
				encodeError(w, "error: could not complete purchase")
				return
			}
		}

		// Make sure we are within our limit
		if (purchasedSlices + wantedSlices) > 3 {
			conn.Do("UNWATCH")
			gluttony(w)
			return
		}

		// Make sure we actually have enough slices
		if remainingSlices < wantedSlices {
			conn.Do("UNWATCH")
			gone(w, nil)
			return
		}

		// Perform the transaction
		conn.Send("MULTI")
		conn.Send("DECRBY", slicesKey, wantedSlices)
		conn.Send("INCRBY", purchasesKey, wantedSlices)
		conn.Send("SADD", piePurchasersKey, username)

		// Check to see if the pie is still available?
		if (remainingSlices - wantedSlices) == 0 {
			conn.Send("SREM", PiesAvailableKey, pieID)
		}

		// Check to see if the user can still buy more
		if (purchasedSlices + wantedSlices) == 3 {
			conn.Send("SADD", userUnavailableKey, pieID)
			conn.Send("SDIFFSTORE", userAvailableKey, PiesAvailableKey, userUnavailableKey)
		}

		reply, err := conn.Do("EXEC")
		if err != nil {
			transactionError = err
			continue
		}

		if reply != nil {
			log.Printf("debug: success purchase: user=%q, wanted=%d, remaining=%d, newRemaining=%d\n",
				username, wantedSlices, remainingSlices, remainingSlices-wantedSlices)
			w.WriteHeader(http.StatusCreated)
			return
		}
	}

	if transactionError != nil {
		redisError(w, transactionError)
		return
	}

	// failed
	encodeError(w, "could not perform purchase")
}

// Gluttony returns the gluttony response
func gluttony(w http.ResponseWriter) {
	w.WriteHeader(http.StatusTooManyRequests)
	encoder := json.NewEncoder(w)
	err := errorResponse{"Gluttony is discouraged."}
	encoder.Encode(err)
}

// Gone returns the "gone" response.
// It takes an optional message instead of the default "No more of that pie" message.
func gone(w http.ResponseWriter, msg interface{}) {
	var resp string
	if msg != nil {
		resp, _ = msg.(string)
	} else {
		resp = "No more of that pie. Try something else."
	}

	w.WriteHeader(http.StatusGone)
	encoder := json.NewEncoder(w)
	err := errorResponse{resp}
	encoder.Encode(err)
}

// wrongMaths tells you that you did maths wrong ;)
func wrongMaths(w http.ResponseWriter) {
	encoder := json.NewEncoder(w)
	err := errorResponse{"You did math wrong."}
	w.WriteHeader(http.StatusPaymentRequired)
	encoder.Encode(err)
}

func recommend(w http.ResponseWriter, r *http.Request, p *pie.RecommendPie) {
	encoder := json.NewEncoder(w)
	resp := struct {
		PieURL string `json:"pie_url"`
	}{"http://" + r.Host + "/pies/" + strconv.FormatUint(p.ID, 10)}
	encoder.Encode(resp)
}

// noReommended pies for you sir
func noRecommended(w http.ResponseWriter) {
	encoder := json.NewEncoder(w)
	err := errorResponse{"Sorry we don’t have what you’re looking for.  Come back early tomorrow before the crowds come from the best pie selection."}
	w.WriteHeader(http.StatusNotFound)
	encoder.Encode(err)
}

// ----------------------------------------------------------------------------
// API Response Helpers
// ----------------------------------------------------------------------------

type errorResponse struct {
	Error string `json:"error"`
}

type errorsResponse struct {
	Errors []string `json:"errors"`
}

// encodeJSON is a helper that creates a new json encoder
// and serializes the data and writes it out to the response.
func encodeJSON(w http.ResponseWriter, data interface{}, statusCode interface{}) {
	code := http.StatusOK
	if statusCode != nil {
		code = statusCode.(int)
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}

// encodeError is a helper that takes an array of error messages and
// serializes into an errors JSON response while setting the http status code
// to 500 (internal status errorr)
func encodeError(w http.ResponseWriter, msg string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	errors := errorsResponse{[]string{msg}}
	encoder := json.NewEncoder(w)
	encoder.Encode(errors)
}

// encodeBadRequest is a helper function that takes in an array of error
// messages and returns a BadRequest response with the list of errors
func encodeBadRequest(w http.ResponseWriter, msgs ...string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	errors := errorsResponse{msgs}
	encoder := json.NewEncoder(w)
	encoder.Encode(errors)
}

// redisError is a helper function that reports all redis errors
func redisError(w http.ResponseWriter, e error) {
	errMsg := fmt.Sprintf("error: redis connection error: err=%q\n", e)
	log.Println(errMsg)
	encodeError(w, errMsg)
}
