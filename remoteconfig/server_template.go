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
	"net/http"
	"strconv"

	"firebase.google.com/go/v4/internal"
)

// ServerTemplate represents a template with configuration data, cache, and service information.
type ServerTemplate struct {
	RcClient  *rcClient
	Cache     *ServerTemplateData
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
	// Defaults to type string if unspecified.
	ValueType ParameterValueType
}

// Represents a Remote Config parameter value
// that could be either an explicit parameter value or an in-app default value.
type RemoteConfigParameterValue struct {
	// The `string` value that the parameter is set to when it is an explicit parameter value
	Value *string

	// If true, indicates that the in-app default value is to be used for the parameter
	UseInAppDefault *bool
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
	return nil
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Set(templateData *ServerTemplateData) {
	s.Cache = templateData
}

func stringifyDefaultConfig(context map[string]any) map[string]Value {
	config := make(map[string]Value)
	for key, value := range context {
		var valueAsString string
		switch value := value.(type) {
		case string:
			valueAsString = value
		case int:
			valueAsString = strconv.Itoa(value)
		case float64:
			valueAsString = strconv.FormatFloat(value, 'f', -1, 64)
		case bool:
			valueAsString = strconv.FormatBool(value)
		}
		config[key] = Value{source: Default, value: valueAsString}
	}
	return config
}

// Process the cached template data with a condition evaluator based on the provided context.
func (s *ServerTemplate) Evaluate(context map[string]any) (*ServerConfig, error) {
	if s.Cache == nil {
		return &ServerConfig{}, errors.New("no Remote Config Server template in cache, call Load() before calling Evaluate()")
	}
	ce := ConditionEvaluator{
		conditions:        s.Cache.Conditions,
		evaluationContext: context,
	}
	orderedConditions, evaluatedConditions := ce.evaluateConditions()
	config := make(map[string]Value)
	for key, param := range s.Cache.Parameters {
		var paramValueWrapper RemoteConfigParameterValue 
		for _, condition := range orderedConditions {
			if value, ok := param.ConditionalValues[condition]; ok && evaluatedConditions[condition] {
				paramValueWrapper = value 
				break 
			}
		}

		if paramValueWrapper.UseInAppDefault != nil && *paramValueWrapper.UseInAppDefault {
			continue
		}

		if paramValueWrapper.Value != nil {
			config[key] = Value{source: Remote, value: *paramValueWrapper.Value}
			continue
		}

		if param.DefaultValue.UseInAppDefault != nil && *param.DefaultValue.UseInAppDefault {
			continue
		}

		if param.DefaultValue.Value != nil {
			config[key] = Value{source: Remote, value : *param.DefaultValue.Value}
		}
	}
	return &ServerConfig{ConfigValues: config}, nil
}
