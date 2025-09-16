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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
)

const (
	testProjectID = "test-project-id"
	testLocation  = "test-location"
	testServiceID = "test-service-id"
	testQuery     = "query { users { id, name } }"
)

var (
	successResponse = &ExecuteGraphqlResponse{
		Data: map[string]interface{}{"foo": "bar"},
	}
	queryErrorJSON = `{"errors": [{"message": "Something went wrong"}]}`
	queryErrorMsg  = "Something went wrong"
)

// mockServer is a convenient helper for testing against a mock HTTP server.
type mockServer struct {
	server *httptest.Server
	client *Client
	req    *http.Request
}

// newMockServer creates a new mock server and a corresponding Data Connect client.
func newMockServer(t *testing.T, h http.HandlerFunc) *mockServer {
	t.Helper()
	server := httptest.NewServer(h)

	// Set the emulator host env var to the mock server's address.
	originalHost := os.Getenv(emulatorHostEnvVar)
	os.Setenv(emulatorHostEnvVar, server.Listener.Addr().String())

	conf := &internal.DataConnectConfig{
		ProjectID: testProjectID,
		Location:  testLocation,
		ServiceID: testServiceID,
		Opts:      []option.ClientOption{option.WithoutAuthentication()},
	}
	client, err := NewClient(context.Background(), conf)
	if err != nil {
		server.Close()
		os.Setenv(emulatorHostEnvVar, originalHost)
		t.Fatalf("NewClient() = %v; want nil", err)
	}

	return &mockServer{
		server: server,
		client: client,
	}
}

func (s *mockServer) Close() {
	s.server.Close()
}

func TestNewClient(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		conf := &internal.DataConnectConfig{
			ProjectID: testProjectID,
			Location:  testLocation,
			ServiceID: testServiceID,
			Opts:      []option.ClientOption{option.WithoutAuthentication()},
		}
		client, err := NewClient(context.Background(), conf)
		if err != nil {
			t.Fatalf("NewClient() error = %v; want nil", err)
		}
		if client.projectID != testProjectID {
			t.Errorf("client.projectID = %q; want %q", client.projectID, testProjectID)
		}
		if client.endpoint != dataConnectProdURLFormat {
			t.Errorf("client.endpoint = %q; want %q", client.endpoint, dataConnectProdURLFormat)
		}
	})

	t.Run("Emulator", func(t *testing.T) {
		host := "localhost:9099"
		os.Setenv(emulatorHostEnvVar, host)
		defer os.Unsetenv(emulatorHostEnvVar)

		conf := &internal.DataConnectConfig{
			ProjectID: testProjectID,
			Location:  testLocation,
			ServiceID: testServiceID,
			Opts:      []option.ClientOption{option.WithoutAuthentication()},
		}
		client, err := NewClient(context.Background(), conf)
		if err != nil {
			t.Fatalf("NewClient() with emulator error = %v; want nil", err)
		}
		if client.endpoint != "http://"+host {
			t.Errorf("client.endpoint = %q; want %q", client.endpoint, "http://"+host)
		}
	})

	validationTestCases := []struct {
		name   string
		config *internal.DataConnectConfig
	}{
		{"MissingProjectID", &internal.DataConnectConfig{Location: testLocation, ServiceID: testServiceID}},
		{"MissingLocation", &internal.DataConnectConfig{ProjectID: testProjectID, ServiceID: testServiceID}},
		{"MissingServiceID", &internal.DataConnectConfig{ProjectID: testProjectID, Location: testLocation}},
	}
	for _, tc := range validationTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(context.Background(), tc.config); err == nil {
				t.Errorf("NewClient() error = nil; want error")
			}
		})
	}
}

