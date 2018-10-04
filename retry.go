package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"time"
)

var verbose bool

type retry interface {
	post(req *http.Request, client http.Client) (response *http.Response, err error)
}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type retryingClient struct {
	maxAttempts int
	client      httpClient
}

func newHTTPClient() *http.Client {
	netTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 1 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Timeout:   time.Second * 1,
		Transport: netTransport,
	}

	return client
}

func newHTTPPost(url string, data []byte) *http.Request {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if nil != err {
		log.Printf("could not create a new post reqest: %v", err)
	}

	if err != nil {
		panic(err)
	}

	return req
}

func (rc retryingClient) post(url string, data []byte) (response *http.Response, err error) {
	attempt := 1

	// keep trying until we return a success or we run out of attempts
	for attempt <= rc.maxAttempts {
		req := newHTTPPost(url, data)

		if attempt != 1 {
			backoff(attempt)
		}

		response, err = rc.client.Do(req)

		if err == nil {
			if verbose {
				log.Printf("server responded with status: %v\n", response.Status)
			}

			// no connection error and successful response code, don't retry
			if response.StatusCode >= 200 && response.StatusCode < 299 {
				return response, nil
			}

			// no connection error but a client request error, don't retry
			if response.StatusCode >= 400 && response.StatusCode < 499 {
				err = fmt.Errorf("client error %d, not retrying", response.StatusCode)
				return response, err
			}
		}

		if err != nil {
			log.Printf("error posting request: %v", err)
		}

		attempt++
	}

	err = fmt.Errorf("maxAttempts %v reached, last attempt failed with: %v", rc.maxAttempts, err)
	return response, err

}

func backoff(attempt int) {
	// x^4 gives us the nicest backoff rate
	exponential := math.Pow(float64(attempt), 4)
	sleep := time.Duration(exponential) * time.Millisecond
	jitter := time.Duration(rand.Int63n(int64(sleep)))
	sleep = sleep + jitter/2
	if verbose {
		log.Printf("attempt=%d back-off=%s", attempt, sleep)
	}
	time.Sleep(sleep)
}

func post() error {
	rc := &retryingClient{}
	rc.client = newHTTPClient()
	rc.maxAttempts = 3
	url := "http://localhost:8000/random"
	data := []byte(`{"key":"value"}`)

	resp, err := rc.post(url, data)
	if err != nil {
		log.Printf("Unable to post message: %v", err)
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Printf("post: %d %s\n", resp.StatusCode, (string(body)))
	defer resp.Body.Close()

	return nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	post()
}
