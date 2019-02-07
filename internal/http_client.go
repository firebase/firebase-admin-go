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

package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

// HTTPClient is a convenient API to make HTTP calls.
//
// This API handles repetitive tasks such as entity serialization and deserialization
// when making HTTP calls. It provides a convenient mechanism to set headers and query
// parameters on outgoing requests, while enforcing that an explicit context is used per request.
// Responses returned by HTTPClient can be easily unmarshalled as JSON.
//
// HTTPClient also handles automatically retrying failed HTTP requests.
type HTTPClient struct {
	Client      *http.Client
	RetryConfig *RetryConfig
	ErrParser   ErrorParser
}

// NewHTTPClient creates a new HTTPClient using the provided client options and the default
// RetryConfig.
//
// The default RetryConfig retries requests on all low-level network errors as well as on HTTP
// InternalServerError (500) and ServiceUnavailable (503) errors. Repeatedly failing requests are
// retried up to 4 times with exponential backoff. Retry delay is never longer than 2 minutes.
//
// NewHTTPClient returns the created HTTPClient along with the target endpoint URL. The endpoint
// is obtained from the client options passed into the function.
func NewHTTPClient(ctx context.Context, opts ...option.ClientOption) (*HTTPClient, string, error) {
	hc, endpoint, err := transport.NewHTTPClient(ctx, opts...)
	if err != nil {
		return nil, "", err
	}
	twoMinutes := time.Duration(2) * time.Minute
	client := &HTTPClient{
		Client: hc,
		RetryConfig: &RetryConfig{
			MaxRetries: 4,
			CheckForRetry: retryNetworkAndHTTPErrors(
				http.StatusInternalServerError,
				http.StatusServiceUnavailable,
			),
			ExpBackoffFactor: 0.5,
			MaxDelay:         &twoMinutes,
		},
	}
	return client, endpoint, nil
}

// Do executes the given Request, and returns a Response.
//
// If a RetryConfig is specified on the client, Do attempts to retry failing requests.
func (c *HTTPClient) Do(ctx context.Context, req *Request) (*Response, error) {
	var result *attemptResult
	var err error

	for retries := 0; ; retries++ {
		result, err = c.attempt(ctx, req, retries)
		if err != nil {
			return nil, err
		}
		if !result.Retry {
			break
		}
		if err = result.waitForRetry(ctx); err != nil {
			return nil, err
		}
	}
	return result.handleResponse()
}

func (c *HTTPClient) attempt(ctx context.Context, req *Request, retries int) (*attemptResult, error) {
	hr, err := req.buildHTTPRequest()
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(hr.WithContext(ctx))
	result := &attemptResult{
		Resp:      resp,
		Err:       err,
		ErrParser: c.ErrParser,
	}

	// If a RetryConfig is available, always consult it to determine if the request should be retried
	// or not. Even if there was a network error, we may not want to retry the request based on the
	// RetryConfig that is in effect.
	if c.RetryConfig != nil {
		delay, retry := c.RetryConfig.retryDelay(retries, resp, err)
		result.RetryAfter = delay
		result.Retry = retry
		if retry && resp != nil {
			defer resp.Body.Close()
		}
	}
	return result, nil
}

type attemptResult struct {
	Resp       *http.Response
	Err        error
	Retry      bool
	RetryAfter time.Duration
	ErrParser  ErrorParser
}

func (r *attemptResult) waitForRetry(ctx context.Context) error {
	if r.RetryAfter > 0 {
		select {
		case <-ctx.Done():
		case <-time.After(r.RetryAfter):
		}
	}
	return ctx.Err()
}

func (r *attemptResult) handleResponse() (*Response, error) {
	if r.Err != nil {
		return nil, r.Err
	}
	return newResponse(r.Resp, r.ErrParser)
}

// Request contains all the parameters required to construct an outgoing HTTP request.
type Request struct {
	Method string
	URL    string
	Body   HTTPEntity
	Opts   []HTTPOption
}

func (r *Request) buildHTTPRequest() (*http.Request, error) {
	var opts []HTTPOption
	var data io.Reader
	if r.Body != nil {
		b, err := r.Body.Bytes()
		if err != nil {
			return nil, err
		}
		data = bytes.NewBuffer(b)
		opts = append(opts, WithHeader("Content-Type", r.Body.Mime()))
	}

	req, err := http.NewRequest(r.Method, r.URL, data)
	if err != nil {
		return nil, err
	}

	opts = append(opts, r.Opts...)
	for _, o := range opts {
		o(req)
	}
	return req, nil
}

// HTTPEntity represents a payload that can be included in an outgoing HTTP request.
type HTTPEntity interface {
	Bytes() ([]byte, error)
	Mime() string
}

type jsonEntity struct {
	Val interface{}
}

// NewJSONEntity creates a new HTTPEntity that will be serialized into JSON.
func NewJSONEntity(v interface{}) HTTPEntity {
	return &jsonEntity{Val: v}
}

func (e *jsonEntity) Bytes() ([]byte, error) {
	return json.Marshal(e.Val)
}

func (e *jsonEntity) Mime() string {
	return "application/json"
}

// Response contains information extracted from an HTTP response.
type Response struct {
	Status    int
	Header    http.Header
	Body      []byte
	errParser ErrorParser
}

