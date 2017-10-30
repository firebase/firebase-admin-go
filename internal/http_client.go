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
	"net/http"
)

// Null represents JSON null value.
var Null struct{} = jsonNull{}

type jsonNull struct{}

// Request contains all the parameters required to construct an outgoing HTTP request.
type Request struct {
	Method string
	URL    string
	Body   interface{}
	Opts   []HTTPOption
}

// Send executes the current Request using the given context and HTTP client.
//
// If the Body is not nil, it is serialized into a JSON string. To send JSON null as the body, use
// the internal.Null variable.
func (r *Request) Send(ctx context.Context, hc *http.Client) (*Response, error) {
	req, err := r.newHTTPRequest()
	if err != nil {
		return nil, err
	}

	resp, err := hc.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{
		Status: resp.StatusCode,
		Body:   b,
		Header: resp.Header,
	}, nil
}

func (r *Request) newHTTPRequest() (*http.Request, error) {
	var opts []HTTPOption
	var data io.Reader
	if r.Body != nil {
		var body interface{}
		if r.Body == Null {
			body = nil
		} else {
			body = r.Body
		}
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		data = bytes.NewBuffer(b)
		opts = append(opts, WithHeader("Content-Type", "application/json"))
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

// Response contains information extracted from an HTTP response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// CheckStatus checks whether the Response status code has the given HTTP status code.
//
// Returns an error if the status code does not match. If an ErroParser is specified, uses that to
// construct the returned error message. Otherwise includes the full response body in the error.
func (r *Response) CheckStatus(want int, ep ErrorParser) error {
	if r.Status == want {
		return nil
	}

	var msg string
	if ep != nil {
		msg = ep(r)
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
func (r *Response) Unmarshal(want int, ep ErrorParser, v interface{}) error {
	if err := r.CheckStatus(want, ep); err != nil {
		return err
	} else if err := json.Unmarshal(r.Body, v); err != nil {
		return err
	}
	return nil
}

// ErrorParser is a function that is used to construct custom error messages.
type ErrorParser func(r *Response) string

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
