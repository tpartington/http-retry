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

type retry interface {
	post(req *http.Request, client http.Client) (response *http.Response, err error)
}

type retryingClient struct {
	MaxAttempts int
}

func newHTTPClient() http.Client {
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

	return *client
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

func (rc retryingClient) post(client http.Client, url string, data []byte) (response *http.Response, err error) {
	attempt := 0

	// keep trying until we return a success or we run out of attempts
	for attempt < rc.MaxAttempts {
		req := newHTTPPost(url, data)

		if attempt != 0 {
			backoff(attempt)
		}

		response, err = client.Do(req)

		if err == nil {
			log.Printf("server responded with status: %v\n", response.Status)

			if response.StatusCode >= 200 && response.StatusCode < 299 {
				// no connection error and successful response code, don't retry
				return response, nil
			}

			if response.StatusCode >= 400 && response.StatusCode < 499 {
				// no connection error but a client request error, don't retry
				err = fmt.Errorf("client error %d, not retrying", response.StatusCode)
				return nil, err
			}
		}

		if err != nil {
			log.Printf("error posting request: %v", err)
		}

		attempt++
	}

	err = fmt.Errorf("MaxAttempts %v reached, last attempt failed with: %v", rc.MaxAttempts, err)
	return nil, err

}

func backoff(attempt int) {
	exponential := math.Pow(2, float64(attempt))
	sleep := time.Duration(exponential) * 100000000
	jitter := time.Duration(rand.Int63n(int64(sleep)))
	sleep = sleep + jitter/2
	log.Printf("attempt=%d back-off=%s", attempt, sleep)
	time.Sleep(sleep)
}

func post() error {
	client := newHTTPClient()
	rc := retryingClient{}
	rc.MaxAttempts = 5
	url := "http://localhost:5512/"
	data := []byte(`{"key":"value"}`)

	resp, err := rc.post(client, url, data)
	if err != nil {
		log.Printf("Unable to post message: %v", err)
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
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
