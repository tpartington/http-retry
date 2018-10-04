package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"testing"
	"time"
)

type response struct {
	StatusCode int
	Status     string
}

type fakeClient struct {
	attempts  int
	responses []response
}

func newFakeClientSucceed() *fakeClient {
	responses := []response{response{200, "success"}}
	return &fakeClient{0, responses}
}

func newFakeClientFail() *fakeClient {
	responses := []response{response{500, "failure"}}
	return &fakeClient{0, responses}
}

func newFakeClientFailFailSuccess() *fakeClient {
	responses := []response{response{500, "failure 0"}, response{500, "failure 1"}, response{200, "success 2"}}
	return &fakeClient{0, responses}
}

func newFakeClientFailFailFail() *fakeClient {
	responses := []response{response{500, "failure 0"}, response{500, "failure 1"}, response{500, "failure 2"}}
	return &fakeClient{0, responses}
}

func newFakeClientFail400() *fakeClient {
	responses := []response{response{400, "failure 0"}}
	return &fakeClient{0, responses}
}

func (fc *fakeClient) Do(req *http.Request) (*http.Response, error) {
	if verbose {
		fmt.Printf("attempt=%d, method=%s, url=%s, body=%s\n", fc.attempts, req.Method, req.URL, req.Body)
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	var i = (fc.attempts % len(fc.responses))

	r := &http.Response{
		Status:        fc.responses[i].Status,
		StatusCode:    fc.responses[i].StatusCode,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
		Header:        make(http.Header, 0),
	}

	fc.attempts++
	return r, nil
}

func TestPostSucceed(t *testing.T) {
	rc := retryingClient{}
	rc.client = newFakeClientSucceed()
	rc.maxAttempts = 3
	url := "http://localhost:8000/random"
	data := []byte(`{"key":"value"}`)

	resp, err := rc.post(url, data)
	if err != nil {
		log.Printf("post returned error: %v", err)
		t.Fail()
	}

	if resp.StatusCode != 200 {
		fmt.Printf("expected status code 200, received: %d", resp.StatusCode)
		t.Fail()
	}
}

func TestPostFail(t *testing.T) {
	rc := retryingClient{}
	rc.client = newFakeClientFail()
	rc.maxAttempts = 3
	url := "http://localhost:8000/random"
	data := []byte(`{"key":"value"}`)

	resp, err := rc.post(url, data)

	if resp.StatusCode != 500 && err != nil {
		fmt.Printf("expected status code 500, received: %d", resp.StatusCode)
		t.Fail()
	}
}

func TestPostClient400(t *testing.T) {
	rc := retryingClient{}
	rc.client = newFakeClientFail400()
	rc.maxAttempts = 3
	url := "http://localhost:8000/random"
	data := []byte(`{"key":"value"}`)

	resp, err := rc.post(url, data)

	if resp.StatusCode != 400 && err != nil {
		fmt.Printf("expected status code 400, received: %d", resp.StatusCode)
		t.Fail()
	}
}

func TestFakeClientBackoff(t *testing.T) {
	rc := retryingClient{}
	rc.client = newFakeClientFail()
	rc.maxAttempts = 5
	url := "http://localhost:8000/random"
	data := []byte(`{"key":"value"}`)
	start := time.Now()

	_, err := rc.post(url, data)
	if err != nil {
		log.Printf("post returned error: %v", err)
	}
	elapsed := time.Since(start)

	// in an exponential function only the last two values really matter so calulate those,
	// and then check if the time the function took is less than that
	nearMaxSleep := math.Pow(float64(rc.maxAttempts-1), 4)
	maxSleep := math.Pow(float64(rc.maxAttempts), 4)
	approxSleep := (time.Duration(maxSleep) + time.Duration(nearMaxSleep)) * time.Millisecond

	if verbose {
		fmt.Printf("%s (elapsed) vs %s (approxSleep)\n", elapsed, approxSleep)
	}

	if elapsed.Round(time.Millisecond) < approxSleep.Round(time.Millisecond) {
		fmt.Printf("%s (elapsed) < %s (approxSleep), backoff not functioning\n", elapsed, approxSleep)
		t.Fail()
	}
}
