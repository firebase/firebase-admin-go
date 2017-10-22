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
	"io"
	"net/http"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/internal"

	"net/url"

	"io/ioutil"

	"encoding/json"

	"runtime"

	"bytes"

	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const invalidChars = "[].#$"

var userAgent = fmt.Sprintf("Firebase/HTTP/%s/%s/AdminGo", firebase.Version, runtime.Version())

// Client is the interface for the Firebase Realtime Database service.
type Client struct {
	hc      *http.Client
	baseURL string
}

func NewClient(ctx context.Context, c *internal.DatabaseConfig) (*Client, error) {
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
	if strings.ContainsAny(path, invalidChars) {
		return nil, fmt.Errorf("path %q contains one or more invalid characters", path)
	}
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
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

func (c *Client) send(r *request) (*response, error) {
	url := fmt.Sprintf("%s%s%s", c.baseURL, r.Path, ".json")

	var data io.Reader
	if r.Body != nil {
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, err
		}
		data = bytes.NewBuffer(b)
	}

	req, err := http.NewRequest(r.Method, url, data)
	if err != nil {
		return nil, err
	}
	if data != nil {
		req.Header.Add("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &response{Status: resp.StatusCode, Body: b}, nil
}

type request struct {
	Method string
	Path   string
	Body   interface{}
}

type response struct {
	Status int
	Body   []byte
}

func (r *response) CheckStatus(want int) error {
	if r.Status != want {
		return fmt.Errorf("http error: %d; body: %s", r.Status, string(r.Body))
	}
	return nil
}

func (r *response) CheckAndParse(want int, v interface{}) error {
	if err := r.CheckStatus(want); err != nil {
		return err
	} else if err := json.Unmarshal(r.Body, v); err != nil {
		return err
	}
	return nil
}

type Ref struct {
	Key  string
	Path string

	client *Client
	segs   []string
}

func (r *Ref) Parent() *Ref {
	l := len(r.segs)
	if l > 0 {
		path := strings.Join(r.segs[:l-1], "/")
		parent, _ := r.client.NewRef(path)
		return parent
	}
	return nil
}

func (r *Ref) Get(v interface{}) error {
	resp, err := r.client.send(&request{Method: "GET", Path: r.Path})
	if err != nil {
		return err
	} else if err := resp.CheckAndParse(http.StatusOK, v); err != nil {
		return err
	}
	return nil
}

func (r *Ref) Set(v interface{}) error {
	resp, err := r.client.send(&request{Method: "PUT", Path: r.Path, Body: v})
	if err != nil {
		return err
	} else if err := resp.CheckStatus(http.StatusOK); err != nil {
		return err
	}
	return nil
}
