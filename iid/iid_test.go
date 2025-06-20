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

package iid

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"firebase.google.com/go/v4/app" // Import app package
	"firebase.google.com/go/v4/errorutils"
	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
)

const testProjectID = "test-project"

// Helper to create a new app.App for IID tests
func newTestIIDApp(ctx context.Context) *app.App {
	opts := []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
		option.WithScopes(internal.FirebaseScopes...), // Mimic firebase.NewApp
	}
	appInstance, err := app.New(ctx, &app.Config{ProjectID: testProjectID}, opts...)
	if err != nil {
		log.Fatalf("Error creating test app for IID: %v", err)
	}
	// SDKVersion is handled by appInstance.SDKVersion()
	return appInstance
}

func TestNoProjectID(t *testing.T) {
	// Create an app with no ProjectID
	ctx := context.Background()
	appInstance, err := app.New(ctx, &app.Config{ProjectID: ""}) // Explicitly empty ProjectID
	if err != nil {
		// This case should ideally not happen if config validation is minimal in app.New for ProjectID
		// and relies on service client constructors to validate.
		// If app.New itself errors on empty ProjectID (if it becomes mandatory there), this test needs adjustment.
		// For now, assume app.New allows it, and NewClient will fail.
		t.Logf("app.New with empty ProjectID returned error (unexpected for this test's focus on iid.NewClient): %v", err)
	}

	client, err := NewClient(ctx, appInstance)
	if client != nil || err == nil {
		t.Errorf("NewClient(appWithNoProjectID) = (%v, %v); want = (nil, error)", client, err)
	} else if err.Error() != "project ID is required to initialize Instance ID client" {
		t.Errorf("NewClient(appWithNoProjectID) error = %q; want = %q", err.Error(), "project ID is required to initialize Instance ID client")
	}
}

func TestInvalidInstanceID(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestIIDApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.DeleteInstanceID(ctx, ""); err == nil {
		t.Errorf("DeleteInstanceID(empty) = nil; want error")
	}
}

func TestDeleteInstanceID(t *testing.T) {
	var tr *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestIIDApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL // Override endpoint to use mock server

	if err := client.DeleteInstanceID(ctx, "test-iid"); err != nil {
		t.Errorf("DeleteInstanceID() = %v; want nil", err)
	}

	if tr == nil {
		t.Fatalf("Request = nil; want non-nil")
	}
	if tr.Method != http.MethodDelete {
		t.Errorf("Method = %q; want = %q", tr.Method, http.MethodDelete)
	}
	expectedPath := fmt.Sprintf("/project/%s/instanceId/test-iid", appInstance.ProjectID())
	if tr.URL.Path != expectedPath {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, expectedPath)
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
	xGoogAPIClientHeader := internal.GetMetricsHeader(appInstance.SDKVersion())
	if h := tr.Header.Get("x-goog-api-client"); h != xGoogAPIClientHeader {
		t.Errorf("x-goog-api-client header = %q; want = %q", h, xGoogAPIClientHeader)
	}
}

