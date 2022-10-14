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
	"os"
	"runtime"
	"strings"

	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
)

const userAgentFormat = "Firebase/HTTP/%s/%s/AdminGo"
const invalidChars = "[].#$"
const authVarOverride = "auth_variable_override"
const emulatorHostEnvVar = "FIREBASE_DATABASE_EMULATOR_HOST"

/** TODO:
 * Using https://github.com/firebase/firebase-admin-python/pull/313/files as the example
 * Validate // is not in the url
 * Include ?ns={namespace} query parameter
 * Use emulator admin credentials
 * Fill in parseDatabaseURL
 * Make a set of credentials that correspond to the emulator
 * Additional tests for parsing the url
 * Ensure the parameters are passed on the url
 */

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
	var databaseURL string
	emulatorHost := os.Getenv(emulatorHostEnvVar)
	if emulatorHost == "" {
		p, err := url.ParseRequestURI(c.URL)
		if err != nil {
			return nil, err
		} else if p.Scheme != "https" {
			return nil, fmt.Errorf("invalid database URL: %q; want scheme: %q", c.URL, "https")
		}
		databaseURL = fmt.Sprintf("https://%s", p.Host)
	} else {
		databaseURL = emulatorHost
	}

	var ao []byte
	var err error
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
		url:          databaseURL,
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

// parseDatabaseURL returns the baseURL for the database
// The input can be either be:
// - a production URL (https://foo-bar.firebaseio.com/)
// - an Emulator URL (http://localhost:8080/?ns=foo-bar)
// In case of Emulator URL, the caller should ensure the namespace is extracted from the query param ns.
// The resulting base_url never includes query params.
// If url is a production URL and emulator_host is specified, the resulting base URL will use the emulator_host
//
//	emulator_host is ignored if url is already an emulator URL.
func parseDatabaseURL() string {
	return ""
}
