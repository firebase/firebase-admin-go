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
	"net/http"
	"runtime"
	"strings"

	"firebase.google.com/go/internal"

	"net/url"

	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const userAgentFormat = "Firebase/HTTP/%s/%s/AdminGo"

// Client is the interface for the Firebase Realtime Database service.
type Client struct {
	hc  *http.Client
	url string
	ao  string
}

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

	return &Client{
		hc:  hc,
		url: fmt.Sprintf("https://%s", p.Host),
		ao:  string(ao),
	}, nil
}

type AuthOverrides struct {
	Map map[string]interface{}
}

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

func parsePath(path string) []string {
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}
