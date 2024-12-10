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

// Package messaging contains functions for sending messages and managing
// device subscriptions with Firebase Cloud Messaging (FCM).
package remoteconfig

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"firebase.google.com/go/v4/internal"
)

// ServerTemplate represents a template with configuration data, cache, and service information.
type ServerTemplate struct {
	RcClient      *rcClient
	Cache         *ServerTemplateData
	defaultConfig map[string]any
}

// Represents the data in a Remote Config server template.
type ServerTemplateData struct {
	// A list of conditions in descending order by priority.
	Conditions []NamedCondition

	// Map of parameter keys to their optional default values and optional conditional values.
	Parameters map[string]RemoteConfigParameter

	// Current Remote Config template ETag.
	ETag string

	// Version information for the current Remote Config template.
	Version *Version
}

// Structure representing a Remote Config parameter.
// At minimum, a `defaultValue` or a `conditionalValues` entry must be present for the parameter to have any effect.
type RemoteConfigParameter struct {

	// The value to set the parameter to, when none of the named conditions evaluate to `true`.
	DefaultValue RemoteConfigParameterValue

	// A `(condition name, value)` map. The condition name of the highest priority
	// (the one listed first in the Remote Config template's conditions list) determines the value of this parameter.
	ConditionalValues map[string]RemoteConfigParameterValue

	// A description for this parameter. Should not be over 100 characters and may contain any Unicode characters.
	Description string

	// The data type for all values of this parameter in the current version of the template.
	// It can be a string, number, boolean or JSON, and defaults to type string if unspecified.
	ValueType string
}

// Represents a Remote Config parameter value
// that could be either an explicit parameter value or an in-app default value.
type RemoteConfigParameterValue struct {
	// The `string` value that the parameter is set to when it is an explicit parameter value
	Value *string

	// If true, indicates that the in-app default value is to be used for the parameter
	UseInAppDefault *bool
}

func stringifyDefaultConfig(defaultConfig map[string]any) map[string]Value {
	stringifiedConfig := make(map[string]Value)
	for key, value := range defaultConfig {
		// In-app default values other than these data types will be assigned the empty string
		switch value := value.(type) {
		case string:
			stringifiedConfig[key] = Value{source: Default, value: value}
		case int:
			stringifiedConfig[key] = Value{source: Default, value: strconv.Itoa(value)}
		case float64:
			stringifiedConfig[key] = Value{source: Default, value: strconv.FormatFloat(value, DecimalFormat, MinBitsPossible, DoublePrecisionWidth)}
		case bool:
			stringifiedConfig[key] = Value{source: Default, value: strconv.FormatBool(value)}
		default:
			stringifiedConfig[key] = Value{source: Static}
		}
	}
	return stringifiedConfig
}

// NewServerTemplate initializes a new ServerTemplate with optional default configuration.
func NewServerTemplate(RcClient *rcClient, defaultConfig map[string]any) *ServerTemplate {
	return &ServerTemplate{
		RcClient:      RcClient,
		Cache:         nil,
		defaultConfig: defaultConfig,
	}
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Load(ctx context.Context) error {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/v1/projects/%s/namespaces/firebase-server/serverRemoteConfig", s.RcClient.RcBaseUrl, s.RcClient.Project),
	}

	var templateData ServerTemplateData
	response, err := s.RcClient.HttpClient.DoAndUnmarshal(ctx, request, &templateData)

	if err != nil {
		return err
	}

	templateData.ETag = response.Header.Get("etag")
	s.Cache = &templateData
	return nil
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Set(templateData *ServerTemplateData) {
	s.Cache = templateData
}

// Process the cached template data with a condition evaluator based on the provided context.
func (s *ServerTemplate) Evaluate(context map[string]any) (*ServerConfig, error) {
	if s.Cache == nil {
		return &ServerConfig{}, errors.New("no Remote Config Server template in Cache, call Load() before calling Evaluate()")
	}
	
	// Initializes config object with in-app default values.
	config := stringifyDefaultConfig(s.defaultConfig)
	ce := ConditionEvaluator{
		conditions:        s.Cache.Conditions,
		evaluationContext: context,
	}
	evaluatedConditions := ce.evaluateConditions()

	// Overlays config Value objects derived by evaluating the template.
	for key, param := range s.Cache.Parameters {
		var paramValueWrapper RemoteConfigParameterValue

		// Iterates in order over the condition list. If there is a value associated with a condition, this checks if the condition is true.
		for _, condition := range s.Cache.Conditions {
			if value, ok := param.ConditionalValues[condition.Name]; ok && evaluatedConditions[condition.Name] {
				paramValueWrapper = value
				break
			}
		}

		if paramValueWrapper.UseInAppDefault != nil && *paramValueWrapper.UseInAppDefault {
			log.Printf("[INFO] Using in-app default for the parameter '%s'.\n", key)
			continue
		}

		if paramValueWrapper.Value != nil {
			config[key] = Value{source: Remote, value: *paramValueWrapper.Value}
			continue
		}

		if param.DefaultValue.UseInAppDefault != nil && *param.DefaultValue.UseInAppDefault {
			log.Printf("[INFO] Using in-app default for parameter '%s''s default value.\n", key)
			continue
		}

		if param.DefaultValue.Value != nil {
			config[key] = Value{source: Remote, value: *param.DefaultValue.Value}
		}
	}
	return &ServerConfig{ConfigValues: config}, nil
}