func newResponse(resp *http.Response, errParser ErrorParser) (*Response, error) {
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{
		Status:    resp.StatusCode,
		Body:      b,
		Header:    resp.Header,
		errParser: errParser,
	}, nil
}

// CheckStatus checks whether the Response status code has the given HTTP status code.
//
// Returns an error if the status code does not match. If an ErrorParser is specified, uses that to
// construct the returned error message. Otherwise includes the full response body in the error.
func (r *Response) CheckStatus(want int) error {
	if r.Status == want {
		return nil
	}

	var msg string
	if r.errParser != nil {
		msg = r.errParser(r.Body)
	}
	if msg == "" {
		msg = string(r.Body)
	}
	return fmt.Errorf("http error status: %d; reason: %s", r.Status, msg)
}

// Unmarshal checks if the Response has the given HTTP status code, and if so unmarshals the
// response body into the variable pointed by v.
//
// Unmarshal uses https://golang.org/pkg/encoding/json/#Unmarshal internally, and hence v has the
// same requirements as the json package.
func (r *Response) Unmarshal(want int, v interface{}) error {
	if err := r.CheckStatus(want); err != nil {
		return err
	}
	return json.Unmarshal(r.Body, v)
}

// ErrorParser is a function that is used to construct custom error messages.
type ErrorParser func([]byte) string

// HTTPOption is an additional parameter that can be specified to customize an outgoing request.
type HTTPOption func(*http.Request)

// WithHeader creates an HTTPOption that will set an HTTP header on the request.
func WithHeader(key, value string) HTTPOption {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

// WithQueryParam creates an HTTPOption that will set a query parameter on the request.
func WithQueryParam(key, value string) HTTPOption {
	return func(r *http.Request) {
		q := r.URL.Query()
		q.Add(key, value)
		r.URL.RawQuery = q.Encode()
	}
}

// WithQueryParams creates an HTTPOption that will set all the entries of qp as query parameters
// on the request.
func WithQueryParams(qp map[string]string) HTTPOption {
	return func(r *http.Request) {
		q := r.URL.Query()
		for k, v := range qp {
			q.Add(k, v)
		}
		r.URL.RawQuery = q.Encode()
	}
}

// RetryConfig specifies how the HTTPClient should retry failing HTTP requests.
//
// A request is never retried more than MaxRetries times. If CheckForRetry is nil, all network
// errors, and all 400+ HTTP status codes are retried. If an HTTP error response contains the
// Retry-After header, it is always respected. Otherwise retries are delayed with exponential
// backoff. Set ExpBackoffFactor to 0 to disable exponential backoff, and retry immediately
// after each error.
//
// If MaxDelay is set, retries delay gets capped by that value. If the Retry-After header
// requires a longer delay than MaxDelay, retries are not attempted.
type RetryConfig struct {
	MaxRetries       int
	CheckForRetry    RetryCondition
	ExpBackoffFactor float64
	MaxDelay         *time.Duration
}

// RetryCondition determines if an HTTP request should be retried depending on its last outcome.
type RetryCondition func(resp *http.Response, networkErr error) bool

func (rc *RetryConfig) retryDelay(retries int, resp *http.Response, err error) (time.Duration, bool) {
	if !rc.retryEligible(retries, resp, err) {
		return 0, false
	}
	estimatedDelay := rc.estimateDelayBeforeNextRetry(retries)
	serverRecommendedDelay := parseRetryAfterHeader(resp)
	if serverRecommendedDelay > estimatedDelay {
		estimatedDelay = serverRecommendedDelay
	}
	if rc.MaxDelay != nil && estimatedDelay > *rc.MaxDelay {
		return 0, false
	}
	return estimatedDelay, true
}

func (rc *RetryConfig) retryEligible(retries int, resp *http.Response, err error) bool {
	if retries >= rc.MaxRetries {
		return false
	}
	if rc.CheckForRetry == nil {
		return err != nil || resp.StatusCode >= 500
	}
	return rc.CheckForRetry(resp, err)
}

func (rc *RetryConfig) estimateDelayBeforeNextRetry(retries int) time.Duration {
	if retries == 0 {
		return 0
	}
	delayInSeconds := int64(math.Pow(2, float64(retries)) * rc.ExpBackoffFactor)
	estimatedDelay := time.Duration(delayInSeconds) * time.Second
	if rc.MaxDelay != nil && estimatedDelay > *rc.MaxDelay {
		estimatedDelay = *rc.MaxDelay
	}
	return estimatedDelay
}

var retryTimeClock Clock = SystemClock

func parseRetryAfterHeader(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	retryAfterHeader := resp.Header.Get("retry-after")
	if retryAfterHeader == "" {
		return 0
	}
	if delayInSeconds, err := strconv.ParseInt(retryAfterHeader, 10, 64); err == nil {
		return time.Duration(delayInSeconds) * time.Second
	}
	if timestamp, err := http.ParseTime(retryAfterHeader); err == nil {
		return timestamp.Sub(retryTimeClock.Now())
	}
	return 0
}

func retryNetworkAndHTTPErrors(statusCodes ...int) RetryCondition {
	return func(resp *http.Response, networkErr error) bool {
		if networkErr != nil {
			return true
		}
		for _, retryOnStatus := range statusCodes {
			if resp.StatusCode == retryOnStatus {
				return true
			}
		}
		return false
	}
}