func TestDeleteInstanceIDError(t *testing.T) {
	status := http.StatusOK
	var tr *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		w.WriteHeader(status)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestIIDApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL
	if client.client != nil { // client.client is the internal.HTTPClient
		client.client.RetryConfig = nil
	}


	errorHandlers := map[int]func(error) bool{
		http.StatusBadRequest:          errorutils.IsInvalidArgument,
		http.StatusUnauthorized:        errorutils.IsUnauthenticated,
		http.StatusForbidden:           errorutils.IsPermissionDenied,
		http.StatusNotFound:            errorutils.IsNotFound,
		http.StatusConflict:            errorutils.IsConflict,
		http.StatusTooManyRequests:     errorutils.IsResourceExhausted,
		http.StatusInternalServerError: errorutils.IsInternal,
		http.StatusServiceUnavailable:  errorutils.IsUnavailable,
	}

	deprecatedErrorHandlers := map[int]func(error) bool{
		http.StatusBadRequest:          IsInvalidArgument, // Deprecated version
		http.StatusUnauthorized:        IsInsufficientPermission, // Deprecated version
		http.StatusForbidden:           IsInsufficientPermission, // Deprecated version
		http.StatusNotFound:            IsNotFound, // Deprecated version (same as errorutils for this one)
		http.StatusConflict:            IsAlreadyDeleted, // Deprecated version
		http.StatusTooManyRequests:     IsTooManyRequests, // Deprecated version
		http.StatusInternalServerError: IsInternal, // Deprecated version
		http.StatusServiceUnavailable:  IsServerUnavailable, // Deprecated version
	}

	for code, check := range errorHandlers {
		status = code
		want := fmt.Sprintf("instance id %q: %s", "test-iid", errorMessages[code])
		err := client.DeleteInstanceID(ctx, "test-iid")
		if err == nil || !check(err) || err.Error() != want {
			t.Errorf("DeleteInstanceID() for status %d = %v; want error matching %q and check function", code, err, want)
		}

		resp := errorutils.HTTPResponse(err)
		if resp == nil {
			t.Errorf("HTTPResponse() for status %d = nil; want non-nil response", code)
		} else if resp.StatusCode != code {
			t.Errorf("HTTPResponse().StatusCode for status %d = %d; want = %d", code, resp.StatusCode, code)
		}

		deprecatedCheck := deprecatedErrorHandlers[code]
		if !deprecatedCheck(err) {
			t.Errorf("Deprecated check DeleteInstanceID() for status %d = %v; did not satisfy deprecated check", code, err)
		}

		if tr == nil {
			t.Fatalf("Request = nil for status %d; want non-nil", code)
		}
		if tr.Method != http.MethodDelete {
			t.Errorf("Method for status %d = %q; want = %q", code, tr.Method, http.MethodDelete)
		}
		expectedPath := fmt.Sprintf("/project/%s/instanceId/test-iid", appInstance.ProjectID())
		if tr.URL.Path != expectedPath {
			t.Errorf("Path for status %d = %q; want = %q", code, tr.URL.Path, expectedPath)
		}
		if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
			t.Errorf("Authorization for status %d = %q; want = %q", code, h, "Bearer test-token")
		}
		xGoogAPIClientHeader := internal.GetMetricsHeader(appInstance.SDKVersion())
		if h := tr.Header.Get("x-goog-api-client"); h != xGoogAPIClientHeader {
			t.Errorf("x-goog-api-client header for status %d = %q; want = %q", code, h, xGoogAPIClientHeader)
		}
		tr = nil // Reset for next iteration
	}
}

func TestDeleteInstanceIDUnexpectedError(t *testing.T) {
	var tr *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		w.WriteHeader(511) // Some unexpected status
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	ctx := context.Background()
	appInstance := newTestIIDApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL

	want := "instance id \"test-iid\": unexpected http response with status: 511\n{}"
	err = client.DeleteInstanceID(ctx, "test-iid")
	if err == nil || err.Error() != want {
		t.Errorf("DeleteInstanceID() = %v; want = %v", err, want)
	}
	if !IsUnknown(err) { // Deprecated check
		t.Errorf("IsUnknown() = false; want = true")
	}
	if !errorutils.IsUnknown(err) {
		t.Errorf("errorutils.IsUnknown() = false; want = true")
	}

	if tr == nil {
		t.Fatalf("Request = nil; want non-nil")
	}
	if tr.Method != http.MethodDelete {
		t.Errorf("Method = %q; want = %q", tr.Method, http.MethodDelete)
	}
	expectedPath := fmt.Sprintf("/project/%s/instanceId/test-iid", appInstance.ProjectID())
	if tr.URL.Path != expectedPath {
		t.Errorf("Path = %q; want = %q", tr.URL.Path, expectedPath)
	}
	if h := tr.Header.Get("Authorization"); h != "Bearer test-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer test-token")
	}
	xGoogAPIClientHeader := internal.GetMetricsHeader(appInstance.SDKVersion())
	if h := tr.Header.Get("x-goog-api-client"); h != xGoogAPIClientHeader {
		t.Errorf("x-goog-api-client header = %q; want = %q", h, xGoogAPIClientHeader)
	}
}

func TestDeleteInstanceIDConnectionError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do nothing, server will be closed immediately
	}))
	ts.Close() // Close server immediately to simulate connection error

	ctx := context.Background()
	appInstance := newTestIIDApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = ts.URL
	if client.client != nil { // client.client is the internal.HTTPClient
		client.client.RetryConfig = nil // Disable retries for this test
	}


	if err := client.DeleteInstanceID(ctx, "test-iid"); err == nil {
		t.Fatalf("DeleteInstanceID() with connection error = nil; want = error")
	}
}
