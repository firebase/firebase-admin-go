// +build go1.6

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
	"io/ioutil"
	"net/http"
	"net/url"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

func withContext(ctx context.Context, r *http.Request) *http.Request {
	if ctx == nil {
		panic("nil context")
	}
	r2 := new(http.Request)
	*r2 = *r
	r2.URL = &url.URL{}
	*(r2.URL) = *(r.URL)
	return r2
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
