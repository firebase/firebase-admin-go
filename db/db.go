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
	"errors"
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
const emulatorHostEnvVar = "FIREBASE_DATABASE_EMULATOR_HOST"
const emulatorNamespaceParam = "ns"

var ErrInvalidURL error = errors.New("invalid database url")

/** TODO:
 * Using https://github.com/firebase/firebase-admin-python/pull/313/files as the example
 * Validate // is not in the url
 * Include ?ns={namespace} query parameter
 * Use emulator admin credentials
 * Fill in parseURLConfig
 * Make a set of credentials that correspond to the emulator
 * Additional tests for parsing the url
 * Ensure the parameters are passed on the url
 */

// Client is the interface for the Firebase Realtime Database service.
type Client struct {
	hc           *internal.HTTPClient
	dbURLConfig  *dbURLConfig
	authOverride string
}

type dbURLConfig struct {
	// BaseURL can be either:
	//	- a production url (https://foo-bar.firebaseio.com/)
	//	- an emulator url (http://localhost:8080)
	BaseURL string

	// Namespace is used in for the emulator to specify the databaseName
	// To specify a namespace on your url, pass ns=<database_name> (localhost:8080/?ns=foo-bar)
	Namespace string
}

// NewClient creates a new instance of the Firebase Database Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// Database service through firebase.App.
func NewClient(ctx context.Context, c *internal.DatabaseConfig) (*Client, error) {
	urlConfig, err := parseURLConfig(c.URL)
	if err != nil {
		return nil, err
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
		dbURLConfig:  urlConfig,
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

	req.URL = fmt.Sprintf("%s%s.json", c.dbURLConfig.BaseURL, req.URL)
	if c.authOverride != "" {
		req.Opts = append(req.Opts, internal.WithQueryParam(authVarOverride, c.authOverride))
	}
	if c.dbURLConfig.Namespace != "" {
		req.Opts = append(req.Opts, internal.WithQueryParam(emulatorNamespaceParam, c.dbURLConfig.Namespace))
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

// parseURLConfig returns the dbURLConfig for the database
// dbURL may be a production url (https://foo-bar.firebaseio.com/) or an emulator URL (localhost:8080/?ns=foo-bar)
// The following rules will apply for determining the output:
//   - If the url has no scheme it will be assumed to be an emulator url and be used.
//   - else If the FIREBASE_DATABASE_EMULATOR_HOST environment variable is set it will be used.
//   - else the url will be assumed to be a production url and be used.
func parseURLConfig(dbURL string) (*dbURLConfig, error) {
	parsedURL, err := url.ParseRequestURI(dbURL)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dbURL, ErrInvalidURL)
	}
	if parsedURL.Scheme != "https" {
		return parseEmulatorHost(dbURL, parsedURL)
	}
	return &dbURLConfig{
		BaseURL:   dbURL,
		Namespace: "",
	}, nil
}

func parseEmulatorHost(rawEmulatorHostURL string, parsedEmulatorHost *url.URL) (*dbURLConfig, error) {
	if strings.Contains(rawEmulatorHostURL, "//") {
		return nil, fmt.Errorf(`invalid %s: "%s". It must follow format "host:port": %w`, emulatorHostEnvVar, rawEmulatorHostURL, ErrInvalidURL)
	}

	baseURL := parsedEmulatorHost.Host
	if parsedEmulatorHost.Scheme != "http" {
		baseURL = "http://" + baseURL
	}

	namespace := parsedEmulatorHost.Query().Get(emulatorNamespaceParam)
	if namespace == "" {
		return nil, fmt.Errorf(`invalid database URL: "%s". Database URL must be a valid URL to a Firebase Realtime Database instance (include ?ns=<db-name> query param)`, parsedEmulatorHost)
	}

	return &dbURLConfig{
		BaseURL:   baseURL,
		Namespace: namespace,
	}, nil
}

/**
"""Parses emulator URL like http://localhost:8080/?ns=foo-bar"""
        query_ns = parse.parse_qs(parsed_url.query).get('ns')
        if parsed_url.scheme != 'http' or (not query_ns or len(query_ns) != 1 or not query_ns[0]):
            raise ValueError(
                'Invalid database URL: "{0}". Database URL must be a valid URL to a '
                'Firebase Realtime Database instance.'.format(parsed_url.geturl()))

        namespace = query_ns[0]
        base_url = '{0}://{1}'.format(parsed_url.scheme, parsed_url.netloc)
        return base_url, namespace
*/
