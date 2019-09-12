// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/option"
)

const defaultMaxRetries = 4

var (
	testRetryConfig = RetryConfig{
		MaxRetries:       4,
		ExpBackoffFactor: 0.5,
	}
	tokenSourceOpt = option.WithTokenSource(&MockTokenSource{AccessToken: "test"})
)

var testRequests = []struct {
	req     *Request
	method  string
	body    string
	headers map[string]string
	query   map[string]string
}{
	{
		req: &Request{
			Method: http.MethodGet,
		},
		method: http.MethodGet,
	},
	{
		req: &Request{
			Method: http.MethodGet,
			Opts: []HTTPOption{
				WithHeader("Test-Header", "value1"),
				WithQueryParam("testParam", "value2"),
			},
		},
		method:  http.MethodGet,
		headers: map[string]string{"Test-Header": "value1"},
		query:   map[string]string{"testParam": "value2"},
	},
	{
		req: &Request{
			Method: http.MethodPost,
			Body:   NewJSONEntity(map[string]string{"foo": "bar"}),
			Opts: []HTTPOption{
				WithHeader("Test-Header", "value1"),
				WithQueryParam("testParam1", "value2"),
				WithQueryParam("testParam2", "value3"),
			},
		},
		method:  http.MethodPost,
		body:    "{\"foo\":\"bar\"}",
		headers: map[string]string{"Test-Header": "value1"},
		query:   map[string]string{"testParam1": "value2", "testParam2": "value3"},
	},
	{
		req: &Request{
			Method: http.MethodPost,
			Body:   NewJSONEntity("body"),
			Opts: []HTTPOption{
				WithHeader("Test-Header", "value1"),
				WithQueryParams(map[string]string{"testParam1": "value2", "testParam2": "value3"}),
			},
		},
		method:  http.MethodPost,
		body:    "\"body\"",
		headers: map[string]string{"Test-Header": "value1"},
		query:   map[string]string{"testParam1": "value2", "testParam2": "value3"},
	},
	{
		req: &Request{
			Method: http.MethodPut,
			Body:   NewJSONEntity(nil),
			Opts: []HTTPOption{
				WithHeader("Test-Header", "value1"),
				WithQueryParams(map[string]string{"testParam1": "value2", "testParam2": "value3"}),
			},
		},
		method:  http.MethodPut,
		body:    "null",
		headers: map[string]string{"Test-Header": "value1"},
		query:   map[string]string{"testParam1": "value2", "testParam2": "value3"},
	},
}

func TestHTTPClient(t *testing.T) {
	want := map[string]interface{}{
		"key1": "value1",
		"key2": float64(100),
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	idx := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := testRequests[idx]
		if r.Method != want.method {
			t.Errorf("[%d] Method = %q; want = %q", idx, r.Method, want.method)
		}
		for k, v := range want.headers {
			h := r.Header.Get(k)
			if h != v {
				t.Errorf("[%d] Header(%q) = %q; want = %q", idx, k, h, v)
			}
		}
		if want.query == nil {
			if r.URL.Query().Encode() != "" {
				t.Errorf("[%d] Query = %v; want = empty", idx, r.URL.Query().Encode())
			}
		}
		for k, v := range want.query {
			q := r.URL.Query().Get(k)
			if q != v {
				t.Errorf("[%d] Query(%q) = %q; want = %q", idx, k, q, v)
			}
		}
		if want.body != "" {
			h := r.Header.Get("Content-Type")
			if h != "application/json" {
				t.Errorf("[%d] Content-Type = %q; want = %q", idx, h, "application/json")
			}
			wb := []byte(want.body)
			gb, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(wb, gb) {
				t.Errorf("[%d] Body = %q; want = %q", idx, string(gb), string(wb))
			}
		}

		idx++
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{Client: http.DefaultClient}
	for _, tc := range testRequests {
		tc.req.URL = server.URL
		resp, err := client.Do(context.Background(), tc.req)
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.CheckStatus(http.StatusOK); err != nil {
			t.Errorf("CheckStatus() = %v; want nil", err)
		}
		if err := resp.CheckStatus(http.StatusCreated); err == nil {
			t.Errorf("CheckStatus() = nil; want error")
		}

		var got map[string]interface{}
		if err := resp.Unmarshal(http.StatusOK, &got); err != nil {
			t.Errorf("Unmarshal() = %v; want nil", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Body = %v; want = %v", got, want)
		}
	}
}

func TestDefaultOpts(t *testing.T) {
	var header string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("Test-Header")
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client: http.DefaultClient,
		Opts: []HTTPOption{
			WithHeader("Test-Header", "test-value"),
		},
	}
	req := &Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s%s", server.URL, wantURL),
	}

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Status != http.StatusOK {
		t.Errorf("Status = %d; want = %d", resp.Status, http.StatusOK)
	}
	if header != "test-value" {
		t.Errorf("Test-Header = %q; want = %q", header, "test-value")
	}
}

