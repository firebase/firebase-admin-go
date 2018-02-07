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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
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
