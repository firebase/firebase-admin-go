// Package dataconnect contains functions for interacting with Firebase Data Connect services.
package dataconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"firebase.google.com/go/v4/internal"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const (
	emulatorHostEnvVar = "DATA_CONNECT_EMULATOR_HOST"
	dataconnectAPI     = "https://dataconnect.googleapis.com"
)

var emulatorToken = &oauth2.Token{AccessToken: "owner"}

// Client is the interface for the Firebase Data Connect service.
type Client struct {
	client   *internal.HTTPClient
	endpoint string
}

// NewClient creates a new instance of the Data Connect client.
func NewClient(ctx context.Context, conf *internal.DataConnectConfig) (*Client, error) {
	var opts []option.ClientOption
	var endpoint string

	if host := os.Getenv(emulatorHostEnvVar); host != "" {
		ts := oauth2.StaticTokenSource(emulatorToken)
		opts = append(opts, option.WithTokenSource(ts))
		endpoint = fmt.Sprintf(
			"http://%s/v1/projects/%s/locations/%s/services/%s",
			host, conf.ProjectID, conf.Location, conf.ServiceID)
	} else {
		opts = append(opts, conf.Opts...)
		endpoint = fmt.Sprintf(
			"%s/v1/projects/%s/locations/%s/services/%s",
			dataconnectAPI, conf.ProjectID, conf.Location, conf.ServiceID)
	}

	httpClient, _, err := transport.NewHTTPClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client: %w", err)
	}

	client := internal.WithDefaultRetryConfig(httpClient)
	client.CreateErrFn = handleHTTPError
	client.Opts = []internal.HTTPOption{
		internal.WithHeader("X-Client-Version", fmt.Sprintf("Go/Admin/%s", conf.Version)),
		internal.WithHeader("x-goog-api-client", internal.GetMetricsHeader(conf.Version)),
	}

	return &Client{
		client:   client,
		endpoint: endpoint,
	}, nil
}

// GraphQLOptions provides additional options for a GraphQL execution.
type GraphQLOptions struct {
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// ExecuteGraphQLResponse represents the response from a GraphQL execution.
type ExecuteGraphQLResponse struct {
	Data map[string]interface{} `json:"data"`
}

// ExecuteGraphQL executes a GraphQL query or mutation.
func (c *Client) ExecuteGraphQL(
	ctx context.Context,
	query string,
	options *GraphQLOptions) (*ExecuteGraphQLResponse, error) {
	return c.execute(ctx, query, options)
}

// ExecuteGraphQLRead executes a read-only GraphQL query.
func (c *Client) ExecuteGraphQLRead(
	ctx context.Context,
	query string,
	options *GraphQLOptions) (*ExecuteGraphQLResponse, error) {
	return c.execute(ctx, query, options)
}

func (c *Client) execute(
	ctx context.Context,
	query string,
	options *GraphQLOptions) (*ExecuteGraphQLResponse, error) {

	payload := map[string]interface{}{
		"query": query,
	}
	if options != nil {
		if options.Variables != nil {
			payload["variables"] = options.Variables
		}
		if options.OperationName != "" {
			payload["operationName"] = options.OperationName
		}
	}

	req := &internal.Request{
		Method: "POST",
		URL:    fmt.Sprintf("%s:executeGraphql", c.endpoint),
		Body:   internal.NewJSONEntity(payload),
	}

	resp, err := c.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var result ExecuteGraphQLResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

type httpError struct {
	Error struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func handleHTTPError(resp *internal.Response) error {
	var he httpError
	if err := json.Unmarshal(resp.Body, &he); err != nil {
		return &internal.FirebaseError{
			ErrorCode: internal.Unknown,
			String:    fmt.Sprintf("unexpected error response: %s", string(resp.Body)),
			Response:  resp.LowLevelResponse(),
		}
	}

	return &internal.FirebaseError{
		ErrorCode: internal.ErrorCode(he.Error.Status),
		String:    he.Error.Message,
		Response:  resp.LowLevelResponse(),
	}
}
