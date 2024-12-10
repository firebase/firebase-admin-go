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

// Package for the clients to use Firebase Remote Config with Go.

package remoteconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"firebase.google.com/go/v4/internal"
)

const (
	defaulBaseUrl         = "https://firebaseremoteconfig.googleapis.com"
	firebaseClientHeader  = "X-Firebase-Client"
)

// Client is the interface for the Remote Config Cloud service.
type Client struct {
	*rcClient
}

// NewClient initializes a RemoteConfigClient with app-specific detail and a returns a 
// client to be used by the user.
func NewClient(ctx context.Context, c *internal.RemoteConfigClientConfig) (*Client, error) {
	if c.ProjectID == "" {
		return nil, errors.New("project ID is required to access Remote Conifg")
	}

	hc, _, err := internal.NewHTTPClient(ctx, c.Opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		rcClient: newRcClient(hc, c),
	},nil
}

// RemoteConfigClient facilitates requests to the Firebase Remote Config backend.
type rcClient struct {
	HttpClient	*internal.HTTPClient
	Project		string
	RcBaseUrl	string
	Version		string
}

func newRcClient(client *internal.HTTPClient, conf *internal.RemoteConfigClientConfig) *rcClient {
	goVersion := strings.TrimPrefix(runtime.Version(), "go")
	version := fmt.Sprintf("fire-admin-go/%s", conf.Version)
	client.Opts = []internal.HTTPOption{
		internal.WithHeader(firebaseClientHeader, version),
		internal.WithHeader("X-Firebase-ETag", "true"),
		internal.WithHeader("x-goog-api-client", fmt.Sprintf("gl-go/%s fire-admin/%s", goVersion, conf.Version)),
	}

	client.CreateErrFn = handleRemoteConfigError

	return &rcClient{
		RcBaseUrl: 	defaulBaseUrl,
		Project:   	conf.ProjectID,
		Version:   	version,
		HttpClient:	client,
	}
}

func (c *rcClient) GetServerTemplate(ctx context.Context, defaultConfig map[string]any) (*ServerTemplate, error) { 
	// Initialize a new ServerTemplate instance 
	template := c.InitServerTemplate(nil, defaultConfig) 
  
	// Load the template data from the server and cache it 
	err := template.Load(ctx);

	return template, err;
}
  
func (c *rcClient) InitServerTemplate(templateData *ServerTemplateData, defaultConfig map[string]any) *ServerTemplate { 
	// Create the ServerTemplate instance with defaultConfig
	template := NewServerTemplate(c, defaultConfig) 

	// Set template data if provided 
	if templateData != nil { 
		template.Set(templateData) 
	} 

	return template
}

func handleRemoteConfigError(resp *internal.Response) error {
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

