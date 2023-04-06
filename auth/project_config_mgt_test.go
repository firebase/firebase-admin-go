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
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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

func TestGetProjectConfig(t *testing.T) {
	s := echoServer([]byte(projectConfigResponse), t)
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
	s := echoServer([]byte(projectConfigResponse), t)
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

func TestUpdateProjectNilOptions(t *testing.T) {
	base := &baseClient{}
	want := "project config must not be nil"
	if _, err := base.UpdateProjectConfig(context.Background(), nil); err == nil || err.Error() != want {
		t.Errorf("UpdateProject(nil) = %v, want = %q", err, want)
	}
}

func checkUpdateProjectConfigRequest(s *mockAuthServer, wantBody interface{}, wantMask []string) error {
	req := s.Req[0]
	if req.Method != http.MethodPatch {
		return fmt.Errorf("UpdateProjectConfig() Method = %q; want = %q", req.Method, http.MethodPatch)
	}

	wantURL := "/projects/mock-project-id/config"
	if req.URL.Path != wantURL {
		return fmt.Errorf("UpdateProjectConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}

	queryParam := req.URL.Query().Get("updateMask")
	mask := strings.Split(queryParam, ",")
	sort.Strings(mask)
	if !reflect.DeepEqual(mask, wantMask) {
		return fmt.Errorf("UpdateProjectConfig() Query = %#v; want = %#v", mask, wantMask)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(s.Rbody, &body); err != nil {
		return err
	}

	if diff := cmp.Diff(body, wantBody); diff != "" {
		fmt.Printf("UpdateProjectConfig() diff = %s", diff)
	}

	if !reflect.DeepEqual(body, wantBody) {
		return fmt.Errorf("UpdateProjectConfig() Body = %#v; want = %#v", body, wantBody)
	}

	return nil
}
