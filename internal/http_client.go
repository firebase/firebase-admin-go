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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context/ctxhttp"

	"golang.org/x/net/context"
)

// HTTPClient is a convenient API to make HTTP calls.
//
// This API handles some of the repetitive tasks such as entity serialization and deserialization
// involved in making HTTP calls. It provides a convenient mechanism to set headers and query
// parameters on outgoing requests, while enforcing that an explicit context is used per request.
// Responses returned by HTTPClient can be easily parsed as JSON, and provide a simple mechanism to
// extract error details.
type HTTPClient struct {
	Client    *http.Client
	ErrParser ErrorParser
}

// Do executes the given Request, and returns a Response.
func (c *HTTPClient) Do(ctx context.Context, r *Request) (*Response, error) {
	req, err := r.buildHTTPRequest()
	if err != nil {
		return nil, err
	}

	resp, err := ctxhttp.Do(ctx, c.Client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{
		Status:    resp.StatusCode,
		Body:      b,
		Header:    resp.Header,
		errParser: c.ErrParser,
	}, nil
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

// CheckStatus checks whether the Response status code has the given HTTP status code.
//
// Returns an error if the status code does not match. If an ErroParser is specified, uses that to
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
