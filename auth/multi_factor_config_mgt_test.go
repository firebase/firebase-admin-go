// Copyright 2023 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"testing"
)

func TestMultiFactorConfig(t *testing.T) {
	mfa := MultiFactorConfig{
		ProviderConfigs: []*ProviderConfig{{
			State: Disabled,
			TOTPProviderConfig: &TOTPProviderConfig{
				AdjacentIntervals: 5,
			},
		}},
	}
	if err := mfa.validate(); err != nil {
		t.Errorf("MultiFactorConfig not valid")
	}
}
func TestMultiFactorConfigNoProviderConfigs(t *testing.T) {
	mfa := MultiFactorConfig{}
	want := "\"ProviderConfigs\" must be a non-empty array of type \"ProviderConfig\"s"
	if err := mfa.validate(); err.Error() != want {
		t.Errorf("MultiFactorConfig.validate(nil) = %v, want = %q", err, want)
	}
}

func TestMultiFactorConfigNilProviderConfigs(t *testing.T) {
	mfa := MultiFactorConfig{
		ProviderConfigs: nil,
	}
	want := "\"ProviderConfigs\" must be a non-empty array of type \"ProviderConfig\"s"
	if err := mfa.validate(); err.Error() != want {
		t.Errorf("MultiFactorConfig.validate(nil) = %v, want = %q", err, want)
	}
}

func TestMultiFactorConfigNilProviderConfig(t *testing.T) {
	mfa := MultiFactorConfig{
		ProviderConfigs: []*ProviderConfig{nil},
	}
	want := "\"ProviderConfigs\" must be a non-empty array of type \"ProviderConfig\"s"
	if err := mfa.validate(); err.Error() != want {
		t.Errorf("MultiFactorConfig.validate(nil) = %v, want = %q", err, want)
	}
}

func TestMultiFactorConfigUndefinedProviderConfig(t *testing.T) {
	mfa := MultiFactorConfig{
		ProviderConfigs: []*ProviderConfig{{}},
	}
	want := "\"ProviderConfig\" must be defined"
	if err := mfa.validate(); err.Error() != want {
		t.Errorf("MultiFactorConfig.validate(nil) = %v, want = %q", err, want)
	}
}

func TestMultiFactorConfigInvalidProviderConfigState(t *testing.T) {
	mfa := MultiFactorConfig{
		ProviderConfigs: []*ProviderConfig{{
			State: "invalid",
		}},
	}
	want := "\"ProviderConfig.State\" must be 'Enabled' or 'Disabled'"
	if err := mfa.validate(); err.Error() != want {
		t.Errorf("MultiFactorConfig.validate(nil) = %v, want = %q", err, want)
	}
}

func TestMultiFactorConfigNilTOTPProviderConfig(t *testing.T) {
	mfa := MultiFactorConfig{
		ProviderConfigs: []*ProviderConfig{{
			State:              Disabled,
			TOTPProviderConfig: nil,
		}},
	}
	want := "\"TOTPProviderConfig\" must be defined"
	if err := mfa.validate(); err.Error() != want {
		t.Errorf("MultiFactorConfig.validate(nil) = %v, want = %q", err, want)
	}
}

func TestMultiFactorConfigInvalidAdjacentIntervals(t *testing.T) {
	mfa := MultiFactorConfig{
		ProviderConfigs: []*ProviderConfig{{
			State: Disabled,
			TOTPProviderConfig: &TOTPProviderConfig{
				AdjacentIntervals: 11,
			},
		}},
	}
	want := "\"AdjacentIntervals\" must be an integer between 1 and 10 (inclusive)"
	if err := mfa.validate(); err.Error() != want {
		t.Errorf("MultiFactorConfig.validate(nil) = %v, want = %q", err, want)
	}
}
