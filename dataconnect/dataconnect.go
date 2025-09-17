// Package dataconnect provides functions for interacting with the Firebase Data Connect service.
package dataconnect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const (
	dataConnectProdURLFormat     = "https://firebasedataconnect.googleapis.com/%s/projects/%s/locations/%s/services/%s:%s"
	dataConnectEmulatorURLFormat = "http://%s/%s/projects/%s/locations/%s/services/%s:%s"
	emulatorHostEnvVar           = "FIREBASE_DATA_CONNECT_EMULATOR_HOST"
	apiVersion                   = "v1alpha"
	executeGraphqlEndpoint       = "executeGraphql"
	executeGraphqlReadEndpoint   = "executeGraphqlRead"

	// SDK-generated error codes
	queryError = "QUERY_ERROR"
)

// ConnectorConfig is the configuration for the Data Connect service.
type ConnectorConfig struct {
	Location  string `json:"location"`
	ServiceID string `json:"serviceId"`
}

// GraphqlOptions represents the options for a GraphQL query.
type GraphqlOptions struct {
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// ExecuteGraphqlResponse is the response from a GraphQL query.
type ExecuteGraphqlResponse struct {
	Data map[string]interface{} `json:"data"`
}

// Client is the interface for the Firebase Data Connect service.
type Client struct {
	client       *internal.HTTPClient
	projectID    string
	location     string
	serviceID    string
	isEmulator   bool
	emulatorHost string
}

// NewClient creates a new instance of the Data Connect client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// Data Connect service through firebase.App.
func NewClient(ctx context.Context, conf *internal.DataConnectConfig) (*Client, error) {
	var opts []option.ClientOption
	opts = append(opts, conf.Opts...)

	var isEmulator bool
	emulatorHost := os.Getenv(emulatorHostEnvVar)
	if emulatorHost != "" {
		isEmulator = true
	}

	transport, _, err := transport.NewHTTPClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	hc := internal.WithDefaultRetryConfig(transport)
	hc.CreateErrFn = handleError
	hc.SuccessFn = func(r *internal.Response) bool {
		// If the status isn't already a know success status we handle these responses normally
		if !internal.HasSuccessStatus(r) {
			return false
		}
		// Otherwise we check the successful response body for error
		var errResp graphqlQueryErrorResponse
		if err := json.Unmarshal(r.Body, &errResp); err != nil {
			return true // Cannot parse, assume no query errors and thus success
		}
		return len(errResp.Errors) == 0
	}
	hc.Opts = []internal.HTTPOption{
		internal.WithHeader("X-Client-Version", fmt.Sprintf("Go/Admin/%s", conf.Version)),
		internal.WithHeader("x-goog-api-client", internal.GetMetricsHeader(conf.Version)),
	}

	return &Client{
		client:       hc,
		projectID:    conf.ProjectID,
		location:     conf.Location,
		serviceID:    conf.ServiceID,
		isEmulator:   isEmulator,
		emulatorHost: emulatorHost,
	}, nil
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
	url := c.buildURL(endpoint)

	req := map[string]interface{}{
		"query": query,
	}
	if options != nil {
		if options.Variables != nil {
			req["variables"] = options.Variables
		}
		if options.OperationName != "" {
			req["operationName"] = options.OperationName
		}
	}

	var result ExecuteGraphqlResponse
	request := &internal.Request{
		Method: http.MethodPost,
		URL:    url,
		Body:   internal.NewJSONEntity(req),
	}
	_, err := c.client.DoAndUnmarshal(ctx, request, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) buildURL(endpoint string) string {
	if c.isEmulator {
		return fmt.Sprintf(dataConnectEmulatorURLFormat, c.emulatorHost, apiVersion, c.projectID, c.location, c.serviceID, endpoint)
	}
	return fmt.Sprintf(dataConnectProdURLFormat, apiVersion, c.projectID, c.location, c.serviceID, endpoint)
}

type graphqlQueryErrorResponse struct {
	Errors []map[string]interface{} `json:"errors"`
}

func handleError(resp *internal.Response) error {
	fe := internal.NewFirebaseError(resp)
	var errResp graphqlQueryErrorResponse
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && len(errResp.Errors) > 0 {
		// Unmarshalling here verifies query error exists
		fe.ErrorCode = queryError
	}
	return fe
}

// IsQueryError checks if the given error is a query error.
func IsQueryError(err error) bool {
	fe, ok := err.(*internal.FirebaseError)
	if !ok {
		return false
	}

	return fe.ErrorCode == queryError
}
