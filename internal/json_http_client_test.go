// Copyright 2019 Google Inc. All Rights Reserved.
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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const wantURL = "/test"

func TestDoAndUnmarshalGet(t *testing.T) {
	var req *http.Request
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		resp := `{
			"name": "test"
		}`
		w.Write([]byte(resp))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client: http.DefaultClient,
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s%s", server.URL, wantURL),
	}
	var data responseBody

	resp, err := client.DoAndUnmarshal(context.Background(), get, &data)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Status != http.StatusOK {
		t.Errorf("Status = %d; want = %d", resp.Status, http.StatusOK)
	}
	if data.Name != "test" {
		t.Errorf("Data = %v; want = {Name: %q}", data, "test")
	}
	if req.Method != http.MethodGet {
		t.Errorf("Method = %q; want = %q", req.Method, http.MethodGet)
	}
	if req.URL.Path != wantURL {
		t.Errorf("URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestDoAndUnmarshalPost(t *testing.T) {
	var req *http.Request
	var b []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		b, _ = ioutil.ReadAll(r.Body)
		resp := `{
			"name": "test"
		}`
		w.Write([]byte(resp))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client: http.DefaultClient,
	}
	post := &Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s%s", server.URL, wantURL),
		Body:   NewJSONEntity(map[string]string{"input": "test-input"}),
	}
	var data responseBody

	resp, err := client.DoAndUnmarshal(context.Background(), post, &data)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Status != http.StatusOK {
		t.Errorf("Status = %d; want = %d", resp.Status, http.StatusOK)
	}
	if data.Name != "test" {
		t.Errorf("Data = %v; want = {Name: %q}", data, "test")
	}
	if req.Method != http.MethodPost {
		t.Errorf("Method = %q; want = %q", req.Method, http.MethodGet)
	}
	if req.URL.Path != wantURL {
		t.Errorf("URL = %q; want = %q", req.URL.Path, wantURL)
	}

	var parsed struct {
		Input string `json:"input"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Input != "test-input" {
		t.Errorf("Request Body = %v; want = {Input: %q}", parsed, "test-input")
	}
}

func TestDoAndUnmarshalNotJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client: http.DefaultClient,
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}
	var data interface{}
	wantPrefix := "error while parsing response: "

	resp, err := client.DoAndUnmarshal(context.Background(), get, &data)
	if resp != nil || err == nil || !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("DoAndUnmarshal() = (%v, %v); want = (nil, %q)", resp, err, wantPrefix)
	}

	if data != nil {
		t.Errorf("Data = %v; want = nil", data)
	}
}

func TestDoAndUnmarshalNilPointer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &HTTPClient{
		Client: http.DefaultClient,
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}

	resp, err := client.DoAndUnmarshal(context.Background(), get, nil)
	if err != nil {
		t.Fatalf("DoAndUnmarshal() = %v; want = nil", err)
	}

	if resp.Status != http.StatusOK {
		t.Errorf("Status = %d; want = %d", resp.Status, http.StatusOK)
	}
}

func TestDoAndUnmarshalTransportError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	server := httptest.NewServer(handler)
	server.Close()

	client := &HTTPClient{
		Client: http.DefaultClient,
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    server.URL,
	}
	var data interface{}
	wantPrefix := "error while making http call: "

	resp, err := client.DoAndUnmarshal(context.Background(), get, &data)
	if resp != nil || err == nil || !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("DoAndUnmarshal() = (%v, %v); want = (nil, %q)", resp, err, wantPrefix)
	}

	if data != nil {
		t.Errorf("Data = %v; want = nil", data)
	}
}

type responseBody struct {
	Name string `json:"name"`
}
