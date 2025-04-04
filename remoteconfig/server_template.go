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

package remoteconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"firebase.google.com/go/v4/internal"
)

// serverTemplateData stores the internal representation of the server template.
type serverTemplateData struct {
	// A list of conditions in descending order by priority.
	Conditions []namedCondition `json:"conditions,omitempty"`

	// Map of parameter keys to their optional default values and optional conditional values.
	Parameters map[string]remoteConfigParameter `json:"parameters,omitempty"`

	// Version information for the current Remote Config template.
	Version version `json:"version,omitempty"`

	// Current Remote Config template ETag.
	ETag string `json:"etag,omitempty"`
}

// ServerTemplate represents a template with configuration data, cache, and service information.
type ServerTemplate struct {
	rcClient                 *rcClient
	cache                    atomic.Pointer[serverTemplateData]
	stringifiedDefaultConfig map[string]string
}

// NewServerTemplate initializes a new ServerTemplate with optional default configuration.
func newServerTemplate(rcClient *rcClient, defaultConfig map[string]any) (*ServerTemplate, error) {
	stringifiedConfig := make(map[string]string, len(defaultConfig)) // Pre-allocate map

	for key, value := range defaultConfig {
		if value == nil {
			stringifiedConfig[key] = ""
			continue
		}

		// Marshal the value to JSON bytes
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config key '%s': %w", key, err)
		}

		stringifiedConfig[key] = string(jsonBytes)
	}

	return &ServerTemplate{
		rcClient:                 rcClient,
		stringifiedDefaultConfig: stringifiedConfig,
	}, nil
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Load(ctx context.Context) error {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/v1/projects/%s/namespaces/firebase-server/serverRemoteConfig", s.rcClient.rcBaseURL, s.rcClient.project),
	}

	templateData := new(serverTemplateData)
	response, err := s.rcClient.httpClient.DoAndUnmarshal(ctx, request, &templateData)

	if err != nil {
		return err
	}

	templateData.ETag = response.Header.Get("etag")
	s.cache.Store(templateData)
	return nil
}

// Set initializes a template using a server template JSON.
func (s *ServerTemplate) Set(templateDataJSON string) error {
	templateData := new(serverTemplateData)
	if err := json.Unmarshal([]byte(templateDataJSON), &templateData); err != nil {
		return fmt.Errorf("error while parsing server template: %v", err)
	}
	s.cache.Store(templateData)
	return nil
}

// ToJSON Returns a json representing the cached serverTemplateData.
func (s *ServerTemplate) ToJSON() (string, error) {
	jsonServerTemplate, err := json.Marshal(s.cache.Load())

	if err != nil {
		return "", fmt.Errorf("error while parsing server template: %v", err)
	}

	return string(jsonServerTemplate), nil
}

// Evaluate and processes the cached template data.
func (s *ServerTemplate) Evaluate() *ServerConfig {
	configMap := make(map[string]value)
	return &ServerConfig{ConfigValues: configMap}
}
