package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/davinche/gpies/pie"
	"github.com/dimfeld/httptreemux"
	"github.com/garyburd/redigo/redis"
)

// PieKey is the formatted string that represents the key to get a specific pie's
// JSON stringified representation
const PieKey string = "pie:%s"

// HPieKey is the formatted string that represents the key to get a specific pie and it's fields
const HPieKey string = "hpie:%s"

// PieSlicesKey is the formatted string that represents the key to get the number
// of slices for a specific pie
const PieSlicesKey = "pie:%s:slices"

// PiePurchasersKey is the formatted string that represents the number of users
// who has purchased a specific pie
const PiePurchasersKey = "pie:%s:purchasers"

// LabelKey is the formatted string that represents a label. This key points to a
// set of all Pies that are under that label
const LabelKey = "label:%s"

// PurchaseKey is the formatted string that represents the purchases
// by a specific user for a specific Pie.
const PurchaseKey = "pie:%s:user:%s"

// UserAvailableKey is the formatted string that represents the key to the
// number of remaining pies available to the user
const UserAvailableKey = "user:%s:available"

// PiesAvailable is the key representing the set of available pies left to purchase
const PiesAvailable = "piesavailable"

// Redis Connection Pool
var pool *redis.Pool

func init() {
	pool = &redis.Pool{
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", ":6379")
			if err != nil {
				log.Fatalf("error: could not create redis connection pool: err=%q\n", err)
			}
			return c, err
		},
	}
}

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
	w.WriteHeader(code)
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}

// encodeError is a helper that takes an array of error messages and
// serializes into an errors JSON response while setting the http status code
// to 500 (internal status errorr)
func encodeError(w http.ResponseWriter, msg string) {
	w.WriteHeader(http.StatusInternalServerError)
	errors := errorsResponse{[]string{msg}}
	encoder := json.NewEncoder(w)
	encoder.Encode(errors)
}

// encodeBadRequest is a helper function that takes in an array of error
// messages and returns a BadRequest response with the list of errors
func encodeBadRequest(w http.ResponseWriter, msgs ...string) {
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

// Handle takes a prefix (the prefix route for the API) and registers
// functions that will handle the API requests
func Handle(prefix string, r *httptreemux.TreeMux) {
	api := r.NewGroup(prefix)
	api.GET("/pies", getPies)
	api.GET("/pies/:id", getPie)
	api.GET("/pies/recommended", getRecommended)
	api.POST("/pies/:id/purchases", purchasePie)
}

// getPies returns the list of all pies
func getPies(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	// Stub out pies for now
	p := []*pie.Pie{
		&pie.Pie{
			ID:       1,
			Name:     "Apple Pie",
			ImageURL: "Some URL",
			Price:    1.25,
			Slices:   5,
			Labels:   []string{"apple", "pie"},
		},

		&pie.Pie{
			ID:       2,
			Name:     "Pumpkin Pie",
			ImageURL: "Some URL",
			Price:    1.50,
			Slices:   3,
			Labels:   []string{"pumpkin", "pie"},
		},
	}
	encodeJSON(w, p, nil)
}

// getPie returns the information for a single pie
func getPie(w http.ResponseWriter, r *http.Request, params map[string]string) {
	conn := pool.Get()
	defer conn.Close()
	pieID := params["id"]

	// Redis Keys that we need
	key := fmt.Sprintf(PieKey, pieID)
	slicesKey := fmt.Sprintf(PieSlicesKey, pieID)
	piePurchasersKey := fmt.Sprintf(PiePurchasersKey, pieID)

	// Pie to eventually serialize
	p := pie.Pie{}
	details := pie.Details{
		Pie:       &p,
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

	pieString, err := redis.String(resp[0], nil)
	if err != nil {
		redisError(w, err)
		return
	}

	err = json.Unmarshal([]byte(pieString), &p)
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

	// TODO: get purchases
	members, err := redis.Values(resp[2], nil)
	if err != nil {
		fmt.Println("FK")
		redisError(w, err)
		return
	}

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
	p.Slices = 0
	details.RemainingSlices = slices
	encodeJSON(w, details, nil)
}

// getRecommended gets a recommended pie for a given user
func getRecommended(w http.ResponseWriter, r *http.Request, _ map[string]string) {
}

// getPurchaseParams validates an incoming request and ensures that all
// required data is provided
func getPurchaseParams(r *http.Request) (username string, amount float64, slices int, errors []string) {
	// Get purchase information
	username = r.PostFormValue("username")
	amountStr := r.PostFormValue("amount")
	slicesStr := r.PostFormValue("slices")

	// make sure all data is there
	if username == "" || amountStr == "" || slicesStr == "" {
		errorFmt := "error: missing information: %s"
		if username == "" {
			errors = append(errors, fmt.Sprintf(errorFmt, "missing username"))
		}

		if amountStr == "" {
			errors = append(errors, fmt.Sprintf(errorFmt, "missing amount"))
		}

		if slicesStr == "" {
			errors = append(errors, fmt.Sprintf(errorFmt, "missing slices"))
		}
		return "", 0, 0, errors
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
		gone(w, "not enough remaining slices")
		return
	}

	// Check the maths
	pricePerSlice, err := redis.Float64(conn.Do("HGET", hkey, "price"))
	if err != nil {
		redisError(w, err)
		return
	}

	if pricePerSlice*float64(wantedSlices) != amount {
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
	var transactionError error
	for i := 0; i < 5; i++ {
		// TODO: sleep maybe for exponential backoff?
		_, err := conn.Do("WATCH", slicesKey, piePurchasersKey, purchasesKey, UserAvailableKey, PiesAvailable)
		if err != nil {
			transactionError = err
			continue
		}

		// Create or Update the list of pies available to the user
		existingUser, err := redis.Bool(conn.Do("EXISTS", userAvailableKey))
		if err != nil {
			transactionError = err
			continue
		}

		if !existingUser {
			// grab all available pies
			_, err := conn.Do("SDIFFSTORE", userAvailableKey, PiesAvailable)
			if err != nil {
				transactionError = err
				continue
			}
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
			conn.Send("SREM", PiesAvailable, pieID)
		}

		// Check to see if the user can still buy more
		if (purchasedSlices + wantedSlices) == 3 {
			conn.Send("SREM", userAvailableKey, pieID)
		}

		_, err = conn.Do("EXEC")
		if err == nil {
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
