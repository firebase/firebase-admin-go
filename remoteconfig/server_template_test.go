package remoteconfig

import (
	"fmt"
	"testing"
)

func Test_StringifyDefaultConfig(t *testing.T) {
	defaultConfig := map[string]any{
		"paramInt":    1,
		"paramFloat":  34.534,
		"paramBool":   true,
		"paramString": "string",
		"paramObject": sampleObject{},
		"paramJSON":   "{\"name\" : \"abc\"}",
	}
	stringifiedConfig := stringifyDefaultConfig(defaultConfig)
	expectedConfig := map[string]Value{
		"paramInt":    {value: "1", source: Default},
		"paramFloat":  {value: "34.534", source: Default},
		"paramBool":   {value: "true", source: Default},
		"paramString": {value: "string", source: Default},
		"paramJSON":   {value: "{\"name\" : \"abc\"}", source: Default},
		"paramObject": {value: "", source: Static},
	}

	if len(stringifiedConfig) != len(expectedConfig) {
		t.Fatalf("Incorrect stringified config generated")
	}

	for k, v := range expectedConfig {
		if actualValue, ok := stringifiedConfig[k]; ok {
			if v.value != actualValue.value {
				t.Errorf("Value [%s] for key [%s] does not match the expected value [%s]", actualValue.value, k, v.value)
			}
			if v.source != actualValue.source {
				t.Errorf("Source [%s] for key [%s] does not match the expected value [%s]", actualValue.source, k, v.source)
			}
		} else {
			t.Errorf("Key %s is not present in the stringified config", k)
		}
	}
}

func TestEvaluate_Sequential(t *testing.T) {
	trueVal := true
	param1Condition2Val := "1500"
	param1DefaultVal := "2000"
	param2Condition1Val := "premium content unlocked"
	param2Condition2Val := "upgrade to premium"

	serverTemplateData := ServerTemplateData{
		Parameters: map[string]RemoteConfigParameter{
			"param_1": {
				ConditionalValues: map[string]RemoteConfigParameterValue{
					"condition_1": {
						UseInAppDefault: &trueVal,
					},
					"condition_2": {
						Value: &param1Condition2Val,
					},
				},
				DefaultValue: RemoteConfigParameterValue{
					Value: &param1DefaultVal,
				},
			},
			"param_2": {
				ConditionalValues: map[string]RemoteConfigParameterValue{
					"condition_1": {
						Value: &param2Condition1Val,
					},
					"condition_2": {
						Value: &param2Condition2Val,
					},
				},
				DefaultValue: RemoteConfigParameterValue{
					UseInAppDefault: &trueVal,
				},
			},
		},
		Conditions: []NamedCondition{
			createNamedCondition("condition_1", OneOfCondition{
				OrCondition: &OrCondition{
					Conditions: []OneOfCondition{
						{
							AndCondition: &AndCondition{
								Conditions: []OneOfCondition{
									{
										CustomSignal: &CustomSignalCondition{
											CustomSignalOperator:     "STRING_EXACTLY_MATCHES",
											CustomSignalKey:          "tier",
											TargetCustomSignalValues: []string{"paid"},
										},
									},
									{
										Percent: &PercentCondition{
											PercentOperator: "BETWEEN",
											MicroPercentRange: MicroPercentRange{
												MicroPercentUpperBound: 10_000_000,
											},
											Seed: "8f2uskkw1m66",
										},
									},
								},
							},
						},
					},
				},
			}),
			createNamedCondition("condition_2", OneOfCondition{
				OrCondition: &OrCondition{
					Conditions: []OneOfCondition{
						{
							AndCondition: &AndCondition{
								Conditions: []OneOfCondition{
									{
										CustomSignal: &CustomSignalCondition{
											CustomSignalOperator:     "STRING_EXACTLY_MATCHES",
											CustomSignalKey:          "tier",
											TargetCustomSignalValues: []string{"free"},
										},
									},
									{
										Percent: &PercentCondition{
											PercentOperator: "BETWEEN",
											MicroPercentRange: MicroPercentRange{
												MicroPercentLowerBound: 60_000_000,
												MicroPercentUpperBound: 76_000_000,
											},
											Seed: "8f2uskkw1m66",
										},
									},
								},
							},
						},
					},
				},
			}),
		},
	}

	evaluateTestCases := []struct {
		description string
		context        map[string]any
		expectedConfig map[string]Value
	}{
		{
			description : "None of the conditions are true; parameters get the default values",
			context: map[string]any{
				"randomizationId": "ca39e9ea-c7dd-4ad6-86a1-7c52a0948481", // instanceMicroPercentile is 84_241_465
				"tier":            "paid",
			},
			expectedConfig: map[string]Value{
				"param_1": {value: "2000", source: Remote},
				"param_2": {value: "welcome!", source: Default},
			},
		},
		{
			description : "condition_1 evaluates to true",
			context: map[string]any{
				"randomizationId": "1234", // instanceMicroPercentile is 9_821_973
				"tier":            "paid",
			},
			expectedConfig: map[string]Value{
				"param_1": {value: "2500", source: Default},
				"param_2": {value: "premium content unlocked", source: Remote},
			},
		},
		{
			description : "condition_2 evaluates to true",
			context: map[string]any{
				"randomizationId": "5353020c-204c-4bb2-9c20-cf9a7fd4c6d4", // instanceMicroPercentile is 74_742_002
				"tier":            "free",
			},
			expectedConfig: map[string]Value{
				"param_1": {value: "1500", source: Remote},
				"param_2": {value: "upgrade to premium", source: Remote},
			},
		},
	}

	for idx, tc := range evaluateTestCases {
		t.Run(fmt.Sprintf("Scenario #%d:%s", idx, tc.description), func(t *testing.T) {
			serverTemplate := ServerTemplate{
				Cache: &serverTemplateData,
				defaultConfig: map[string]any{
					"param_1": 2500,
					"param_2": "welcome!",
				},
			}
			config, err := serverTemplate.Evaluate(tc.context)
			if err != nil {
				t.Fatalf("Error in computing config %s", err.Error())
			}

			if len(config.ConfigValues) != len(tc.expectedConfig) {
				t.Fatalf("incorrect number of parameters are returned")
			}

			for param, eVal := range tc.expectedConfig {
				if aVal, ok := config.ConfigValues[param]; ok {
					if eVal.source != aVal.source {
						t.Errorf("Expected source [%s] for param [%s], but got [%s]", eVal.source, param, aVal.source)
					}

					if eVal.value != aVal.value {
						t.Errorf("Expected value [%s] for param [%s], but got [%s]", eVal.value, param, aVal.value)
					}

				} else {
					t.Errorf("Parameter [%s] not found in the config", param)
				}
			}
		})
	}
}
