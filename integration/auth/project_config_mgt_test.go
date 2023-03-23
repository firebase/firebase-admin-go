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
	"context"
	"reflect"
	"testing"

	"firebase.google.com/go/v4/auth"
)

func TestProjectConfig(t *testing.T) {
	mfaObject := &auth.MultiFactorConfig{
		ProviderConfigs: []*auth.ProviderConfig{
			{
				State: auth.Enabled,
				TOTPProviderConfig: &auth.TOTPProviderConfig{
					AdjacentIntervals: 5,
				},
			},
		},
	}
	want := &auth.ProjectConfig{
		MultiFactorConfig: mfaObject,
	}
	t.Run("UpdateProjectConfig()", func(t *testing.T) {
		mfaConfigReq := *want.MultiFactorConfig
		req := (&auth.ProjectConfigToUpdate{}).
			MultiFactorConfig(mfaConfigReq)
		projectConfig, err := client.UpdateProjectConfig(context.Background(), req)
		if err != nil {
			t.Fatalf("UpdateProjectConfig() = %v", err)
		}
		if !reflect.DeepEqual(projectConfig, want) {
			t.Errorf("UpdateProjectConfig() = %#v; want = %#v", projectConfig, want)
		}
	})

	t.Run("GetProjectConfig()", func(t *testing.T) {
		project, err := client.GetProjectConfig(context.Background())
		if err != nil {
			t.Fatalf("GetProjectConfig() = %v", err)
		}

		if !reflect.DeepEqual(project, want) {
			t.Errorf("GetProjectConfig() = %v; want = %#v", project, want)
		}
	})
}
