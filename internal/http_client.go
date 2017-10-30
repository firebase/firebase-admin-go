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

// Package internal contains functionality that is only accessible from within the Admin SDK.
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

type Request struct {
	Method string
	URL    string
	Body   interface{}
	Opts   []HTTPOption
}

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
		b, err := json.Marshal(r.Body)
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

type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

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

func (r *Response) Unmarshal(want int, ep ErrorParser, v interface{}) error {
	if err := r.CheckStatus(want, ep); err != nil {
		return err
	} else if err := json.Unmarshal(r.Body, v); err != nil {
		return err
	}
	return nil
}

type ErrorParser func(r *Response) string

type HTTPOption func(*http.Request)

func WithHeader(key, value string) HTTPOption {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

func WithQueryParam(key, value string) HTTPOption {
	return func(r *http.Request) {
		q := r.URL.Query()
		q.Add(key, value)
		r.URL.RawQuery = q.Encode()
	}
}

func WithQueryParams(qp map[string]string) HTTPOption {
	return func(r *http.Request) {
		q := r.URL.Query()
		for k, v := range qp {
			q.Add(k, v)
		}
		r.URL.RawQuery = q.Encode()
	}
}
