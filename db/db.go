// Copyright 2018 Google Inc. All Rights Reserved.
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

// Package db contains functions for accessing the Firebase Realtime Database.
package db

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"firebase.google.com/go/internal"

	"net/url"

	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const userAgentFormat = "Firebase/HTTP/%s/%s/AdminGo"
const invalidChars = "[].#$"
const authVarOverride = "auth_variable_override"

// Client is the interface for the Firebase Realtime Database service.
type Client struct {
	hc           *internal.HTTPClient
	url          string
	authOverride string
}

// NewClient creates a new instance of the Firebase Database Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// Database service through firebase.App.
func NewClient(ctx context.Context, c *internal.DatabaseConfig) (*Client, error) {
	opts := append([]option.ClientOption{}, c.Opts...)
	ua := fmt.Sprintf(userAgentFormat, c.Version, runtime.Version())
	opts = append(opts, option.WithUserAgent(ua))
	hc, _, err := transport.NewHTTPClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	p, err := url.ParseRequestURI(c.URL)
	if err != nil {
		return nil, err
	} else if p.Scheme != "https" {
		return nil, fmt.Errorf("invalid database URL: %q; want scheme: %q", c.URL, "https")
	} else if !strings.HasSuffix(p.Host, ".firebaseio.com") {
		return nil, fmt.Errorf("invalid database URL: %q; want host: %q", c.URL, "firebaseio.com")
	}

	var ao []byte
	if c.AuthOverride == nil || len(c.AuthOverride) > 0 {
		ao, err = json.Marshal(c.AuthOverride)
		if err != nil {
			return nil, err
		}
	}

	ep := func(b []byte) string {
		var p struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(b, &p); err != nil {
			return ""
		}
		return p.Error
	}
	return &Client{
		hc:           &internal.HTTPClient{Client: hc, ErrParser: ep},
		url:          fmt.Sprintf("https://%s", p.Host),
		authOverride: string(ao),
	}, nil
}

// NewRef returns a new database reference representing the node at the specified path.
func (c *Client) NewRef(path string) *Ref {
	segs := parsePath(path)
	key := ""
	if len(segs) > 0 {
		key = segs[len(segs)-1]
	}

	return &Ref{
		Key:    key,
		Path:   "/" + strings.Join(segs, "/"),
		client: c,
		segs:   segs,
	}
}

func (c *Client) send(
	ctx context.Context,
	method, path string,
	body internal.HTTPEntity,
	opts ...internal.HTTPOption) (*internal.Response, error) {

	if strings.ContainsAny(path, invalidChars) {
		return nil, fmt.Errorf("invalid path with illegal characters: %q", path)
	}
	if c.authOverride != "" {
		opts = append(opts, internal.WithQueryParam(authVarOverride, c.authOverride))
	}
	return c.hc.Do(ctx, &internal.Request{
		Method: method,
		URL:    fmt.Sprintf("%s%s.json", c.url, path),
		Body:   body,
		Opts:   opts,
	})
}

func parsePath(path string) []string {
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}
