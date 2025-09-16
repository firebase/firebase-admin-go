// Copyright 2024 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may not obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package dataconnect provides functions for interacting with the Firebase Data Connect service.
package dataconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/transport"
)

const (
	dataConnectProdURLFormat   = "https://firebasedataconnect.googleapis.com"
	emulatorHostEnvVar         = "DATA_CONNECT_EMULATOR_HOST"
	apiVersion                 = "v1alpha"
	executeGraphqlEndpoint     = "executeGraphql"
	executeGraphqlReadEndpoint = "executeGraphqlRead"
)

// Client is the interface for the Firebase Data Connect service.
type Client struct {
	client    *internal.HTTPClient
	projectID string
	location  string
	serviceID string
	endpoint  string
}

// NewClient creates a new instance of the Data Connect client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// Data Connect service through firebase.App.
func NewClient(ctx context.Context, conf *internal.DataConnectConfig) (*Client, error) {
	if conf.ProjectID == "" {
		return nil, fmt.Errorf("project ID is required to initialize Data Connect client")
	}
	if conf.Location == "" {
		return nil, fmt.Errorf("location is required to initialize Data Connect client")
	}
	if conf.ServiceID == "" {
		return nil, fmt.Errorf("service ID is required to initialize Data Connect client")
	}

	var endpoint string
	if host := os.Getenv(emulatorHostEnvVar); host != "" {
		endpoint = "http://" + host
	} else {
		endpoint = dataConnectProdURLFormat
	}

	httpClient, _, err := transport.NewHTTPClient(ctx, conf.Opts...)
	if err != nil {
		return nil, err
	}

	client := &internal.HTTPClient{
		Client:      httpClient,
		CreateErrFn: handleError,
	}

	return &Client{
		client:    client,
		projectID: conf.ProjectID,
		location:  conf.Location,
		serviceID: conf.ServiceID,
		endpoint:  endpoint,
	}, nil
}

// IsQueryError checks if the given error is a query error.
// A query error is returned when the backend successfully processes a request
// but the GraphQL query itself fails to execute due to schema or data-related issues.
func IsQueryError(err error) bool {
	fe, ok := err.(*internal.FirebaseError)
	return ok && fe.ErrorCode == "query-error"
}

// handleError creates a new FirebaseError from an HTTP response.
func handleError(resp *internal.Response) error {
	return internal.NewFirebaseError(resp)
}

// graphqlRequest is the JSON payload for a GraphQL request.
type graphqlRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// ExecuteGraphql executes a GraphQL query or mutation.
func (c *Client) ExecuteGraphql(ctx context.Context, query string, options *GraphqlOptions) (*ExecuteGraphqlResponse, error) {
	return c.execute(ctx, executeGraphqlEndpoint, query, options)
}

// ExecuteGraphqlRead executes a GraphQL read-only query.
func (c *Client) ExecuteGraphqlRead(ctx context.Context, query string, options *GraphqlOptions) (*ExecuteGraphqlResponse, error) {
	return c.execute(ctx, executeGraphqlReadEndpoint, query, options)
}

func (c *Client) execute(ctx context.Context, endpoint, query string, options *GraphqlOptions) (*ExecuteGraphqlResponse, error) {
	if query == "" {
		return nil, &internal.FirebaseError{
			ErrorCode: internal.InvalidArgument,
			String:    "query must not be empty",
		}
	}

	payload := graphqlRequest{Query: query}
	if options != nil {
		payload.Variables = options.Variables
		payload.OperationName = options.OperationName
	}

	url := fmt.Sprintf("%s/%s/projects/%s/locations/%s/services/%s:%s", c.endpoint, apiVersion, c.projectID, c.location, c.serviceID, endpoint)
	request := &internal.Request{
		Method: http.MethodPost,
		URL:    url,
		Body:   internal.NewJSONEntity(payload),
	}

	var result ExecuteGraphqlResponse
	resp, err := c.client.DoAndUnmarshal(ctx, request, &result)
	if err != nil {
		if _, ok := err.(*internal.FirebaseError); ok {
			return nil, err
		}
		var httpResp *http.Response
		if resp != nil {
			httpResp = resp.LowLevelResponse()
		}
		return nil, &internal.FirebaseError{
			ErrorCode: internal.Unknown,
			String:    fmt.Sprintf("failed to execute request: %v", err),
			Response:  httpResp,
		}
	}

	var errResp struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && len(errResp.Errors) > 0 {
		var messages []string
		for _, e := range errResp.Errors {
			messages = append(messages, e.Message)
		}
		return nil, &internal.FirebaseError{
			ErrorCode: "query-error",
			String:    strings.Join(messages, "; "),
			Response:  resp.LowLevelResponse(),
		}
	}

	return &result, nil
}
