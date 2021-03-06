package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type configType struct {
	port           int
	maxConnection  int
	requestTimeout time.Duration
	jobCount       int
	maxAllowedUrls int
}

type workerResult struct {
	url    string
	result string
	err    error
}

var (
	config        configType
	connCountChan chan struct{}
)

func init() {
	config.port = 8080
	config.maxConnection = 100
	config.requestTimeout = time.Second
	config.jobCount = 4
	config.maxAllowedUrls = 20
}

func main() {
	connCountChan = make(chan struct{}, config.maxConnection)

	server := http.Server{
		Addr:    ":" + strconv.Itoa(config.port),
		Handler: checkConnectionsCount(postHandler),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Got http server error: %v", err)
		}
	}()

	gracefulStop := make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	sig := <-gracefulStop
	log.Printf("Received system call: %+v", sig)
	log.Print("Start shutdown App")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("got error while shotdown: %v", err)
	}

	log.Print("App shutdown")
}

func checkConnectionsCount(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case connCountChan <- struct{}{}:
			next.ServeHTTP(w, r)
			<-connCountChan
		default:
			log.Print("The maximum number of used connections is exhausted.")
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		}
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if len(body) == 0 || r.Header.Get("content-type") != "application/json" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var urls []string
	err = json.Unmarshal(body, &urls)
	if err != nil || len(urls) > config.maxAllowedUrls || len(urls) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	for i := range urls {
		_, err := url.ParseRequestURI(urls[i])
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	}

	urlsCount := len(urls)
	jobCount := config.jobCount
	if urlsCount < config.jobCount {
		jobCount = urlsCount
	}

	ctx := r.Context()
	urlsResult, err := getUrlsResult(ctx, urls, jobCount)
	if err != nil {
		log.Print(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	jsonResult, err := json.Marshal(urlsResult)
	if err != nil {
		log.Print(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(jsonResult); err != nil {
		log.Println(err.Error())
	}

}

func getUrlsResult(ctx context.Context, urls []string, jobCount int) (map[string]string, error) {
	var err error
	urlsCount := len(urls)
	result := make(map[string]string)
	inputChan := make(chan string, urlsCount)
	outputChan := make(chan workerResult, urlsCount)

	workerContext, cancel := context.WithCancel(ctx)
	defer cancel()

	for i := 0; i < jobCount; i++ {
		go worker(workerContext, inputChan, outputChan)
	}
	for _, url := range urls {
		inputChan <- url
	}
	k := 0
	for item := range outputChan {
		if item.err != nil {
			log.Print(err)
			cancel()
			break
		}
		result[item.url] = item.result
		k++
		if k == urlsCount {
			break
		}
	}

	return result, err
}

func worker(ctx context.Context, inputChan <-chan string, outputChan chan<- workerResult) {
	for {
		select {
		case <-ctx.Done():
			return
		case url := <-inputChan:
			var data workerResult
			var resp *http.Response
			var body []byte
			data.url = url

			httpClient := &http.Client{
				Timeout: config.requestTimeout,
			}
			resp, data.err = httpClient.Get(url)
			if data.err == nil {
				if resp.StatusCode != 200 {
					data.err = errors.New("Error, StatusCode is not 200")
				} else {
					body, data.err = ioutil.ReadAll(resp.Body)
				}
			}
			if data.err == nil {
				data.result = string(body)
			}
			outputChan <- data
		}
	}
}
