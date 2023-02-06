// Copyright 2019 Google Inc. All Rights Reserved.
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

package auth

import (
	"reflect"
	"testing"
)

// test validated and converted MFA config
func TestMfaConfig(t *testing.T) {
	mfaConfig := map[string]interface{}{
		"state": "ENABLED",
		"factorIds": []string{
			"PHONE_SMS",
		},
		"providerConfigs": []map[string]interface{}{
			{
				"state": "ENABLED",
				"totpProviderConfig": map[string]interface{}{
					"adjacentIntervals": 5,
				},
			},
		},
	}
	body, err := validateAndConvertMultiFactorConfig(mfaConfig)
	want := nestedMap{
		"state": "ENABLED",
		"enabledProviders": []string{
			"PHONE_SMS",
		},
		"providerConfigs": []map[string]interface{}{
			{
				"state": "ENABLED",
				"totpProviderConfig": map[string]interface{}{
					"adjacentIntervals": 5,
				},
			},
		},
	}
	if !reflect.DeepEqual(body, want) || err != nil {
		t.Errorf("TestMfaConfig() = (%v, %q), want = (%v, nil)", body, err, want)
	}
}

// test for invalid MFA config type
func TestInvalidMfaConfig(t *testing.T) {
	invalidMfaConfigs := []interface{}{
		"",
		[]int{},
		1,
		true,
		map[int]int{},
	}
	for _, mfaConfig := range invalidMfaConfigs {
		body, err := validateAndConvertMultiFactorConfig(mfaConfig)
		want := `multiFactorConfig must be a valid MultiFactorConfig type`
		if body != nil || want != err.Error() {
			t.Errorf(`TestInvalidMfaConfig() = (%v, %q), want = (nil, %q)`, body, err, want)
		}
	}
}

// test for invalid MFA Config params
func TestInvalidMfaConfigParams(t *testing.T) {
	mfaConfig := map[string]interface{}{
		"state":   "ENABLED",
		"invalid": "",
	}
	body, err := validateAndConvertMultiFactorConfig(mfaConfig)
	want := `"invalid" is not a valid MultiFactorConfig parameter`
	if body != nil || want != err.Error() {
		t.Errorf("TestInvalidMfaConfigParams = (%v, %q), want = (nil, %q)", body, err, want)
	}
}

// test for undefined MFA config state
func TestMfaConfigNoState(t *testing.T) {
	mfaConfig := map[string]interface{}{
		"factorIds": []interface{}{"PHONE_SMS"},
	}
	body, err := validateAndConvertMultiFactorConfig(mfaConfig)
	want := `multiFactorConfig.state should be defined`
	if body != nil || want != err.Error() {
		t.Errorf("TestMfaConfigNoState() = (%v, %q), want = (nil, %q)", body, err, want)
	}
}

// test for invalid MFA config state
func TestMfaConfigInvalidStates(t *testing.T) {
	body, err := validateAndConvertMultiFactorConfig(map[string]interface{}{
		"state": "INVALID_STATE",
	})
	want := `multiFactorConfig.state must be either "ENABLED" or "DISABLED"`
	if body != nil || want != err.Error() {
		t.Errorf("TestMfaConfigInvalidStates() = (%v, %q), want = (nil, %q)", body, err, want)
	}
}

// /test for invalid factorIds type
func TestMfaConfigInvalidFactorIds(t *testing.T) {
	invalidFactorIds := []interface{}{
		"invalid",
		true,
		1,
		map[string]interface{}{},
		[]int{},
	}
	for _, factorIds := range invalidFactorIds {
		body, err := validateAndConvertMultiFactorConfig(map[string]interface{}{
			"state":     "ENABLED",
			"factorIds": factorIds,
		})
		want := `multiFactorConfig.factorIds must be a defined list of AuthFactor type strings`
		if body != nil || want != err.Error() {
			t.Errorf("TestMfaConfigInvalidFactorIds() = (%v, %q), want = (nil, %q)", body, err, want)
		}
	}
}

// test for invalid Factor ID string
func TestMfaConfigInvalidFactorIdsString(t *testing.T) {
	invalidFactorId := []string{
		"invalid",
	}
	body, err := validateAndConvertMultiFactorConfig(map[string]interface{}{
		"state":     "ENABLED",
		"factorIds": invalidFactorId,
	})
	want := `factorId must be a valid AuthFactor type string`
	if body != nil || want != err.Error() {
		t.Errorf("TestMfaConfigInvalidFactorIdsString() = (%v, %q), want = (nil, %q)", body, err, want)
	}
}

// test for invalid Provider configs types
func TestMfaConfigInvalidProviderConfigs(t *testing.T) {
	invalidProviderConfigs := []interface{}{
		"invalid",
		true,
		1,
		map[string]interface{}{},
		[]int{},
	}
	for _, providerConfigs := range invalidProviderConfigs {
		body, err := validateAndConvertMultiFactorConfig(map[string]interface{}{
			"state":           "ENABLED",
			"providerConfigs": providerConfigs,
		})
		want := `multiFactorConfig.providerConfigs must be a list of ProviderConfigs`
		if body != nil || want != err.Error() {
			t.Errorf("TestMfaConfigInvalidProviderConfigs() = (%v, %q), want = (nil, %q)", body, err, want)
		}
	}
}

