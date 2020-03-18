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
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
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

func TestTimeoutError(t *testing.T) {
	client := &HTTPClient{
		Client: &http.Client{
			Transport: &faultyTransport{
				Err: &timeoutError{},
			},
		},
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    "http://test.url",
	}
	want := "timed out while making an http call"

	resp, err := client.Do(context.Background(), get)
	if resp != nil || err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
	}

	fe, ok := err.(*FirebaseError)
	if !ok {
		t.Fatalf("Do() err = %v; want = FirebaseError", err)
	}

	if fe.ErrorCode != DeadlineExceeded {
		t.Errorf("Do() err.ErrorCode = %q; want = %q", fe.ErrorCode, DeadlineExceeded)
	}
	if fe.Response != nil {
		t.Errorf("Do() err.Response = %v; want = nil", fe.Response)
	}
	if fe.Ext == nil || len(fe.Ext) > 0 {
		t.Errorf("Do() err.Ext = %v; want = empty-map", fe.Ext)
	}
}

type timeoutError struct{}

func (t *timeoutError) Error() string {
	return "test timeout error"
}

func (t *timeoutError) Timeout() bool {
	return true
}

func TestNetworkOutageError(t *testing.T) {
	errors := []struct {
		name string
		err  error
	}{
		{"NetDialError", &net.OpError{Op: "dial", Err: errors.New("test error")}},
		{"NetReadError", &net.OpError{Op: "read", Err: errors.New("test error")}},
		{
			"WrappedNetReadError",
			&net.OpError{
				Op:  "test",
				Err: &net.OpError{Op: "read", Err: errors.New("test error")},
			},
		},
		{"ECONNREFUSED", syscall.ECONNREFUSED},
	}

	get := &Request{
		Method: http.MethodGet,
		URL:    "http://test.url",
	}
	want := "failed to establish a connection"

	for _, tc := range errors {
		t.Run(tc.name, func(t *testing.T) {
			client := &HTTPClient{
				Client: &http.Client{
					Transport: &faultyTransport{
						Err: tc.err,
					},
				},
			}

			resp, err := client.Do(context.Background(), get)
			if resp != nil || err == nil || !strings.HasPrefix(err.Error(), want) {
				t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
			}

			fe, ok := err.(*FirebaseError)
			if !ok {
				t.Fatalf("Do() err = %v; want = FirebaseError", err)
			}

			if fe.ErrorCode != Unavailable {
				t.Errorf("Do() err.ErrorCode = %q; want = %q", fe.ErrorCode, Unavailable)
			}
			if fe.Response != nil {
				t.Errorf("Do() err.Response = %v; want = nil", fe.Response)
			}
			if fe.Ext == nil || len(fe.Ext) > 0 {
				t.Errorf("Do() err.Ext = %v; want = empty-map", fe.Ext)
			}
		})
	}
}

func TestUnknownNetworkError(t *testing.T) {
	client := &HTTPClient{
		Client: &http.Client{
			Transport: &faultyTransport{
				Err: errors.New("unknown error"),
			},
		},
	}
	get := &Request{
		Method: http.MethodGet,
		URL:    "http://test.url",
	}
	want := "unknown error while making an http call"

	resp, err := client.Do(context.Background(), get)
	if resp != nil || err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("Do() = (%v, %v); want = (nil, %q)", resp, err, want)
	}

	fe, ok := err.(*FirebaseError)
	if !ok {
		t.Fatalf("Do() err = %v; want = FirebaseError", err)
	}

	if fe.ErrorCode != Unknown {
		t.Errorf("Do() err.ErrorCode = %q; want = %q", fe.ErrorCode, Unknown)
	}
	if fe.Response != nil {
		t.Errorf("Do() err.Response = %v; want = nil", fe.Response)
	}
	if fe.Ext == nil || len(fe.Ext) > 0 {
		t.Errorf("Do() err.Ext = %v; want = empty-map", fe.Ext)
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
