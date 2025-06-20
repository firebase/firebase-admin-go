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
	"os"
	"runtime"
	"strings"

	"firebase.google.com/go/v4/app" // Import app package
	"firebase.google.com/go/v4/internal"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const userAgentFormat = "Firebase/HTTP/%s/%s/AdminGo"
const invalidChars = "[].#$"
const authVarOverride = "auth_variable_override"
const emulatorDatabaseEnvVar = "FIREBASE_DATABASE_EMULATOR_HOST"
const emulatorNamespaceParam = "ns"

// errInvalidURL tells whether the given database url is invalid
// It is invalid if it is malformed, or not of the format "host:port"
var errInvalidURL = errors.New("invalid database url")

var emulatorToken = &oauth2.Token{
	AccessToken: "owner",
}

// Client is the interface for the Firebase Realtime Database service.
type Client struct {
	hc           *internal.HTTPClient
	dbURLConfig  *dbURLConfig
	authOverride string
}

type dbURLConfig struct {
	// BaseURL can be either:
	//	- a production url (https://foo-bar.firebaseio.com/)
	//	- an emulator url (http://localhost:9000)
	BaseURL string

	// Namespace is used in for the emulator to specify the databaseName
	// To specify a namespace on your url, pass ns=<database_name> (localhost:9000/?ns=foo-bar)
	Namespace string
}

// NewClient creates a new instance of the Firebase Database Client.
//
// It requires a context and a previously initialized *app.App instance.
// Optionally, a specific databaseURL can be provided as the first element of dbURL.
// If no dbURL is provided, the DatabaseURL from the app's configuration (app.Config.DatabaseURL)
// will be used. If neither is available, an error is returned.
//
// The *app.App provides the necessary configuration (like Project ID, credentials,
// and auth overrides) for the Database client.
func NewClient(ctx context.Context, appInstance *app.App, dbURL ...string) (*Client, error) {
	var targetURL string
	if len(dbURL) > 0 && dbURL[0] != "" {
		targetURL = dbURL[0]
	} else {
		targetURL = appInstance.DatabaseURL()
	}

	if targetURL == "" {
		return nil, errors.New("database URL must be specified in app config or as an argument")
	}

	urlConfig, isEmulator, err := parseURLConfig(targetURL)
	if err != nil {
		return nil, err
	}

	var ao []byte
	authOverride := appInstance.AuthOverride() // Get from app.App
	if authOverride == nil || len(authOverride) > 0 { // Logic for when to marshal remains similar
		ao, err = json.Marshal(authOverride)
		if err != nil {
			return nil, err
		}
	}

	// Use options from app.App
	clientOpts := append([]option.ClientOption{}, appInstance.Options()...)
	if isEmulator {
		ts := oauth2.StaticTokenSource(emulatorToken)
		clientOpts = append(clientOpts, option.WithTokenSource(ts))
	}
	// User-Agent now uses SDKVersion from app.App
	ua := fmt.Sprintf(userAgentFormat, appInstance.SDKVersion(), runtime.Version())
	clientOpts = append(clientOpts, option.WithUserAgent(ua))
	hc, _, err := internal.NewHTTPClient(ctx, clientOpts...)
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
// dbURL may be either:
//   - a production url (https://foo-bar.firebaseio.com/)
//   - an emulator URL (localhost:9000/?ns=foo-bar)
//
// The following rules will apply for determining the output:
//   - If the url does not use an https scheme it will be assumed to be an emulator url and be used.
//   - else If the FIREBASE_DATABASE_EMULATOR_HOST environment variable is set it will be used.
//   - else the url will be assumed to be a production url and be used.
func parseURLConfig(dbURL string) (*dbURLConfig, bool, error) {
	parsedURL, err := url.ParseRequestURI(dbURL)
	if err == nil && parsedURL.Scheme != "https" {
		cfg, err := parseEmulatorHost(dbURL, parsedURL)
		return cfg, true, err
	}

	environmentEmulatorURL := os.Getenv(emulatorDatabaseEnvVar)
	if environmentEmulatorURL != "" {
		parsedURL, err = url.ParseRequestURI(environmentEmulatorURL)
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", environmentEmulatorURL, errInvalidURL)
		}
		cfg, err := parseEmulatorHost(environmentEmulatorURL, parsedURL)
		return cfg, true, err
	}

	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", dbURL, errInvalidURL)
	}

	return &dbURLConfig{
		BaseURL:   dbURL,
		Namespace: "",
	}, false, nil
}

func parseEmulatorHost(rawEmulatorHostURL string, parsedEmulatorHost *url.URL) (*dbURLConfig, error) {
	if strings.Contains(rawEmulatorHostURL, "//") {
		return nil, fmt.Errorf(`invalid %s: "%s". It must follow format "host:port": %w`, emulatorDatabaseEnvVar, rawEmulatorHostURL, errInvalidURL)
	}

	baseURL := strings.Replace(rawEmulatorHostURL, fmt.Sprintf("?%s", parsedEmulatorHost.RawQuery), "", -1)
	if parsedEmulatorHost.Scheme != "http" {
		baseURL = fmt.Sprintf("http://%s", baseURL)
	}

	namespace := parsedEmulatorHost.Query().Get(emulatorNamespaceParam)
	if namespace == "" {
		if strings.Contains(rawEmulatorHostURL, ".") {
			namespace = strings.Split(rawEmulatorHostURL, ".")[0]
		}
		if namespace == "" {
			return nil, fmt.Errorf(`invalid database URL: "%s". Database URL must be a valid URL to a Firebase Realtime Database instance (include ?ns=<db-name> query param)`, parsedEmulatorHost)
		}
	}

	return &dbURLConfig{
		BaseURL:   baseURL,
		Namespace: namespace,
	}, nil
}
