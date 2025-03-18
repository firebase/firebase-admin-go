// Copyright 2025 Google Inc. All Rights Reserved.
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

// Package messaging contains functions for sending messages and managing
// device subscriptions with Firebase Cloud Messaging (FCM).
package remoteconfig

import (
	"context"
	"fmt"
	"net/http"
	"unsafe"
	
	"firebase.google.com/go/v4/internal"
)

type ServerTemplateData struct {
	Conditions []struct {
		Name      string      `json:"name"`
		Condition interface{} `json:"condition"`
	} `json:"conditions"`
	Parameters map[string]RemoteConfigParameter `json:"parameters"`

	Version struct {
		VersionNumber string `json:"versionNumber"`
		IsLegacy bool `json:"isLegacy"`
	}	`json:"version"`

	ETag  string
}

type RemoteConfigParameter struct {
	DefaultValue struct {
		Value string `json:"value"`
	} `json:"defaultValue"`
	ConditionalValues map[string]RemoteConfigParameterValue `json:"conditionalValues"`
}

type RemoteConfigParameterValue interface{}

// ServerTemplate represents a template with configuration data, cache, and service information.
type ServerTemplate struct {
	rcClient  *rcClient
	cache     Pointer[ServerTemplateData]
	stringifiedDefaultConfig map[string]string
}

// NewServerTemplate initializes a new ServerTemplate with optional default configuration.
func newServerTemplate(rcClient *rcClient, defaultConfig map[string]any) *ServerTemplate {
	// TODO: Create stringified config to be type safe:
	stringifiedConfig := make(map[string]string)
    for key, value := range defaultConfig{
        stringifiedConfig[key] = string(defaultConfig[key])
    }

	return &ServerTemplate{
		rcClient: rcClient,
		cache:	  nil,
		stringifiedDefaultConfig: stringifiedConfig,
	}
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Load(ctx context.Context) error {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/v1/projects/%s/namespaces/firebase-server/serverRemoteConfig", s.rcClient.rcBaseUrl , s.rcClient.project),
	}

	var templateData ServerTemplateData
	response, err := s.rcClient.httpClient.DoAndUnmarshal(ctx, request, &templateData)

	if err != nil {
		return err
	}

	templateData.ETag = response.Header.Get("etag")
	s.cache.Store(templateData)
	fmt.Println("Etag", s.cache.Load().ETag) // TODO: Remove ETag 
	return nil
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Set(templateDataJson string) {
    var templateData ServerTemplateData
    if err := json.Unmarshal(templateDataJson, &templateData); err != nil {
        return fmt.Errorf("error while parsing server template: %v", err)
    }
    s.cache.Store(templateData)
}

// Returns a json representing the cached ServerTemplateData.
func (s *ServerTemplate) ToJSON() string, error {
    jsonServerTemplate, err := json.Marshal(s.cache.Load())
 
    if (err != nil) {
        return nil, fmt.Errorf("error while parsing server template: %v", err)
    }

    return string(jsonServerTemplate), nil
}

// Evaluate processes the cached template data with a condition evaluator 
// based on the provided context.
func (s *ServerTemplate) Evaluate(context map[string]interface{}) *ServerConfig {
	// TODO: Write ConditionalEvaluator for evaluating
    configMap := make(map[string]Value)
    for key, value := range s.cache.Load().Parameters{
        configMap[key] = *NewValue(Remote, value.DefaultValue.Value)
    }

	return &ServerConfig{ConfigValues: configMap}
}