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
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/transport"
)

const (
	defaulBaseUrl = "https://firebaseremoteconfig.googleapis.com'"
	firebaseClientHeader   = "X-Firebase-Client"
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
		rcClient: newRcClient(hc, c)
    },nil
}

// RemoteConfigClient facilitates requests to the Firebase Remote Config backend.
type rcClient struct {
    httpClient   *internal.HTTPClient
    projectId    string
    rcBaseUrl    string
	version:	 string
}

func newRcClient(hc *http.Client, conf *internal.RemoteConfigClientConfig) *rcClient {
	client := internal.WithDefaultRetryConfig(hc)

	goVersion := strings.TrimPrefix(runtime.Version(), "go")
	version := fmt.Sprintf("fire-admin-go/%s", conf.Version)
	client.Opts = []internal.HTTPOption{
		internal.WithHeader(firebaseClientHeader, version),
		internal.WithHeader("X-Firebase-ETag", "true"),
		internal.WithHeader("x-goog-api-client", fmt.Sprintf("gl-go/%s fire-admin/%s", goVersion, conf.Version)),
	}

	hc.CreateErrFn = handleRemoteConfigError

	return &rcClient{
		rcBaseUrl: 	defaulBaseUrl,
		project:   	conf.ProjectID,
		version:   	version,
		httpClient:	client,
	}
}

func (c *rcClient) GetServerTemplate(ctx: context.Context, defaultConfig map[string]string) (*ServerTemplate, error) { 
	// Initialize a new ServerTemplate instance 
	template := c.InitServerTemplate(defaultConfig, nil) 
  
	// Load the template data from the server and cache it 
	_, err := template.Load(ctx);

	return template, err;
  }
  
func (c *rcClient) InitServerTemplate(ctx: context.Context, defaultConfig map[string]string, 
	templateData *ServerTemplateData) *ServerTemplate { 
	// Create the ServerTemplate instance with defaultConfig
	if (defaultConfig != nil) { 
		template := NewServerTemplate(defaultConfig) 
	}
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
    Conditions map[string]interface{} `json:"conditions"`
    Parameters map[string]RemoteConfigParameter `json:"parameters"`
	Version struct {
		VersionNumber string `json:"versionNumber"`
		IsLegacy      bool   `json:"isLegacy"`
	} `json:"version"`
	ETag       string
}

type RemoteConfigParameter struct {
    DefaultValue      RemoteConfigParameterValue `json:"defaultValue"`
    ConditionalValues map[string]RemoteConfigParameterValue `json:"conditionalValues"`
}

type RemoteConfigParameterValue interface{}

// ServerTemplate represents a template with configuration data, cache, and service information.
type ServerTemplate struct {
    rcClient	*rcClient
    cache       *ServerTemplateData
}

// NewServerTemplateData initializes a ServerTemplateData instance with template data and an ETag.
func NewServerTemplateData(templateResponse *internal.Response) *ServerTemplateData {
    return &ServerTemplateData{
		Conditions:	templateData["conditions"].([]NamedCondition),
		Parameters:	templateData["parameters"].(map[string]RemoteConfigParameter),
        Version:	templateData["version"].(string),
        ETag:		templateData["etag"].(string)
    }
}

// NewServerTemplate initializes a new ServerTemplate with optional default configuration.
func NewServerTemplate(rcClient *rcClient) *ServerTemplate {
    return &ServerTemplate{
		rcClient: rcClient,
        cache:	  nil
    }
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Load(ctx context.Context) error {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/v1/projects/%s/namespaces/firebase-server/serverRemoteConfig", c.rcBaseUrl , c.project),
		Body:   internal.NewJSONEntity(req),
	}

	var templateData ServerTemplateData
    response, err := s.rcClient.httpClient.Do(ctx, request, &templateData)

	if err != nil {
		return err
	}

	templateData.ETag = response.Header.Get("etag")
    s.cache = templateData
    return nil
}


// Evaluate processes the cached template data with a condition evaluator 
// based on the provided context.
func (s *ServerTemplate) Evaluate(context map[string]interface{}) *ServerConfig {
    // TODO: Write ConditionalEvaluator for evaluating
    return &ServerConfig{ConfigValues: evaluator.Evaluate()}
}


type ServerConfig struct {
	Values	map[string]interface{}
}

func (s *ServerConfig) GetValue(key string) bool {
	return Values[key]
}





  
