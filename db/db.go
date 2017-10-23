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

const invalidChars = "[].#$"
const userAgent = "Firebase/HTTP/%s/%s/AdminGo"

// Client is the interface for the Firebase Realtime Database service.
type Client struct {
	hc      *http.Client
	baseURL string
}

func NewClient(ctx context.Context, c *internal.DatabaseConfig) (*Client, error) {
	userAgent := fmt.Sprintf(userAgent, c.Version, runtime.Version())
	o := []option.ClientOption{option.WithUserAgent(userAgent)}
	o = append(o, c.Opts...)

	hc, _, err := transport.NewHTTPClient(ctx, o...)
	if err != nil {
		return nil, err
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("database url not specified")
	}
	url, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	} else if url.Scheme != "https" {
		return nil, fmt.Errorf("invalid database URL (incorrect scheme): %q", c.BaseURL)
	} else if !strings.HasSuffix(url.Host, ".firebaseio.com") {
		return nil, fmt.Errorf("invalid database URL (incorrest host): %q", c.BaseURL)
	}
	return &Client{
		hc:      hc,
		baseURL: fmt.Sprintf("https://%s", url.Host),
	}, nil
}

func (c *Client) NewRef(path string) (*Ref, error) {
	segs, err := parsePath(path)
	if err != nil {
		return nil, err
	}

	key := ""
	if len(segs) > 0 {
		key = segs[len(segs)-1]
	}

	return &Ref{
		Key:    key,
		Path:   "/" + strings.Join(segs, "/"),
		client: c,
		segs:   segs,
	}, nil
}

func parsePath(path string) ([]string, error) {
	if strings.ContainsAny(path, invalidChars) {
		return nil, fmt.Errorf("path %q contains one or more invalid characters", path)
	}
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs, nil
}
