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
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"runtime"
	"strings"

	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
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

	opts := append([]option.ClientOption{}, c.Opts...)
	ua := fmt.Sprintf(userAgentFormat, c.Version, runtime.Version())
	opts = append(opts, option.WithUserAgent(ua))
	hc, _, err := internal.NewHTTPClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	hc.CreateErrFn = handleRTDBError
	return &Client{
		hc:           hc,
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

func (c *Client) sendAndUnmarshal(
	ctx context.Context, req *internal.Request, v interface{}) (*internal.Response, error) {
	if strings.ContainsAny(req.URL, invalidChars) {
		return nil, fmt.Errorf("invalid path with illegal characters: %q", req.URL)
	}

	req.URL = fmt.Sprintf("%s%s.json", c.url, req.URL)
	if c.authOverride != "" {
		req.Opts = append(req.Opts, internal.WithQueryParam(authVarOverride, c.authOverride))
	}

	return c.hc.DoAndUnmarshal(ctx, req, v)
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

func handleRTDBError(resp *internal.Response) error {
	err := internal.NewFirebaseError(resp)
	var p struct {
		Error string `json:"error"`
	}
	json.Unmarshal(resp.Body, &p)
	if p.Error != "" {
		err.String = fmt.Sprintf("http error status: %d; reason: %s", resp.Status, p.Error)
	}

	return err
}
