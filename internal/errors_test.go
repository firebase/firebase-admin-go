// Copyright 2020 Google Inc. All Rights Reserved.
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
	"testing"
)

var platformErrorCodes = []ErrorCode{
	InvalidArgument,
	Unauthenticated,
	NotFound,
	Aborted,
	AlreadyExists,
	Internal,
	Unavailable,
	Unknown,
}

func TestPlatformError(t *testing.T) {
	var body string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(body))
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
	want := "Test error message"

	for _, code := range platformErrorCodes {
		body = fmt.Sprintf(`{
			"error": {
				"status": %q,
				"message": "Test error message"
			}
		}`, code)

		resp, err := client.Do(context.Background(), get)
		if resp != nil || err == nil || err.Error() != want {
			t.Fatalf("[%s]: Do() = (%v, %v); want = (nil, %q)", code, resp, err, want)
		}
		if !HasPlatformErrorCode(err, code) {
			t.Errorf("[%s]: HasPlatformErrorCode() = false; want = true", code)
		}

		fe, ok := err.(*FirebaseError)
		if !ok {
			t.Fatalf("[%s]: Do() err = %v; want = FirebaseError", code, err)
		}

		if fe.ErrorCode != code {
			t.Errorf("[%s]: Do() err.ErrorCode = %q; want = %q", code, fe.ErrorCode, code)
		}
		if fe.Response == nil {
			t.Fatalf("[%s]: Do() err.Response = nil; want = non-nil", code)
		}
		if fe.Response.StatusCode != http.StatusNotFound {
			t.Errorf("[%s]: Do() err.Response.StatusCode = %d; want = %d", code, fe.Response.StatusCode, http.StatusNotFound)
		}
		if fe.Ext == nil || len(fe.Ext) > 0 {
			t.Errorf("[%s]: Do() err.Ext = %v; want = empty-map", code, fe.Ext)
		}
	}
}

func TestPlatformErrorWithoutDetails(t *testing.T) {
	var status int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte("{}"))
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

	httpStatusMappings := map[int]ErrorCode{
		http.StatusNotImplemented: Unknown,
	}

	// Add known error code mappings
	for k, v := range httpStatusToErrorCodes {
		httpStatusMappings[k] = v
	}

	for httpStatus, platformCode := range httpStatusMappings {
		status = httpStatus
		want := fmt.Sprintf("unexpected http response with status: %d\n{}", httpStatus)

		resp, err := client.Do(context.Background(), get)
		if resp != nil || err == nil || err.Error() != want {
			t.Fatalf("[%d]: Do() = (%v, %v); want = (nil, %q)", httpStatus, resp, err, want)
		}
		if !HasPlatformErrorCode(err, platformCode) {
			t.Errorf("[%d]: HasPlatformErrorCode(%q) = false; want = true", httpStatus, platformCode)
		}

		fe, ok := err.(*FirebaseError)
		if !ok {
			t.Fatalf("[%d]: Do() err = %v; want = FirebaseError", httpStatus, err)
		}

		if fe.ErrorCode != platformCode {
			t.Errorf("[%d]: Do() err.ErrorCode = %q; want = %q", httpStatus, fe.ErrorCode, platformCode)
		}
		if fe.Response == nil {
			t.Fatalf("[%d]: Do() err.Response = nil; want = non-nil", httpStatus)
		}
		if fe.Response.StatusCode != httpStatus {
			t.Errorf("[%d]: Do() err.Response.StatusCode = %d; want = %d", httpStatus, fe.Response.StatusCode, httpStatus)
		}
		if fe.Ext == nil || len(fe.Ext) > 0 {
			t.Errorf("[%d]: Do() err.Ext = %v; want = empty-map", httpStatus, fe.Ext)
		}
	}
}

func TestErrorHTTPResponse(t *testing.T) {
	body := `{"key": "value"}`
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(body))
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
	want := fmt.Sprintf("unexpected http response with status: 500\n%s", body)

	resp, err := client.Do(context.Background(), get)
	if resp != nil || err == nil || err.Error() != want {
		t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
	}

	fe, ok := err.(*FirebaseError)
	if !ok {
		t.Fatalf("Do() err = %v; want = FirebaseError", err)
	}

	hr := fe.Response
	defer hr.Body.Close()
	if hr.StatusCode != http.StatusInternalServerError {
		t.Errorf("Do() Response.StatusCode = %d; want = %d", hr.StatusCode, http.StatusInternalServerError)
	}

	b, err := ioutil.ReadAll(hr.Body)
	if err != nil {
		t.Fatalf("ReadAll(Response.Body) = %v", err)
	}

	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal(Response.Body) = %v", err)
	}

	if len(m) != 1 || m["key"] != "value" {
		t.Errorf("Unmarshal(Response.Body) = %v; want = {key: value}", m)
	}
}
