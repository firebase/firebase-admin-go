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
	queryErrorCode               = "query-error"
)

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
		if !internal.HasSuccessStatus(r) {
			return false
		}
		var errResp graphqlErrorResponse
		if err := json.Unmarshal(r.Body, &errResp); err != nil {
			return true // Cannot parse, assume success
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

type graphqlError struct {
	Message string `json:"message"`
}

type graphqlErrorResponse struct {
	Errors []graphqlError `json:"errors"`
}

func handleError(resp *internal.Response) error {
	if resp.Status == 200 {
		var errResp graphqlErrorResponse
		// This will be called only when SuccessFn returns false, so we know there's an errors field.
		// We can ignore the unmarshal error here as it's handled in SuccessFn.
		json.Unmarshal(resp.Body, &errResp)

		var messages []string
		for _, e := range errResp.Errors {
			messages = append(messages, e.Message)
		}
		fe := internal.NewFirebaseError(resp)
		fe.ErrorCode = internal.InvalidArgument
		fe.String = fmt.Sprintf("GraphQL query failed: %s", strings.Join(messages, "; "))
		if fe.Ext == nil {
			fe.Ext = make(map[string]interface{})
		}
		fe.Ext["dataconnectErrorCode"] = queryErrorCode
		return fe
	}
	return internal.NewFirebaseError(resp)
}

// IsQueryError checks if the given error is a query error.
func IsQueryError(err error) bool {
	fe, ok := err.(*internal.FirebaseError)
	if !ok {
		return false
	}

	got, ok := fe.Ext["dataconnectErrorCode"]
	return ok && got == queryErrorCode
}
