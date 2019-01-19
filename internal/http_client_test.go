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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"google.golang.org/api/option"
)

var cases = []struct {
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

var testRetryConfig = RetryConfig{
	MaxRetries:       4,
	CheckForRetry:    defaultRetryPolicy,
	ExpBackoffFactor: 0.5,
}

var tokenSourceOpt = option.WithTokenSource(&MockTokenSource{AccessToken: "test"})

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
		want := cases[idx]
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
	for _, tc := range cases {
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
	_, err := client.Do(context.Background(), req)
	if err == nil {
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

func TestNetworkErrorRetryEligible(t *testing.T) {
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

func TestHttpErrorRetryEligible(t *testing.T) {
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

func TestNilCheckForRetry(t *testing.T) {
	rc := &RetryConfig{
		MaxRetries:    1000,
		CheckForRetry: nil,
	}
	if eligible := rc.retryEligible(999, nil, errors.New("network error")); !eligible {
		t.Errorf("retryEligible(999, nil, err) = false; want = true")
	}
	for i := 200; i < 400; i++ {
		resp := &http.Response{
			StatusCode: i,
		}
		if eligible := rc.retryEligible(i, resp, nil); eligible {
			t.Errorf("retryEligible(%d, %d, nil) = true; want = false", i, resp.StatusCode)
		}
	}
	for i := 400; i < 600; i++ {
		resp := &http.Response{
			StatusCode: i,
		}
		if eligible := rc.retryEligible(i, resp, nil); !eligible {
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
	for i := 0; i < 5; i++ {
		delay := testRetryConfig.retryDelay(i, resp)
		if delay != time.Duration(30)*time.Second {
			t.Errorf("retryDelay = %f s; want = 30.0 s", delay.Seconds())
		}
	}
}

func TestRetryAfterHeaderInTimestampFormat(t *testing.T) {
	header := make(http.Header)
	now := time.Now()
	retryAfter := now.Add(time.Duration(60) * time.Second)
	header.Add("retry-after", retryAfter.UTC().Format(http.TimeFormat))
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     header,
	}
	clock = &MockClock{now}
	for i := 0; i < 5; i++ {
		delay := testRetryConfig.retryDelay(i, resp)
		// HTTP timestamp format only has seconds precision. So the final value could be off by 1s.
		if delay < time.Duration(60-1)*time.Second || delay > time.Duration(60+1)*time.Second {
			t.Errorf("retryDelay = %f s; want = ~60.0 s", delay.Seconds())
		}
	}
}

func TestRetryDelayExponentialBackoff(t *testing.T) {
	want := []int{0, 1, 2, 4, 8}
	for i := 0; i < 5; i++ {
		delay := testRetryConfig.retryDelay(i, nil)
		if delay != time.Duration(want[i])*time.Second {
			t.Errorf("retryDelay = %f s; want = %d.0 s", delay.Seconds(), want[i])
		}
	}
}

func TestRetryDelayNoExponentialBackoff(t *testing.T) {
	retryConfigWithoutBackoff := &RetryConfig{
		MaxRetries: 4,
	}
	for i := 0; i < 5; i++ {
		delay := retryConfigWithoutBackoff.retryDelay(i, nil)
		if delay != 0 {
			t.Errorf("retryDelay = %f s; want = 0.0 s", delay.Seconds())
		}
	}
}

func TestLongestRetryDelayHasPrecedence(t *testing.T) {
	header := make(http.Header)
	now := time.Now()
	retryAfter := now.Add(time.Duration(3) * time.Second)
	header.Add("retry-after", retryAfter.UTC().Format(http.TimeFormat))
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     header,
	}
	clock = &MockClock{now}
	want := []int{0, 1, 2, 4, 8}
	for i := 0; i < 5; i++ {
		delay := testRetryConfig.retryDelay(i, resp)
		if want[i] > 3 {
			if delay != time.Duration(want[i])*time.Second {
				t.Errorf("retryDelay = %f s; want = %d.0 s", delay.Seconds(), want[i])
			}
		} else {
			if delay < time.Duration(3-1)*time.Second || delay > time.Duration(3+1)*time.Second {
				t.Errorf("retryDelay = %f s; want = ~3.0 s", delay.Seconds())
			}
		}
	}
}

func TestNoRetryOnRequestBuildError(t *testing.T) {
	client := &HTTPClient{
		Client:      http.DefaultClient,
		RetryConfig: &testRetryConfig,
	}

	entity := &errorEntry{}
	req := &Request{
		Method: http.MethodGet,
		URL:    "https://firebase.google.com",
		Body:   entity,
	}
	_, err := client.Do(context.Background(), req)
	if err == nil {
		t.Errorf("Do() = nil; want = error")
	}
	if entity.Count != 1 {
		t.Errorf("Total requests = %d; want = %d", entity.Count, 1)
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
	const defaultMaxRetries = 4
	for _, status = range []int{http.StatusInternalServerError, http.StatusServiceUnavailable} {
		requests = 0
		req := &Request{Method: http.MethodGet, URL: server.URL}
		resp, err := client.Do(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.CheckStatus(status); err != nil {
			t.Errorf("CheckStatus() = %q; want = nil", err.Error())
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
		t.Errorf("Total requests = %d; want = %d", requests, 1)
	}
}

type errorEntry struct {
	Count int
}

func (e *errorEntry) Bytes() ([]byte, error) {
	e.Count++
	return nil, errors.New("test error")
}

func (e *errorEntry) Mime() string {
	return "application/json"
}