// test for invalid Provider config params
func TestMfaConfigInvalidProviderConfigParams(t *testing.T) {
	mfaConfig := map[string]interface{}{
		"state": "ENABLED",
		"providerConfigs": []map[string]interface{}{
			{
				"state":   "ENABLED",
				"invalid": "",
			},
		},
	}
	body, err := validateAndConvertMultiFactorConfig(mfaConfig)
	want := `"invalid" is not a valid providerConfig parameter`
	if body != nil || want != err.Error() {
		t.Errorf("TestMfaConfigInvalidProviderConfigParams() = (%v, %q), want = (nil, %q)", body, err, want)
	}
}

// test for undefined Provider config state
func TestMfaConfigUndefinedProviderConfigState(t *testing.T) {
	mfaConfig := map[string]interface{}{
		"state": "ENABLED",
		"providerConfigs": []map[string]interface{}{
			{
				"totpProviderConfig": map[string]interface{}{},
			},
		},
	}
	body, err := validateAndConvertMultiFactorConfig(mfaConfig)
	want := `providerConfig.state should be defined`
	if body != nil || want != err.Error() {
		t.Errorf("TestMfaConfigUndefinedProviderConfigState() = (%v, %q), want = (nil, %q)", body, err, want)
	}
}

// test for invalid Provider config state
func TestMfaConfigInvalidProviderConfigState(t *testing.T) {
	mfaConfig := map[string]interface{}{
		"state": "ENABLED",
		"providerConfigs": []map[string]interface{}{
			{
				"state":              "INVALID_STATE",
				"totpProviderConfig": map[string]interface{}{},
			},
		},
	}
	body, err := validateAndConvertMultiFactorConfig(mfaConfig)
	want := `providerConfig.state must be either "ENABLED" or "DISABLED"`
	if body != nil || want != err.Error() {
		t.Errorf("TestMfaConfigInvalidProviderConfigState() = (%v, %q), want = (nil, %q)", body, err, want)
	}
}

// test for undefined TOTP provider config
func TestMfaConfigUndefinedTotpProviderConfig(t *testing.T) {
	mfaConfig := map[string]interface{}{
		"state": "ENABLED",
		"providerConfigs": []map[string]interface{}{
			{
				"state": "ENABLED",
			},
		},
	}
	body, err := validateAndConvertMultiFactorConfig(mfaConfig)
	want := `providerConfig.totpProviderConfig should be instantiated`
	if body != nil || want != err.Error() {
		t.Errorf("TestMfaConfigUndefinedTotpProviderConfig() = (%v, %q), want = (nil, %q)", body, err, want)
	}
}

// test for invalid TOTP provider config
func TestMfaConfigInvalidTotpProviderConfig(t *testing.T) {
	invalidTotpProviderConfigs := []interface{}{
		[]int{},
		1,
		false,
		map[string]string{},
		"",
	}
	for _, totpProviderConfig := range invalidTotpProviderConfigs {
		mfaConfig := map[string]interface{}{
			"state": "ENABLED",
			"providerConfigs": []map[string]interface{}{
				{
					"state":              "ENABLED",
					"totpProviderConfig": totpProviderConfig,
				},
			},
		}
		body, err := validateAndConvertMultiFactorConfig(mfaConfig)
		want := `totpProviderConfig must be of type TotpProviderConfig`
		if body != nil || want != err.Error() {
			t.Errorf("TestMfaConfigInvalidTotpProviderConfig() = (%v, %q), want = (nil, %q)", body, err, want)
		}
	}
}

// test for invalid TOTP provider config params
func TestMfaConfigInvalidTotpProviderConfigParams(t *testing.T) {
	mfaConfig := map[string]interface{}{
		"state": "ENABLED",
		"providerConfigs": []map[string]interface{}{
			{
				"state": "ENABLED",
				"totpProviderConfig": map[string]interface{}{
					"invalid": 5,
				},
			},
		},
	}
	body, err := validateAndConvertMultiFactorConfig(mfaConfig)
	want := `"invalid" is not a valid totpProviderConfig parameter`
	if body != nil || want != err.Error() {
		t.Errorf("TestMfaConfigInvalidTotpProviderConfigParams() = (%v, %q), want = (nil, %q)", body, err, want)
	}
}

// test for invalid Adjacent intervals
func TestMfaConfigInvalidTotpProviderConfigAdjacentIntervals(t *testing.T) {
	invalidAdjacentIntervals := []interface{}{
		11,
		-1,
		[]int{},
		"",
	}
	for _, adjacentIntervals := range invalidAdjacentIntervals {
		mfaConfig := map[string]interface{}{
			"state": "ENABLED",
			"providerConfigs": []map[string]interface{}{
				{
					"state": "ENABLED",
					"totpProviderConfig": map[string]interface{}{
						"adjacentIntervals": adjacentIntervals,
					},
				},
			},
		}
		body, err := validateAndConvertMultiFactorConfig(mfaConfig)
		want := `adjacentIntervals must be a valid number between 0 and 10 (both inclusive)`
		if body != nil || want != err.Error() {
			t.Errorf("TestMfaConfigInvalidTotpProviderConfigAdjacentIntervals() = (%v, %q), want = (nil, %q)", body, err, want)
		}
	}
}

func TestMfaConfigNil(t *testing.T) {
	body, err := validateAndConvertMultiFactorConfig(nil)
	want := `multiFactorConfig must be defined`
	if body != nil || err.Error() != want {
		t.Errorf("TestMfaConfigNil() = (%v, %q), want = (nil, %q)", body, err, want)
	}
}
