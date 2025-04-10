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
	"testing"
)

const (
	paramOne     = "test_param_one"
	valueOne     = "test_value_one"
	paramTwo     = "test_param_two"
	valueTwo     = "{\"test\" : \"value\"}"
	paramThree   = "test_param_three"
	valueThree   = "123456789.123"
	paramFour    = "test_param_four"
	valueFour    = "1"
	conditionOne = "test_condition_one"
	testEtag     = "test-etag"
	testVersion  = "test-version"
)

// Test newServerTemplate with valid default config
func TestNewServerTemplateStringifiesDefaults(t *testing.T) {
	defaultConfig := map[string]any{
		"key1": "value1",
		"key2": 123,
		"key3": true,
		"key4": nil,
		"key5": "{\"test_param\" : \"test_value\"}",
	}

	expectedStringified := map[string]string{
		"key1": "value1",
		"key2": "123",
		"key3": "true",
		"key4": "", // nil becomes empty string
		"key5": "{\"test_param\" : \"test_value\"}",
	}

	rcClient := &rcClient{}
	template, err := newServerTemplate(rcClient, defaultConfig)
	if err != nil {
		t.Fatalf("newServerTemplate() error = %v", err)
	}
	if template == nil {
		t.Fatal("newServerTemplate() returned nil template")
	}

	if len(template.stringifiedDefaultConfig) != len(defaultConfig) {
		t.Errorf("len(stringifiedDefaultConfig) = %d, want %d", len(template.stringifiedDefaultConfig), len(expectedStringified))
	}

	for key, expectedValue := range expectedStringified {
		t.Run(key, func(t *testing.T) {
			actualValue, ok := template.stringifiedDefaultConfig[key]
			if !ok {
				t.Errorf("Key %q not found in stringifiedDefaultConfig", key)
			} else if actualValue != expectedValue {
				t.Errorf("stringifiedDefaultConfig[%q] = %q, want %q", key, actualValue, expectedValue)
			}
		})
	}
}

// Test ServerTemplate.Set with valid JSON
func TestServerTemplateSetSuccess(t *testing.T) {
	template := &ServerTemplate{}
	json := `{"conditions": [{"name": "percent_condition", "condition": {"orCondition": {"conditions": [{"andCondition": {"conditions": [{"percent": {"percentOperator": "BETWEEN", "seed": "fb4aczak670h", "microPercentRange": {"microPercentUpperBound": 34000000}}}]}}]}}}, {"name": "percent_2", "condition": {"orCondition": {"conditions": [{"andCondition": {"conditions": [{"percent": {"percentOperator": "BETWEEN", "seed": "yxmb9v8fafxg", "microPercentRange": {"microPercentLowerBound": 12000000, "microPercentUpperBound": 100000000}}}, {"customSignal": {"customSignalOperator": "STRING_CONTAINS", "customSignalKey": "test", "targetCustomSignalValues": ["hello"]}}]}}]}}}], "parameters": {"test": {"defaultValue": {"useInAppDefault": true}, "conditionalValues": {"percent_condition": {"value": "{\"condition\" : \"percent\"}"}}}}, "version": {"versionNumber": "266", "isLegacy": true}, "etag": "test_etag"}`
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
	template := &ServerTemplate{}
	value := "test_value_one" // The raw string value
	data := &serverTemplateData{
		Parameters: map[string]parameter{
			paramOne: {
				DefaultValue: parameterValue{
					Value: &value,
				},
			},
		},
		Version: struct {
			VersionNumber string "json:\"versionNumber\""
			IsLegacy      bool   "json:\"isLegacy\""
		}{
			VersionNumber: testVersion,
			IsLegacy:      true,
		},
		ETag: testEtag,
	}
	template.cache.Store(data)
	json, err := template.ToJSON()
	if err != nil {
		t.Fatalf("ServerTemplate.ToJSON failed: %v", err)
	}

	expectedJSON := `{"parameters":{"test_param_one":{"defaultValue":{"value":"test_value_one"}}},"version":{"versionNumber":"test-version","isLegacy":true},"etag":"test-etag"}`
	if json != expectedJSON {
		t.Fatalf("ServerTemplate.ToJSON returned incorrect json: %v want %v", json, expectedJSON)
	}
}

func TestServerTemplateReturnsDefaultFromRemote(t *testing.T) {
	paramVal := valueOne
	template := &ServerTemplate{}
	data := &serverTemplateData{
		Parameters: map[string]parameter{
			paramOne: {
				DefaultValue: parameterValue{
					Value: &paramVal,
				},
			},
		},
		Version: struct {
			VersionNumber string "json:\"versionNumber\""
			IsLegacy      bool   "json:\"isLegacy\""
		}{
			VersionNumber: testVersion,
		},
		ETag: testEtag,
	}
	template.cache.Store(data)

	context := make(map[string]any)
	config, err := template.Evaluate(context)

	if err != nil {
		t.Fatalf("Error in evaluating template %v", err)
	}
	if config == nil {
		t.Fatal("ServerTemplate.Evaluate returned nil config")
	}
	val := config.GetString(paramOne)
	src := config.GetValueSource(paramOne)
	if val != valueOne {
		t.Fatalf("ServerTemplate.Evaluate returned incorrect value: %v want %v", val, valueOne)
	}
	if src != Remote {
		t.Fatalf("ServerTemplate.Evaluate returned incorrect source: %v want %v", src, Remote)
	}
}

