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
	"net/http"
	"runtime"
	"strings"

	"firebase.google.com/go/v4/internal"
)

const (
	defaulBaseUrl			= "https://firebaseremoteconfig.googleapis.com"
	firebaseClientHeader   	= "X-Firebase-Client"
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
	HttpClient   *internal.HTTPClient
	Project    string
	RcBaseUrl    string
	Version	  	 string
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

func (c *rcClient) GetServerTemplate(ctx context.Context) (*ServerTemplate, error) { 
	// Initialize a new ServerTemplate instance 
	template := c.InitServerTemplate(nil) 
  
	// Load the template data from the server and cache it 
	err := template.Load(ctx);

	return template, err;
  }
  
func (c *rcClient) InitServerTemplate(templateData *ServerTemplateData) *ServerTemplate { 
	// Create the ServerTemplate instance with defaultConfig
	template := NewServerTemplate(c) 

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

type ServerTemplateData struct {
	Conditions		interface{}		`json:"conditions"`
	Parameters 		map[string]RemoteConfigParameter	`json:"parameters"`

	Version struct {
		VersionNumber 	string 	`json:"versionNumber"`
		IsLegacy      	bool   	`json:"isLegacy"`
	} 							`json:"version"`

	ETag	string
}

type RemoteConfigParameter struct {
	DefaultValue      RemoteConfigParameterValue 			`json:"defaultValue"`
	ConditionalValues map[string]RemoteConfigParameterValue `json:"conditionalValues"`
}

type RemoteConfigParameterValue interface{}

// ServerTemplate represents a template with configuration data, cache, and service information.
type ServerTemplate struct {
	RcClient	*rcClient
	Cache       *ServerTemplateData
}

// NewServerTemplate initializes a new ServerTemplate with optional default configuration.
func NewServerTemplate(rcClient *rcClient) *ServerTemplate {
	return &ServerTemplate{
		RcClient: rcClient,
		Cache:	  nil,
	}
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Load(ctx context.Context) error {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/v1/projects/%s/namespaces/firebase-server/serverRemoteConfig", s.RcClient.RcBaseUrl , s.RcClient.Project),
	}

	var templateData ServerTemplateData
	response, err := s.RcClient.HttpClient.DoAndUnmarshal(ctx, request, &templateData)

	if err != nil {
		return err
	}

	templateData.ETag = response.Header.Get("etag")
	s.Cache = &templateData
	fmt.Println("Etag", s.Cache.ETag) // TODO: Remove ETag 
	return nil
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Set(templateData *ServerTemplateData) {
	s.Cache = templateData 
}

// Evaluate processes the cached template data with a condition evaluator 
// based on the provided context.
func (s *ServerTemplate) Evaluate(context map[string]interface{}) *ServerConfig {
	// TODO: Write ConditionalEvaluator for evaluating
	return &ServerConfig{ConfigValues: s.Cache.Parameters}
}

type ServerConfig struct {
	ConfigValues map[string]RemoteConfigParameter
}

// GetValue returns the raw value associated with a key in the configuration.
func (sc *ServerConfig) GetValue(key string) interface{} {
	return sc.ConfigValues[key].DefaultValue
}
