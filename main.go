package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/memcache"
)

var apiKey string
var ttl time.Duration

func init() {
	var err error
	ttl, err = time.ParseDuration("24h")
	if err != nil {
		log.Fatal("Bad durations")
	}

	apiKey = os.Getenv("OPENWEATHERMAP_API_KEY")
	if apiKey == "" {
		log.Fatal("environment variable OPENWEATHERMAP_API_KEY is not set")
	}
}

func api(ctx context.Context, latScaled int, lonScaled int) (string, error) {
	cacheKey := fmt.Sprintf("%d:%d", latScaled, lonScaled)
	fmt.Printf("cacheKey=%s\n", cacheKey)

	item, err := memcache.Get(ctx, cacheKey)
	if err != nil && err != memcache.ErrCacheMiss {
		return "", err
	}
	if err == nil {
		fmt.Print("HIT\n")
		return string(item.Value), nil
	}
	fmt.Print("MISS\n")

	requestURL := fmt.Sprintf(
		`https://api.openweathermap.org/data/2.5/forecast?lat=%f&lon=%f&units=metric&appid=%s`,
		unscale(latScaled), unscale(lonScaled), apiKey)
	fmt.Printf("%s\n", requestURL)
	res, err := http.Get(requestURL)
	if err != nil {
		return fmt.Sprintf("error making http request: '%s'", requestURL), err
	}

	fmt.Printf("client: got response!\n")
	fmt.Printf("client: status code: %d\n", res.StatusCode)

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "error reading response body", err
	}
	json := string(resBody)

	item1 := &memcache.Item{
		Key:        cacheKey,
		Value:      []byte(json),
		Expiration: ttl,
	}
	if err := memcache.Set(ctx, item1); err != nil {
		return "", err
	}
	return json, nil
}

func scale(x float64) int {
	return int(math.Round(x * 10000))
}
func unscale(i int) float64 {
	return float64(i) / 10000
}

func Api(w http.ResponseWriter, req *http.Request) {
	ctx := appengine.NewContext(req)

	query := req.URL.Query()
	lat, err := strconv.ParseFloat(query.Get("lat"), 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	lon, err := strconv.ParseFloat(query.Get("lon"), 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", req.Header.Get("Origin"))
	result, err := api(ctx, scale(lat), scale(lon))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write([]byte(result))
}

func main() {
	http.HandleFunc("/", Api)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}
	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
