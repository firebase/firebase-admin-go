// Package dataconnect provides functions for interacting with the Firebase Data Connect service.
package dataconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	testProjectID = "test-project-id"
	testLocation  = "test-location"
	testServiceID = "test-service-id"
	testVersion   = "test-version"
)

func TestNewClient(t *testing.T) {
	conf := &internal.DataConnectConfig{
		ProjectID: testProjectID,
		Location:  testLocation,
		ServiceID: testServiceID,
		Version:   testVersion,
		Opts:      []option.ClientOption{option.WithoutAuthentication()},
	}

	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.projectID != testProjectID {
		t.Errorf("client.projectID = %q; want = %q", client.projectID, testProjectID)
	}
	if client.location != testLocation {
		t.Errorf("client.location = %q; want = %q", client.location, testLocation)
	}
	if client.serviceID != testServiceID {
		t.Errorf("client.serviceID = %q; want = %q", client.serviceID, testServiceID)
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
		Opts:      []option.ClientOption{option.WithoutAuthentication()},
	}

	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if !client.isEmulator {
		t.Error("client.isEmulator = false; want = true")
	}
	if client.emulatorHost != "localhost:9099" {
		t.Errorf("client.emulatorHost = %q; want = %q", client.emulatorHost, "localhost:9099")
	}
}

func TestExecuteGraphql(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := fmt.Sprintf("/%s/projects/%s/locations/%s/services/%s:%s", apiVersion, testProjectID, testLocation, testServiceID, executeGraphqlEndpoint)
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q; want = %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != wantPath {
			t.Errorf("Path = %q; want = %q", r.URL.Path, wantPath)
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}

		if req["query"] != "test query" {
			t.Errorf("req.query = %q; want = %q", req["query"], "test query")
		}

		resp := &ExecuteGraphqlResponse{
			Data: map[string]interface{}{"foo": "bar"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, err := newTestClient(ts)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.ExecuteGraphql(context.Background(), "test query", nil)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}

	want := &ExecuteGraphqlResponse{
		Data: map[string]interface{}{"foo": "bar"},
	}
	if !reflect.DeepEqual(resp, want) {
		t.Errorf("ExecuteGraphql() response = %#v; want = %#v", resp, want)
	}
}

func TestExecuteGraphqlRead(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := fmt.Sprintf("/%s/projects/%s/locations/%s/services/%s:%s", apiVersion, testProjectID, testLocation, testServiceID, executeGraphqlReadEndpoint)
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q; want = %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != wantPath {
			t.Errorf("Path = %q; want = %q", r.URL.Path, wantPath)
		}
		resp := &ExecuteGraphqlResponse{
			Data: map[string]interface{}{"foo": "bar"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, err := newTestClient(ts)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.ExecuteGraphqlRead(context.Background(), "test query", nil)
	if err != nil {
		t.Fatalf("ExecuteGraphqlRead() error = %v", err)
	}

	want := &ExecuteGraphqlResponse{
		Data: map[string]interface{}{"foo": "bar"},
	}
	if !reflect.DeepEqual(resp, want) {
		t.Errorf("ExecuteGraphqlRead() response = %#v; want = %#v", resp, want)
	}
}

func TestExecuteGraphqlError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"test error"}}`))
	}))
	defer ts.Close()

	client, err := newTestClient(ts)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.ExecuteGraphql(context.Background(), "test query", nil)
	if err == nil {
		t.Fatal("ExecuteGraphql() error = nil; want error")
	}
}

func TestExecuteGraphqlQueryError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"errors":[{"message":"test query error"}]}`))
	}))
	defer ts.Close()

	client, err := newTestClient(ts)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.ExecuteGraphql(context.Background(), "test query", nil)
	if err == nil {
		t.Fatal("ExecuteGraphql() error = nil; want error")
	}

	if !IsQueryError(err) {
		t.Error("IsQueryError() = false; want = true")
	}

	if !strings.Contains(err.Error(), "test query error") {
		t.Errorf("error message = %q; want to contain %q", err.Error(), "test query error")
	}
}

func TestIsQueryError(t *testing.T) {
	testQueryError := &internal.FirebaseError{
		ErrorCode: queryError,
		String:    "GraphQL query failed: test",
	}

	otherFirebaseError := &internal.FirebaseError{
		ErrorCode: internal.Unknown,
		String:    "Unknown error",
	}

	otherError := fmt.Errorf("some other error")

	if !IsQueryError(testQueryError) {
		t.Error("IsQueryError(queryError) = false; want = true")
	}
	if IsQueryError(otherFirebaseError) {
		t.Error("IsQueryError(otherFirebaseError) = true; want = false")
	}
	if IsQueryError(otherError) {
		t.Error("IsQueryError(otherError) = true; want = false")
	}
}

func newTestClient(ts *httptest.Server) (*Client, error) {
	emulatorHost := strings.TrimPrefix(ts.URL, "http://")
	os.Setenv(emulatorHostEnvVar, emulatorHost)

	conf := &internal.DataConnectConfig{
		ProjectID: testProjectID,
		Location:  testLocation,
		ServiceID: testServiceID,
		Version:   testVersion,
		Opts:      []option.ClientOption{option.WithoutAuthentication()},
	}

	client, err := NewClient(context.Background(), conf)
	os.Unsetenv(emulatorHostEnvVar) // Clean up env var
	return client, err
}