func TestEvaluateReturnsInAppDefault(t *testing.T) {
	booleanTrue := true
	td := &serverTemplateData{
		Parameters: map[string]parameter{
			paramOne: {
				DefaultValue: parameterValue{
					UseInAppDefault: &booleanTrue,
				},
			},
		},
		Version: struct {
			VersionNumber string "json:\"versionNumber\""
			IsLegacy      bool   "json:\"isLegacy\""
		}{
			VersionNumber: testVersion,
		},
		ETag: testEtag,
	}

	testCases := []struct {
		name                     string
		stringifiedDefaultConfig map[string]string
		expectedValue            string
		expectedSource           ValueSource
	}{
		{
			name:                     "No In-App Default Provided",
			stringifiedDefaultConfig: map[string]string{},
			expectedValue:            "",
			expectedSource:           Static,
		},
		{
			name:                     "In-App Default Provided",
			stringifiedDefaultConfig: map[string]string{paramOne: valueOne},
			expectedValue:            valueOne,
			expectedSource:           Default,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			st := ServerTemplate{
				stringifiedDefaultConfig: tc.stringifiedDefaultConfig,
			}
			st.cache.Store(td)

			config, err := st.Evaluate(map[string]any{})

			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if config == nil {
				t.Fatal("Evaluate() returned nil config")
			}
			val := config.GetString(paramOne)
			src := config.GetValueSource(paramOne)
			if val != tc.expectedValue {
				t.Errorf("GetString(%q) = %q, want %q", paramOne, val, tc.expectedValue)
			}
			if src != tc.expectedSource {
				t.Errorf("GetValueSource(%q) = %v, want %v", paramOne, src, tc.expectedSource)
			}
		})
	}
}

func TestEvaluate_WithACondition_ReturnsConditionalRemoteValue(t *testing.T) {
	vOne := valueOne
	vTwo := valueTwo

	template := &ServerTemplate{}
	data := &serverTemplateData{
		Parameters: map[string]parameter{
			paramOne: {
				DefaultValue: parameterValue{
					Value: &vOne,
				},
				ConditionalValues: map[string]parameterValue{
					conditionOne: {
						Value: &vTwo,
					},
				},
			},
		},
		Conditions: []namedCondition{
			{
				Name: conditionOne,
				Condition: &oneOfCondition{
					OrCondition: &orCondition{
						Conditions: []oneOfCondition{
							{
								Percent: &percentCondition{
									PercentOperator: between,
									Seed:            testSeed,
									MicroPercentRange: microPercentRange{
										MicroPercentLowerBound: 0,
										MicroPercentUpperBound: totalMicroPercentiles, // upper bound is set to the max; the percent condition will always evaluate to true
									},
								},
							},
						},
					},
				},
			},
		},
		Version: struct {
			VersionNumber string "json:\"versionNumber\""
			IsLegacy      bool   "json:\"isLegacy\""
		}{
			VersionNumber: testVersion,
		},
		ETag: testEtag,
	}
	template.cache.Store(data)

	context := map[string]any{randomizationID: testRandomizationID}
	config, err := template.Evaluate(context)

	if err != nil {
		t.Fatalf("Error in evaluating template %v", err)
	}
	if config == nil {
		t.Fatal("ServerTemplate.Evaluate returned nil config")
	}
	val := config.GetString(paramOne)
	src := config.GetValueSource(paramOne)
	if val != vTwo {
		t.Fatalf("ServerTemplate.Evaluate returned incorrect value: %v want %v", val, vTwo)
	}
	if src != Remote {
		t.Fatalf("ServerTemplate.Evaluate returned incorrect source: %v want %v", src, Remote)
	}
}

func TestEvaluate_WithACondition_ReturnsConditionalInAppDefaultValue(t *testing.T) {
	vOne := valueOne
	boolTrue := true
	template := &ServerTemplate{
		stringifiedDefaultConfig: map[string]string{paramOne: valueThree},
	}
	data := &serverTemplateData{
		Parameters: map[string]parameter{
			paramOne: {
				DefaultValue: parameterValue{
					Value: &vOne,
				},
				ConditionalValues: map[string]parameterValue{
					conditionOne: {
						UseInAppDefault: &boolTrue,
					},
				},
			},
		},
		Conditions: []namedCondition{
			{
				Name: conditionOne,
				Condition: &oneOfCondition{
					OrCondition: &orCondition{
						Conditions: []oneOfCondition{
							{
								Percent: &percentCondition{
									PercentOperator: between,
									Seed:            testSeed,
									MicroPercentRange: microPercentRange{
										MicroPercentLowerBound: 0,
										MicroPercentUpperBound: totalMicroPercentiles,
									},
								},
							},
						},
					},
				},
			},
		},
		Version: struct {
			VersionNumber string "json:\"versionNumber\""
			IsLegacy      bool   "json:\"isLegacy\""
		}{
			VersionNumber: testVersion,
		},
		ETag: testEtag,
	}
	template.cache.Store(data)

	context := map[string]any{randomizationID: testRandomizationID}
	config, err := template.Evaluate(context)

	if err != nil {
		t.Fatalf("Error in evaluating template %v", err)
	}
	if config == nil {
		t.Fatal("ServerTemplate.Evaluate returned nil config")
	}
	val := config.GetString(paramOne)
	src := config.GetValueSource(paramOne)
	if val != valueThree {
		t.Fatalf("ServerTemplate.Evaluate returned incorrect value: %v want %v", val, valueThree)
	}
	if src != Default {
		t.Fatalf("ServerTemplate.Evaluate returned incorrect source: %v want %v", src, Default)
	}
}