func TestIsQueryError(t *testing.T) {
	queryErr := &internal.FirebaseError{ErrorCode: "query-error"}
	otherErr := &internal.FirebaseError{ErrorCode: "internal-error"}
	nonFBErr := errors.New("not a firebase error")

	if !IsQueryError(queryErr) {
		t.Errorf("IsQueryError(queryErr) = false; want true")
	}
	if IsQueryError(otherErr) {
		t.Errorf("IsQueryError(otherErr) = true; want false")
	}
	if IsQueryError(nonFBErr) {
		t.Errorf("IsQueryError(nonFBErr) = true; want false")
	}
	if IsQueryError(nil) {
		t.Errorf("IsQueryError(nil) = true; want false")
	}
}

type executeTestCase struct {
	name       string
	scenario   func(w http.ResponseWriter, r *http.Request)
	query      string
	options    *GraphqlOptions
	wantResp   *ExecuteGraphqlResponse
	wantErr    error
	checkError func(t *testing.T, err error)
}

func TestExecuteGraphql(t *testing.T) {
	runExecuteTest(t, "ExecuteGraphql", func(c *Client, q string, o *GraphqlOptions) (*ExecuteGraphqlResponse, error) {
		return c.ExecuteGraphql(context.Background(), q, o)
	}, executeGraphqlEndpoint)
}

func TestExecuteGraphqlRead(t *testing.T) {
	runExecuteTest(t, "ExecuteGraphqlRead", func(c *Client, q string, o *GraphqlOptions) (*ExecuteGraphqlResponse, error) {
		return c.ExecuteGraphqlRead(context.Background(), q, o)
	}, executeGraphqlReadEndpoint)
}

func runExecuteTest(t *testing.T, methodName string, fn func(*Client, string, *GraphqlOptions) (*ExecuteGraphqlResponse, error), endpoint string) {
	testCases := []executeTestCase{
		{
			name: "Success",
			scenario: func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(successResponse)
			},
			query:    testQuery,
			wantResp: successResponse,
		},
		{
			name:  "EmptyQuery",
			query: "",
			checkError: func(t *testing.T, err error) {
				if !internal.HasPlatformErrorCode(err, internal.InvalidArgument) {
					t.Errorf("err = %v; want InvalidArgument", err)
				}
			},
		},
		{
			name: "QueryError",
			scenario: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(queryErrorJSON))
			},
			query: testQuery,
			checkError: func(t *testing.T, err error) {
				if !IsQueryError(err) {
					t.Fatalf("IsQueryError() = false; want true")
				}
				if err.Error() != queryErrorMsg {
					t.Errorf("err.Error() = %q; want %q", err.Error(), queryErrorMsg)
				}
			},
		},
		{
			name: "HTTPError",
			scenario: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "internal server error", http.StatusInternalServerError)
			},
			query: testQuery,
			checkError: func(t *testing.T, err error) {
				if !internal.HasPlatformErrorCode(err, internal.Internal) {
					t.Errorf("err = %v; want Internal", err)
				}
			},
		},
		{
			name: "BadJSONResponse",
			scenario: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("this is not json"))
			},
			query: testQuery,
			checkError: func(t *testing.T, err error) {
				if !internal.HasPlatformErrorCode(err, internal.Unknown) {
					t.Errorf("err = %v; want Unknown", err)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/%s/projects/%s/locations/%s/services/%s:%s", apiVersion, testProjectID, testLocation, testServiceID, endpoint)
				if r.URL.Path != expectedPath {
					http.Error(w, fmt.Sprintf("unexpected path: got %q, want %q", r.URL.Path, expectedPath), http.StatusBadRequest)
					return
				}
				if tc.scenario != nil {
					tc.scenario(w, r)
				}
			})
			server := newMockServer(t, handler)
			defer server.Close()

			resp, err := fn(server.client, tc.query, tc.options)
			if tc.checkError != nil {
				if err == nil {
					t.Fatalf("got response = %#v; want error", resp)
				}
				tc.checkError(t, err)
			} else {
				if err != nil {
					t.Fatalf("got error = %v; want nil", err)
				}
				if !reflect.DeepEqual(resp, tc.wantResp) {
					t.Errorf("response = %#v; want %#v", resp, tc.wantResp)
				}
			}
		})
	}
}
