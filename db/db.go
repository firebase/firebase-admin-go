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
	hc  *internal.HTTPClient
	url string
	ao  string
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
		return nil, fmt.Errorf("invalid database URL (incorrect scheme): %q", c.URL)
	} else if !strings.HasSuffix(p.Host, ".firebaseio.com") {
		return nil, fmt.Errorf("invalid database URL (incorrest host): %q", c.URL)
	}

	var ao []byte
	if c.AO == nil || len(c.AO) > 0 {
		ao, err = json.Marshal(c.AO)
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
		hc:  &internal.HTTPClient{Client: hc, ErrParser: ep},
		url: fmt.Sprintf("https://%s", p.Host),
		ao:  string(ao),
	}, nil
}

// AuthOverride regulates how Firebase security rules are enforced on database invocations.
//
// By default, the database calls made by the Admin SDK have administrative privileges, thereby
// allowing them to completely bypass all Firebase security rules. This behavior can be overridden
// by setting an AuthOverride. When specified, the AuthOverride value will become visible to the
// database server during security rule evaluation. Specifically, this value will be accessible
// via the auth variable of the security rules.
//
// Refer to https://firebase.google.com/docs/database/admin/start#authenticate-with-limited-privileges
// for more details and code samples.
type AuthOverride struct {
	Map map[string]interface{}
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

func (c *Client) send(ctx context.Context, r *dbReq) (*internal.Response, error) {
	req, err := r.NewHTTPRequest(c)
	if err != nil {
		return nil, err
	}
	return c.hc.Do(ctx, req)
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

type dbReq struct {
	Method, Path string
	Body         interface{}
	Opts         []internal.HTTPOption
}

func (r *dbReq) NewHTTPRequest(c *Client) (*internal.Request, error) {
	if strings.ContainsAny(r.Path, invalidChars) {
		return nil, fmt.Errorf("invalid path with illegal characters: %q", r.Path)
	}

	var opts []internal.HTTPOption
	if c.ao != "" {
		opts = append(opts, internal.WithQueryParam(authVarOverride, c.ao))
	}
	var b internal.HTTPEntity
	if r.Body != nil {
		b = internal.NewJSONEntity(r.Body)
	}
	return &internal.Request{
		Method: r.Method,
		URL:    fmt.Sprintf("%s%s.json", c.url, r.Path),
		Body:   b,
		Opts:   append(opts, r.Opts...),
	}, nil
}
