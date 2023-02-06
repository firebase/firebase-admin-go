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

const projectResponse = `{
	"mfa": {
		"state":"ENABLED",
		"enabledProviders": ["PHONE_SMS"],
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

var testProject = &ProjectConfig{
	MultiFactorConfig: &MultiFactorConfig{
		State:            "ENABLED",
		EnabledProviders: []string{"PHONE_SMS"},
		ProviderConfigs: []*ProviderConfig{
			{
				State: "ENABLED",
				TotpProviderConfig: &TotpMfaProviderConfig{
					AdjacentIntervals: 5,
				},
			},
		},
	},
}

func TestUpdatProject(t *testing.T) {
	s := echoServer([]byte(projectResponse), t)
	defer s.Close()

	client := s.Client
	options := (&ProjectToUpdate{}).
		MultiFactorConfig(*testProject.MultiFactorConfig)
	project, err := client.UpdateProjectConfig(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(project, testProject) {
		t.Errorf("UpdateProjectConfig() = %#v; want = %#v", project, testProject)
	}
	var wantEnabledProviders []interface{}
	for _, p := range testProject.MultiFactorConfig.EnabledProviders {
		wantEnabledProviders = append(wantEnabledProviders, p)
	}
	wantProviderConfigs := map[string]interface{}{
		"state": testProject.MultiFactorConfig.ProviderConfigs[0].State,
		"totpProviderConfig": map[string]interface{}{
			"adjacentIntervals": float64(testProject.MultiFactorConfig.ProviderConfigs[0].TotpProviderConfig.AdjacentIntervals),
		},
	}
	wantBody := map[string]interface{}{
		"mfa": map[string]interface{}{
			"state":            testProject.MultiFactorConfig.State,
			"enabledProviders": wantEnabledProviders,
			"providerConfigs":  []interface{}{wantProviderConfigs},
		},
	}
	wantMask := []string{"mfa"}
	if err := checkUpdateProjectConfigRequest(s, wantBody, wantMask); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateProjectNilOptions(t *testing.T) {
	base := &baseClient{}
	want := "project must not be nil"
	if _, err := base.UpdateProjectConfig(context.Background(), nil); err == nil || err.Error() != want {
		t.Errorf("UpdateProject(nil) = %v, want = %q", err, want)
	}
}

func checkUpdateProjectConfigRequest(s *mockAuthServer, wantBody interface{}, wantMask []string) error {
	req := s.Req[0]
	if req.Method != http.MethodPatch {
		return fmt.Errorf("UpdateProjectConfig() Method = %q; want = %q", req.Method, http.MethodPatch)
	}

	wantURL := "/"
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
