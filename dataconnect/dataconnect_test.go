// Copyright 2024 Google Inc. All Rights Reserved.
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

package dataconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
)

const (
	testProjectID = "mock-project-id"
	testLocation  = "us-central1"
	testServiceID = "mock-service"
	testVersion   = "test-version"
)

func TestNewClient(t *testing.T) {
	os.Unsetenv(emulatorHostEnvVar)
	conf := &internal.DataConnectConfig{
		ProjectID: testProjectID,
		Location:  testLocation,
		ServiceID: testServiceID,
		Version:   testVersion,
		Opts: []option.ClientOption{
			option.WithTokenSource(&internal.MockTokenSource{AccessToken: "test-token"}),
		},
	}

	client, err := NewClient(context.Background(), conf)
	t.Logf("client: %+v, err: %v", client, err)
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}

	wantEndpoint := fmt.Sprintf("https://dataconnect.googleapis.com/v1/projects/%s/locations/%s/services/%s", testProjectID, testLocation, testServiceID)
	if client.endpoint != wantEndpoint {
		t.Errorf("client.endpoint = %q; want = %q", client.endpoint, wantEndpoint)
	}
}

func TestNewClientEmulator(t *testing.T) {
	os.Setenv(emulatorHostEnvVar, "localhost:9099")
	defer os.Unsetenv(emulatorHostEnvVar)

	conf := &internal.DataConnectConfig{
		ProjectID: testProjectID,
		Location:  testLocation,
		ServiceID: testServiceID,
		Version:   testVersion,
	}

	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}

	wantEndpoint := fmt.Sprintf("http://localhost:9099/v1/projects/%s/locations/%s/services/%s", testProjectID, testLocation, testServiceID)
	if client.endpoint != wantEndpoint {
		t.Errorf("client.endpoint = %q; want = %q", client.endpoint, wantEndpoint)
	}
}

func TestExecuteGraphQL(t *testing.T) {
	var (
		reqBody   map[string]interface{}
		reqPath   string
		reqMethod string
	)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath = r.URL.Path
		reqMethod = r.Method
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"user": {"name": "test-user"}}}`))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	os.Setenv(emulatorHostEnvVar, strings.TrimPrefix(server.URL, "http://"))
	defer os.Unsetenv(emulatorHostEnvVar)

	conf := &internal.DataConnectConfig{
		ProjectID: testProjectID,
		Location:  testLocation,
		ServiceID: testServiceID,
		Version:   testVersion,
	}
	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}

	query := "query { user { name } }"
	options := &GraphQLOptions{
		Variables:     map[string]interface{}{"id": "123"},
		OperationName: "GetUser",
	}
	resp, err := client.ExecuteGraphQL(context.Background(), query, options)
	if err != nil {
		t.Fatalf("ExecuteGraphQL() = %v", err)
	}

	wantPath := fmt.Sprintf("/v1/projects/%s/locations/%s/services/%s:executeGraphql", testProjectID, testLocation, testServiceID)
	if reqPath != wantPath {
		t.Errorf("Request path = %q; want = %q", reqPath, wantPath)
	}
	if reqMethod != "POST" {
		t.Errorf("Request method = %q; want = %q", reqMethod, "POST")
	}

	wantBody := map[string]interface{}{
		"query":         query,
		"variables":     options.Variables,
		"operationName": options.OperationName,
	}
	if !reflect.DeepEqual(reqBody, wantBody) {
		t.Errorf("Request body = %v; want = %v", reqBody, wantBody)
	}

	wantResp := &ExecuteGraphQLResponse{
		Data: map[string]interface{}{"user": map[string]interface{}{"name": "test-user"}},
	}
	if !reflect.DeepEqual(resp, wantResp) {
		t.Errorf("Response = %v; want = %v", resp, wantResp)
	}
}

func TestExecuteGraphQLRead(t *testing.T) {
	var (
		reqBody   map[string]interface{}
		reqPath   string
		reqMethod string
	)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath = r.URL.Path
		reqMethod = r.Method
		json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": {"user": {"name": "test-user-read"}}}`))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	os.Setenv(emulatorHostEnvVar, strings.TrimPrefix(server.URL, "http://"))
	defer os.Unsetenv(emulatorHostEnvVar)

	conf := &internal.DataConnectConfig{
		ProjectID: testProjectID,
		Location:  testLocation,
		ServiceID: testServiceID,
		Version:   testVersion,
	}
	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}

	query := "query { user { name } }"
	resp, err := client.ExecuteGraphQLRead(context.Background(), query, nil)
	if err != nil {
		t.Fatalf("ExecuteGraphQLRead() = %v", err)
	}

	wantPath := fmt.Sprintf("/v1/projects/%s/locations/%s/services/%s:executeGraphql", testProjectID, testLocation, testServiceID)
	if reqPath != wantPath {
		t.Errorf("Request path = %q; want = %q", reqPath, wantPath)
	}
	if reqMethod != "POST" {
		t.Errorf("Request method = %q; want = %q", reqMethod, "POST")
	}

	wantBody := map[string]interface{}{
		"query": query,
	}
	if !reflect.DeepEqual(reqBody, wantBody) {
		t.Errorf("Request body = %v; want = %v", reqBody, wantBody)
	}

	wantResp := &ExecuteGraphQLResponse{
		Data: map[string]interface{}{"user": map[string]interface{}{"name": "test-user-read"}},
	}
	if !reflect.DeepEqual(resp, wantResp) {
		t.Errorf("Response = %v; want = %v", resp, wantResp)
	}
}

func TestExecuteGraphQLError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "test-error", "status": "INVALID_ARGUMENT"}}`))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	os.Setenv(emulatorHostEnvVar, strings.TrimPrefix(server.URL, "http://"))
	defer os.Unsetenv(emulatorHostEnvVar)

	conf := &internal.DataConnectConfig{
		ProjectID: testProjectID,
		Location:  testLocation,
		ServiceID: testServiceID,
		Version:   testVersion,
	}
	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}

	_, err = client.ExecuteGraphQL(context.Background(), "invalid-query", nil)
	if err == nil {
		t.Fatal("ExecuteGraphQL() = nil; want = error")
	}

	wantError := "test-error"
	if !strings.Contains(err.Error(), wantError) {
		t.Errorf("Error message = %q; want to contain = %q", err.Error(), wantError)
	}
}
