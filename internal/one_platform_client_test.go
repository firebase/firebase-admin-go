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

func TestGet(t *testing.T) {
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

	client := &OnePlatformClient{
		BaseURL:    server.URL,
		APIVersion: "v1",
		ProjectID:  "project-id",
		HTTPClient: &HTTPClient{Client: http.DefaultClient},
	}

	var data struct {
		Name string `json:"name"`
	}
	resp, err := client.Get(context.Background(), "/test", &data)
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

	wantURL := "/v1/projects/project-id/test"
	if req.URL.Path != wantURL {
		t.Errorf("URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestPost(t *testing.T) {
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

	client := &OnePlatformClient{
		BaseURL:    server.URL,
		APIVersion: "v1",
		ProjectID:  "project-id",
		HTTPClient: &HTTPClient{Client: http.DefaultClient},
	}

	entity := struct {
		Input string `json:"input"`
	}{
		Input: "test-input",
	}
	var data struct {
		Name string `json:"name"`
	}
	resp, err := client.Post(context.Background(), "/test", &entity, &data)
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

	wantURL := "/v1/projects/project-id/test"
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

func TestNonJsonResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &OnePlatformClient{
		BaseURL:    server.URL,
		APIVersion: "v1",
		ProjectID:  "project-id",
		HTTPClient: &HTTPClient{Client: http.DefaultClient},
	}

	var data interface{}
	wantPrefix := "error while parsing response: "
	resp, err := client.MakeRequest(context.Background(), http.MethodGet, "/test", nil, &data)
	if resp != nil || err == nil || !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("MakeRequest() = (%v, %v); want = (nil, %q)", resp, err, wantPrefix)
	}

	if data != nil {
		t.Errorf("Data = %v; want = nil", data)
	}
}

func TestTransportError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	server := httptest.NewServer(handler)
	server.Close()

	client := &OnePlatformClient{
		BaseURL:    server.URL,
		APIVersion: "v1",
		ProjectID:  "project-id",
		HTTPClient: &HTTPClient{Client: http.DefaultClient},
	}

	var data interface{}
	wantPrefix := "error while calling remote service: "
	resp, err := client.MakeRequest(context.Background(), http.MethodGet, "/test", nil, &data)
	if resp != nil || err == nil || !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("MakeRequest() = (%v, %v); want = (nil, %q)", resp, err, wantPrefix)
	}

	if data != nil {
		t.Errorf("Data = %v; want = nil", data)
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

	client := &OnePlatformClient{
		BaseURL:    server.URL,
		APIVersion: "v1",
		ProjectID:  "project-id",
		HTTPClient: &HTTPClient{Client: http.DefaultClient},
	}

	want := "Requested entity not found"
	resp, err := client.MakeRequest(context.Background(), http.MethodGet, "/test", nil, nil)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("MakeRequest() = (%v, %v); want = (nil, %q)", resp, err, want)
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

	client := &OnePlatformClient{
		BaseURL:    server.URL,
		APIVersion: "v1",
		ProjectID:  "project-id",
		HTTPClient: &HTTPClient{Client: http.DefaultClient},
	}

	want := "unexpected http response with status: 404; body: {}"
	resp, err := client.MakeRequest(context.Background(), http.MethodGet, "/test", nil, nil)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("MakeRequest() = (%v, %v); want = (nil, %q)", resp, err, want)
	}

	if !HasErrorCode(err, "UNKNOWN") {
		t.Errorf("ErrorCode = %q; want = %q", err.(*FirebaseError).Code, "UNKNOWN")
	}
}

func TestCustomErrorHandler(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("{}"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := &OnePlatformClient{
		BaseURL:    server.URL,
		APIVersion: "v1",
		ProjectID:  "project-id",
		HTTPClient: &HTTPClient{Client: http.DefaultClient},
		CreateErr: func(r *Response) error {
			return fmt.Errorf("custom error with status: %d", r.Status)
		},
	}

	want := "custom error with status: 404"
	resp, err := client.MakeRequest(context.Background(), http.MethodGet, "/test", nil, nil)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("MakeRequest() = (%v, %v); want = (nil, %q)", resp, err, want)
	}
}
