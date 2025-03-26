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
	"sync/atomic"
	"testing"
)

// Test newServerTemplate with valid default config
func TestNewServerTemplateSuccess(t *testing.T) {
	defaultConfig := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
		"key4": nil,
	}

	rcClient := &rcClient{}
	template, err := newServerTemplate(rcClient, defaultConfig)
	if err != nil {
		t.Fatalf("newServerTemplate failed: %v", err)
	}
	if template == nil {
		t.Error("newServerTemplate returned nil template")
	}

	if len(template.stringifiedDefaultConfig) != len(defaultConfig) {
		t.Errorf("newServerTemplate stringifiedDefaultConfig length = %v, want %v", len(template.stringifiedDefaultConfig), len(defaultConfig))
	}

	if template.stringifiedDefaultConfig["key1"] != "\"value1\"" {
		t.Errorf("newServerTemplate stringifiedDefaultConfig key1 = %v, want %v", template.stringifiedDefaultConfig["key1"], "\"value1\"")
	}

	if template.stringifiedDefaultConfig["key2"] != "123" {
		t.Errorf("newServerTemplate stringifiedDefaultConfig key2 = %v, want %v", template.stringifiedDefaultConfig["key2"], "123")
	}
	if template.stringifiedDefaultConfig["key3"] != "true" {
		t.Errorf("newServerTemplate stringifiedDefaultConfig key3 = %v, want %v", template.stringifiedDefaultConfig["key3"], "true")
	}
	if template.stringifiedDefaultConfig["key4"] != "" {
		t.Errorf("newServerTemplate stringifiedDefaultConfig key4 = %v, want %v", template.stringifiedDefaultConfig["key4"], "")
	}
}

// Test ServerTemplate.Set with valid JSON
func TestServerTemplateSetSuccess(t *testing.T) {
	template := &ServerTemplate{}
	json := `{"parameters": {"test_param": {"defaultValue": {"value": "test_value"}}}}`
	err := template.Set(json)
	if err != nil {
		t.Fatalf("ServerTemplate.Set failed: %v", err)
	}
	if template.cache.Load() == nil {
		t.Fatal("ServerTemplate.Set did not store data in cache")
	}
}

// Test ServerTemplate.ToJSON with valid data
func TestServerTemplateToJSONSuccess(t *testing.T) {
	template := &ServerTemplate{
		cache: atomic.Pointer[serverTemplateData]{},
	}
	data := &serverTemplateData{
		Parameters: map[string]parameter{
			"test_param": {
				// Just provide the field values; Go infers the correct anonymous struct type
				DefaultValue: struct {
					Value string `json:"value"`
				}{
					Value: "test_value",
				},
			},
		},
	}
	template.cache.Store(data)
	json, err := template.ToJSON()
	if err != nil {
		t.Fatalf("ServerTemplate.ToJSON failed: %v", err)
	}

	expectedJSON := `{"conditions":null,"parameters":{"test_param":{"defaultValue":{"value":"test_value"},"conditionalValues":null}},"version":{"versionNumber":"","isLegacy":false},"ETag":""}`
	if json != expectedJSON {
		t.Fatalf("ServerTemplate.ToJSON returned incorrect json: %v want %v", json, expectedJSON)
	}
}

// Test ServerTemplate.Evaluate with valid data
func TestServerTemplateEvaluateSuccess(t *testing.T) {
	template := &ServerTemplate{
		cache: atomic.Pointer[serverTemplateData]{},
	}
	data := &serverTemplateData{
		Parameters: map[string]parameter{
			"test_param": {
				DefaultValue: struct {
					Value string `json:"value"`
				}{Value: "test_value"},
			},
		},
	}
	template.cache.Store(data)

	context := map[string]interface{}{"test_context": "test_value"}
	config := template.Evaluate(context)
	if config == nil {
		t.Fatal("ServerTemplate.Evaluate returned nil config")
	}

	if config.GetString("test_param") != "test_value" {
		t.Fatalf("ServerTemplate.Evaluate returned incorrect value: %v want %v", config.GetString("test_param"), "test_value")
	}
}
