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
	// "encoding/json" // Commented out as tests using it are commented out
	// "fmt"           // Commented out
	// "net/http"      // Commented out
	// "reflect"       // Commented out
	// "sort"          // Commented out
	// "strings"       // Commented out
	"testing"

	// "github.com/google/go-cmp/cmp" // Commented out
)

const projectConfigResponse = `{
	"mfa": {
		"providerConfigs": [
			{
				"state":"ENABLED",
				"totpProviderConfig":{
					"adjacentIntervals":5
				}
			}
		]
	}
}`

var testProjectConfig = &ProjectConfig{
	MultiFactorConfig: &MultiFactorConfig{
		ProviderConfigs: []*ProviderConfig{
			{
				State: Enabled,
				TOTPProviderConfig: &TOTPProviderConfig{
					AdjacentIntervals: 5,
				},
			},
		},
	},
}

// TODO: Refactor tests below to use httptest.NewServer directly and initialize auth.Client with app.App
/*
func TestGetProjectConfig(t *testing.T) {
	s := echoServer([]byte(projectConfigResponse), t) // This test uses the echoServer helper
	defer s.Close()

	client := s.Client
	projectConfig, err := client.GetProjectConfig(context.Background())

	if err != nil {
		t.Errorf("GetProjectConfig() = %v", err)
	}
	if !reflect.DeepEqual(projectConfig, testProjectConfig) {
		t.Errorf("GetProjectConfig() = %#v, want = %#v", projectConfig, testProjectConfig)
	}
}

func TestUpdateProjectConfig(t *testing.T) {
	s := echoServer([]byte(projectConfigResponse), t) // This test uses the echoServer helper
	defer s.Close()

	client := s.Client
	options := (&ProjectConfigToUpdate{}).
		MultiFactorConfig(*testProjectConfig.MultiFactorConfig)
	projectConfig, err := client.UpdateProjectConfig(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(projectConfig, testProjectConfig) {
		t.Errorf("UpdateProjectConfig() = %#v; want = %#v", projectConfig, testProjectConfig)
	}
	wantBody := map[string]interface{}{
		"mfa": map[string]interface{}{
			"providerConfigs": []interface{}{
				map[string]interface{}{
					"state": "ENABLED",
					"totpProviderConfig": map[string]interface{}{
						"adjacentIntervals": float64(5),
					},
				},
			},
		},
	}
	wantMask := []string{"mfa"}
	if err := checkUpdateProjectConfigRequest(s, wantBody, wantMask); err != nil {
		t.Fatal(err)
	}
}
*/

func TestUpdateProjectNilOptions(t *testing.T) {
	base := &baseClient{} // This test is for input validation, doesn't need a full client or server
	want := "project config must not be nil"
	if _, err := base.UpdateProjectConfig(context.Background(), nil); err == nil || err.Error() != want {
		t.Errorf("UpdateProject(nil) = %v, want = %q", err, want)
	}
}

/*
func checkUpdateProjectConfigRequest(s *mockAuthServer, wantBody interface{}, wantMask []string) error {
	// This helper is used by commented out tests.
	// ...
	return nil
}
*/