func TestSuccessFn(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client: http.DefaultClient,
		SuccessFn: func(r *Response) bool {
			return false
		},
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}
	want := "unexpected http response with status: 200; body: {}"

	resp, err := client.Do(context.Background(), get)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
	}

	if !HasErrorCode(err, "UNKNOWN") {
		t.Errorf("ErrorCode = %q; want = %q", err.(*FirebaseError).Code, "UNKNOWN")
	}
}

func TestSuccessFnOnRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client:    http.DefaultClient,
		SuccessFn: HasSuccessStatus,
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
		SuccessFn: func(r *Response) bool {
			return false
		},
	}
	want := "unexpected http response with status: 200; body: {}"

	resp, err := client.Do(context.Background(), get)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
	}

	if !HasErrorCode(err, "UNKNOWN") {
		t.Errorf("ErrorCode = %q; want = %q", err.(*FirebaseError).Code, "UNKNOWN")
	}
}

func TestPlatformError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"error": {
				"status": "NOT_FOUND",
				"message": "Requested entity not found"
			}
		}`

		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(resp))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client:    http.DefaultClient,
		SuccessFn: HasSuccessStatus,
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}
	want := "Requested entity not found"

	resp, err := client.Do(context.Background(), get)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
	}

	if !HasErrorCode(err, "NOT_FOUND") {
		t.Errorf("ErrorCode = %q; want = %q", err.(*FirebaseError).Code, "NOT_FOUND")
	}
}

func TestPlatformErrorWithoutDetails(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client:    http.DefaultClient,
		SuccessFn: HasSuccessStatus,
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}
	want := "unexpected http response with status: 404; body: {}"

	resp, err := client.Do(context.Background(), get)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
	}

	if !HasErrorCode(err, "UNKNOWN") {
		t.Errorf("ErrorCode = %q; want = %q", err.(*FirebaseError).Code, "UNKNOWN")
	}
}

func TestCreateErrFn(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client: http.DefaultClient,
		CreateErrFn: func(r *Response) error {
			return fmt.Errorf("custom error with status: %d", r.Status)
		},
		SuccessFn: HasSuccessStatus,
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}
	want := "custom error with status: 404"

	resp, err := client.Do(context.Background(), get)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
	}
}

func TestCreateErrFnOnRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client: http.DefaultClient,
		CreateErrFn: func(r *Response) error {
			return fmt.Errorf("custom error with status: %d", r.Status)
		},
		SuccessFn: HasSuccessStatus,
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
		CreateErrFn: func(r *Response) error {
			return fmt.Errorf("custom error from req with status: %d", r.Status)
		},
	}
	want := "custom error from req with status: 404"

	resp, err := client.Do(context.Background(), get)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
	}
}

func TestContext(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{Client: http.DefaultClient}
	ctx, cancel := context.WithCancel(context.Background())
	resp, err := client.Do(ctx, &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.CheckStatus(http.StatusOK); err != nil {
		t.Fatal(err)
	}

	cancel()
	resp, err = client.Do(ctx, &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	})
	if resp != nil || err == nil {
		t.Errorf("Do() = (%v; %v); want = (nil, error)", resp, err)
	}
}

func TestErrorParser(t *testing.T) {
	data := map[string]interface{}{
		"error": "test error",
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	ep := func(b []byte) string {
		var p struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return ""
		}
		return p.Error
	}
	client := &HTTPClient{
		Client:    http.DefaultClient,
		ErrParser: ep,
	}
	req := &Request{Method: http.MethodGet, URL: server.URL}
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	want := "http error status: 500; reason: test error"
	if err := resp.CheckStatus(http.StatusOK); err.Error() != want {
		t.Errorf("CheckStatus() = %q; want = %q", err.Error(), want)
	}
	var got map[string]interface{}
	if err := resp.Unmarshal(http.StatusOK, &got); err.Error() != want {
		t.Errorf("CheckStatus() = %q; want = %q", err.Error(), want)
	}
	if got != nil {
		t.Errorf("Body = %v; want = nil", got)
	}
}

func TestInvalidURL(t *testing.T) {
	req := &Request{
		Method: http.MethodGet,
		URL:    "http://localhost:250/mock.url",
	}
	client := &HTTPClient{Client: http.DefaultClient}
	if _, err := client.Do(context.Background(), req); err == nil {
		t.Errorf("Send() = nil; want error")
	}
}

func TestUnmarshalError(t *testing.T) {
	data := map[string]interface{}{
		"foo": "bar",
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	req := &Request{Method: http.MethodGet, URL: server.URL}
	client := &HTTPClient{Client: http.DefaultClient}
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	var got func()
	if err := resp.Unmarshal(http.StatusOK, &got); err == nil {
		t.Errorf("Unmarshal() = nil; want error")
	}
}

func TestRetryDisabled(t *testing.T) {
	requests := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client:      http.DefaultClient,
		RetryConfig: nil,
	}
	req := &Request{Method: http.MethodGet, URL: server.URL}
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.CheckStatus(http.StatusServiceUnavailable); err != nil {
		t.Errorf("CheckStatus() = %q; want = nil", err.Error())
	}
	if requests != 1 {
		t.Errorf("Total requests = %d; want = 1", requests)
	}
}

func TestNetworkErrorMaxRetries(t *testing.T) {
	err := errors.New("network error")
	maxRetries := testRetryConfig.MaxRetries
	for i := 0; i < maxRetries; i++ {
		if eligible := testRetryConfig.retryEligible(i, nil, err); !eligible {
			t.Errorf("retryEligible(%d, nil, err) = false; want = true", i)
		}
	}
	if eligible := testRetryConfig.retryEligible(maxRetries, nil, err); eligible {
		t.Errorf("retryEligible(%d, nil, err) = true; want = false", maxRetries)
	}
}

func TestHTTPErrorMaxRetries(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
	}
	maxRetries := testRetryConfig.MaxRetries
	for i := 0; i < maxRetries; i++ {
		if eligible := testRetryConfig.retryEligible(i, resp, nil); !eligible {
			t.Errorf("retryEligible(%d, 503, nil) = false; want = true", i)
		}
	}
	if eligible := testRetryConfig.retryEligible(maxRetries, resp, nil); eligible {
		t.Errorf("retryEligible(%d, 503, nil) = true; want = false", maxRetries)
	}
}

func TestNoRetryOnRequestBuildError(t *testing.T) {
	client := &HTTPClient{
		Client:      http.DefaultClient,
		RetryConfig: &testRetryConfig,
	}

	entity := &faultyEntity{}
	req := &Request{
		Method: http.MethodGet,
		URL:    "https://firebase.google.com",
		Body:   entity,
	}
	if _, err := client.Do(context.Background(), req); err == nil {
		t.Errorf("Do(<faultyEntity>) = nil; want = error")
	}
	if entity.RequestAttempts != 1 {
		t.Errorf("Request attempts = %d; want = 1", entity.RequestAttempts)
	}
}

func TestNoRetryOnInvalidMethod(t *testing.T) {
	client := &HTTPClient{
		Client:      http.DefaultClient,
		RetryConfig: &testRetryConfig,
	}

	req := &Request{
		Method: "Invalid/Method",
		URL:    "https://firebase.google.com",
	}
	if _, err := client.Do(context.Background(), req); err == nil {
		t.Errorf("Do(<faultyEntity>) = nil; want = error")
	}
}

func TestNoRetryOnHTTPSuccessCodes(t *testing.T) {
	for i := http.StatusOK; i < http.StatusBadRequest; i++ {
		resp := &http.Response{
			StatusCode: i,
		}
		if eligible := testRetryConfig.retryEligible(0, resp, nil); eligible {
			t.Errorf("retryEligible(%d, %d, nil) = true; want = false", i, resp.StatusCode)
		}
	}
}

func TestRetryOnHTTPErrorCodes(t *testing.T) {
	for i := http.StatusInternalServerError; i <= http.StatusNetworkAuthenticationRequired; i++ {
		resp := &http.Response{
			StatusCode: i,
		}
		if eligible := testRetryConfig.retryEligible(0, resp, nil); !eligible {
			t.Errorf("retryEligible(%d, %d, nil) = false; want = true", i, resp.StatusCode)
		}
	}
}

func TestRetryAfterHeaderInSecondsFormat(t *testing.T) {
	header := make(http.Header)
	header.Add("retry-after", "30")
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     header,
	}
	maxRetries := testRetryConfig.MaxRetries
	for i := 0; i < maxRetries; i++ {
		delay, ok := testRetryConfig.retryDelay(i, resp, nil)
		if !ok || delay != time.Duration(30)*time.Second {
			t.Errorf("retryDelay(%d) = (%f, %v); want = (30.0, true)", i, delay.Seconds(), ok)
		}
	}
	delay, ok := testRetryConfig.retryDelay(maxRetries, resp, nil)
	if ok || delay != 0 {
		t.Errorf("retryDelay(%d) = (%f, %v); want = (0.0, false)", maxRetries, delay.Seconds(), ok)
	}
}

func TestRetryAfterHeaderInTimestampFormat(t *testing.T) {
	header := make(http.Header)
	now := time.Now()
	retryAfter := now.Add(time.Duration(60) * time.Second)
	// http.TimeFormat requires the time be in UTC.
	header.Add("retry-after", retryAfter.UTC().Format(http.TimeFormat))
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     header,
	}
	retryTimeClock = &MockClock{now}
	maxRetries := testRetryConfig.MaxRetries
	for i := 0; i < maxRetries; i++ {
		delay, ok := testRetryConfig.retryDelay(i, resp, nil)
		// HTTP timestamp format has seconds precision. So the final value could be off by 1s.
		if !ok || delay < time.Duration(60-1)*time.Second || delay > time.Duration(60+1)*time.Second {
			t.Errorf("retryDelay(%d) = (%f, %v); want = (~60.0, true)", i, delay.Seconds(), ok)
		}
	}
	delay, ok := testRetryConfig.retryDelay(maxRetries, resp, nil)
	if ok || delay != 0 {
		t.Errorf("retryDelay(%d) = (%f, %v); want = (0.0, false)", maxRetries, delay.Seconds(), ok)
	}
}

func TestMaxDelayWithRetryAfterHeader(t *testing.T) {
	header := make(http.Header)
	header.Add("retry-after", "30")
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     header,
	}
	tenSeconds := time.Duration(10) * time.Second
	rc := &RetryConfig{
		MaxRetries: 4,
		MaxDelay:   &tenSeconds,
	}
	delay, ok := rc.retryDelay(0, resp, nil)
	if ok || delay != 0 {
		t.Errorf("retryDelay() = (%f, %v); want = (0.0, false)", delay.Seconds(), ok)
	}
}

func TestRetryDelayExpBackoff(t *testing.T) {
	want := []int{0, 1, 2, 4}
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
	}
	maxRetries := testRetryConfig.MaxRetries
	for i := 0; i < maxRetries; i++ {
		delay, ok := testRetryConfig.retryDelay(i, resp, nil)
		if !ok || delay != time.Duration(want[i])*time.Second {
			t.Errorf("retryDelay(%d) = (%f, %v); want = (%d, true)", i, delay.Seconds(), ok, want[i])
		}
	}
	delay, ok := testRetryConfig.retryDelay(maxRetries, resp, nil)
	if ok || delay != 0 {
		t.Errorf("retryDelay(%d) = (%f, %v); want = (0, false)", maxRetries, delay.Seconds(), ok)
	}
}

func TestMaxDelayWithExpBackoff(t *testing.T) {
	want := []int{0, 2, 4, 5, 5}
	fiveSeconds := time.Duration(5) * time.Second
	rc := &RetryConfig{
		MaxRetries:       5,
		MaxDelay:         &fiveSeconds,
		ExpBackoffFactor: 1,
	}
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
	}
	for i := 0; i < 5; i++ {
		delay, ok := rc.retryDelay(i, resp, nil)
		if !ok || delay != time.Duration(want[i])*time.Second {
			t.Errorf("retryDelay(%d) = (%f, %v); want = (%d, true)", i, delay.Seconds(), ok, want[i])
		}
	}
}

func TestRetryDelayDisableExponentialBackoff(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
	}
	rc := &RetryConfig{
		MaxRetries:       4,
		ExpBackoffFactor: 0,
	}
	for i := 0; i < 4; i++ {
		delay, ok := rc.retryDelay(i, resp, nil)
		if !ok || delay != 0 {
			t.Errorf("retryDelay(%d) = (%f, %v); want = (0, true)", i, delay.Seconds(), ok)
		}
	}
}

func TestLongestRetryDelayHasPrecedence(t *testing.T) {
	header := make(http.Header)
	header.Add("retry-after", "3")
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     header,
	}
	want := []int{0, 1, 2, 4}
	for i := 0; i < 4; i++ {
		delay, ok := testRetryConfig.retryDelay(i, resp, nil)
		if !ok {
			t.Errorf("retryDelay(%d) = false; want = true", i)
		}
		if want[i] <= 3 {
			if delay < time.Duration(3-1)*time.Second || delay > time.Duration(3+1)*time.Second {
				t.Errorf("retryDelay(%d) = %f; want = ~3.0", i, delay.Seconds())
			}
		} else {
			if delay != time.Duration(want[i])*time.Second {
				t.Errorf("retryDelay(%d) = %f; want = %d", i, delay.Seconds(), want[i])
			}
		}
	}
}

func TestContextCancellationStopsRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	result := &attemptResult{}
	if err := result.waitForRetry(ctx); err != nil {
		t.Fatalf("prepareRequest() = %v; want = nil", err)
	}
	cancel()
	if err := result.waitForRetry(ctx); err != context.Canceled {
		t.Errorf("prepareRequest() = %v; want = %v", err, context.Canceled)
	}
}

func TestNewHTTPClient(t *testing.T) {
	wantEndpoint := "https://cloud.google.com"
	opts := []option.ClientOption{
		tokenSourceOpt,
		option.WithEndpoint(wantEndpoint),
	}
	client, endpoint, err := NewHTTPClient(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	wantRetry := &RetryConfig{
		MaxRetries:       4,
		ExpBackoffFactor: 0.5,
	}
	gotRetry := client.RetryConfig
	if gotRetry.MaxRetries != wantRetry.MaxRetries ||
		gotRetry.ExpBackoffFactor != wantRetry.ExpBackoffFactor ||
		gotRetry.CheckForRetry == nil {
		t.Errorf("NewHTTPClient().RetryConfig = %v; want = %v", *gotRetry, wantRetry)
	}
	if endpoint != wantEndpoint {
		t.Errorf("NewHTTPClient() = %q; want = %q", endpoint, wantEndpoint)
	}
}

func TestNewHTTPClientRetryOnNetworkErrors(t *testing.T) {
	client, _, err := NewHTTPClient(context.Background(), tokenSourceOpt)
	if err != nil {
		t.Fatal(err)
	}
	tansport := &faultyTransport{}
	client.Client.Transport = tansport
	client.RetryConfig.ExpBackoffFactor = 0

	req := &Request{Method: http.MethodGet, URL: "http://firebase.google.com"}
	resp, err := client.Do(context.Background(), req)
	if resp != nil || err == nil {
		t.Errorf("Do() = (%v, %v); want = (nil, error)", resp, err)
	}

	wantRequests := 1 + defaultMaxRetries
	if tansport.RequestAttempts != wantRequests {
		t.Errorf("Total requests = %d; want = %d", tansport.RequestAttempts, wantRequests)
	}
}

func TestNewHTTPClientRetryOnHTTPErrors(t *testing.T) {
	var status int
	requests := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(status)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client, _, err := NewHTTPClient(context.Background(), tokenSourceOpt)
	if err != nil {
		t.Fatal(err)
	}
	client.RetryConfig.ExpBackoffFactor = 0
	for _, status = range []int{http.StatusInternalServerError, http.StatusServiceUnavailable} {
		requests = 0
		req := &Request{Method: http.MethodGet, URL: server.URL}
		resp, err := client.Do(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.CheckStatus(status); err != nil {
			t.Errorf("CheckStatus(%d) = %q; want = nil", status, err.Error())
		}
		wantRequests := 1 + defaultMaxRetries
		if requests != wantRequests {
			t.Errorf("Total requests = %d; want = %d", requests, wantRequests)
		}
	}
}

func TestNewHttpClientNoRetryOnNotFound(t *testing.T) {
	requests := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client, _, err := NewHTTPClient(context.Background(), tokenSourceOpt)
	if err != nil {
		t.Fatal(err)
	}
	req := &Request{Method: http.MethodGet, URL: server.URL}
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.CheckStatus(http.StatusNotFound); err != nil {
		t.Errorf("CheckStatus() = %q; want = nil", err.Error())
	}
	if requests != 1 {
		t.Errorf("Total requests = %d; want = 1", requests)
	}
}

func TestNewHttpClientRetryOnResponseReadError(t *testing.T) {
	requests := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		// Lie about the content-length forcing a read error on the client
		w.Header().Set("Content-Length", "1")
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client, _, err := NewHTTPClient(context.Background(), tokenSourceOpt)
	if err != nil {
		t.Fatal(err)
	}
	client.RetryConfig.ExpBackoffFactor = 0
	wantPrefix := "error while making http call: "

	req := &Request{Method: http.MethodGet, URL: server.URL}
	resp, err := client.Do(context.Background(), req)
	if resp != nil || err == nil || !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("Do() = (%v, %v); want = (nil, %q)", resp, err, wantPrefix)
	}

	wantRequests := 1 + defaultMaxRetries
	if requests != wantRequests {
		t.Errorf("Total requests = %d; want = %d", requests, wantRequests)
	}
}

type faultyEntity struct {
	RequestAttempts int
}

func (e *faultyEntity) Bytes() ([]byte, error) {
	e.RequestAttempts++
	return nil, errors.New("test error")
}

func (e *faultyEntity) Mime() string {
	return "application/json"
}

type faultyTransport struct {
	RequestAttempts int
}

func (e *faultyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	e.RequestAttempts++
	return nil, errors.New("test error")
}
