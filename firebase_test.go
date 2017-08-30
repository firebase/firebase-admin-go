// Copyright 2017 Google Inc. All Rights Reserved.
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

package firebase

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2/google"

	"google.golang.org/api/transport"

	"encoding/json"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

func TestServiceAcctFile(t *testing.T) {
	app, err := NewApp(context.Background(), nil, option.WithCredentialsFile("testdata/service_account.json"))
	if err != nil {
		t.Fatal(err)
	}
	verifyServiceAcct(t, app)
}

func TestClientOptions(t *testing.T) {
	ts := initMockTokenServer()
	defer ts.Close()

	b, err := mockServiceAcct(ts.URL)
	config, err := google.JWTConfigFromJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	app, err := NewApp(ctx, nil, option.WithTokenSource(config.TokenSource(ctx)))
	if err != nil {
		t.Fatal(err)
	}

	var bearer string
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"output": "test"}`))
	}))
	defer service.Close()

	client, _, err := transport.NewHTTPClient(ctx, app.opts...)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Get(service.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Status: %d; want: 200", resp.StatusCode)
	}
	if bearer != "Bearer mock-token" {
		t.Errorf("Bearer token: %q; want: %q", bearer, "Bearer mock-token")
	}
}

func TestRefreshTokenFile(t *testing.T) {
	app, err := NewApp(context.Background(), nil, option.WithCredentialsFile("testdata/refresh_token.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(app.opts) != 2 {
		t.Errorf("Client opts: %d; want: 2", len(app.opts))
	}
	if app.ctx == nil {
		t.Error("Context: nil; want: ctx")
	}
}

func TestRefreshTokenFileWithConfig(t *testing.T) {
	config := &Config{ProjectID: "mock-project-id"}
	app, err := NewApp(context.Background(), config, option.WithCredentialsFile("testdata/refresh_token.json"))
	if err != nil {
		t.Fatal(err)
	}
	if app.projectID != "mock-project-id" {
		t.Errorf("Project ID: %q; want: mock-project-id", app.projectID)
	}
	if len(app.opts) != 2 {
		t.Errorf("Client opts: %d; want: 2", len(app.opts))
	}
	if app.ctx == nil {
		t.Error("Context: nil; want: ctx")
	}
}

func TestRefreshTokenWithEnvVar(t *testing.T) {
	varName := "GCLOUD_PROJECT"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, "mock-project-id"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

	app, err := NewApp(context.Background(), nil, option.WithCredentialsFile("testdata/refresh_token.json"))
	if err != nil {
		t.Fatal(err)
	}
	if app.projectID != "mock-project-id" {
		t.Errorf("Project ID: %q; want: mock-project-id", app.projectID)
	}
	if len(app.opts) != 2 {
		t.Errorf("Client opts: %d; want: 2", len(app.opts))
	}
	if app.ctx == nil {
		t.Error("Context: nil; want: ctx")
	}
}

func TestAppDefault(t *testing.T) {
	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, "testdata/service_account.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

	ctx := context.Background()
	app, err := NewApp(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(app.opts) != 1 {
		t.Errorf("Client opts: %d; want: 1", len(app.opts))
	}
	if app.ctx == nil {
		t.Error("Context: nil; want: ctx")
	}
}

func TestAppDefaultWithInvalidFile(t *testing.T) {
	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, "testdata/non_existing.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

	app, err := NewApp(context.Background(), nil)
	if app != nil || err == nil {
		t.Errorf("NewApp() = (%v, %v); want: (nil, error)", app, err)
	}
}

func TestInvalidCredentialFile(t *testing.T) {
	invalidFiles := []string{
		"testdata",
		"testdata/plain_text.txt",
	}

	ctx := context.Background()
	for _, tc := range invalidFiles {
		app, err := NewApp(ctx, nil, option.WithCredentialsFile(tc))
		if app != nil || err == nil {
			t.Errorf("NewApp(%q) = (%v, %v); want: (nil, error)", tc, app, err)
		}
	}
}

func TestAuth(t *testing.T) {
	app, err := NewApp(context.Background(), nil, option.WithCredentialsFile("testdata/service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	if c, err := app.Auth(); c == nil || err != nil {
		t.Errorf("Auth() = (%v, %v); want (auth, nil)", c, err)
	}
}

func TestCustomTokenSource(t *testing.T) {
	ctx := context.Background()
	ts := &testTokenSource{AccessToken: "mock-token-from-custom"}
	app, err := NewApp(ctx, nil, option.WithTokenSource(ts))
	if err != nil {
		t.Fatal(err)
	}

	client, _, err := transport.NewHTTPClient(ctx, app.opts...)
	if err != nil {
		t.Fatal(err)
	}

	var bearer string
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"output": "test"}`))
	}))
	defer service.Close()

	resp, err := client.Get(service.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Status: %d; want: 200", resp.StatusCode)
	}
	if bearer != "Bearer "+ts.AccessToken {
		t.Errorf("Bearer token: %q; want: %q", bearer, "Bearer "+ts.AccessToken)
	}
}

func TestVersion(t *testing.T) {
	segments := strings.Split(Version, ".")
	if len(segments) != 3 {
		t.Errorf("Incorrect number of segments: %d; want: 3", len(segments))
	}
	for _, segment := range segments {
		if _, err := strconv.Atoi(segment); err != nil {
			t.Errorf("Invalid segment in version number: %q; want integer", segment)
		}
	}
}

type testTokenSource struct {
	AccessToken string
	Expiry      time.Time
}

func (t *testTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: t.AccessToken,
		Expiry:      t.Expiry,
	}, nil
}

func verifyServiceAcct(t *testing.T, app *App) {
	// TODO: Compare creds JSON
	if app.projectID != "mock-project-id" {
		t.Errorf("Project ID: %q; want: %q", app.projectID, "mock-project-id")
	}
	if len(app.opts) != 2 {
		t.Errorf("Client opts: %d; want: 2", len(app.opts))
	}
	if app.ctx == nil {
		t.Error("Context: nil; want: ctx")
	}
}

// mockServiceAcct generates a service account configuration with the provided URL as the
// token_url value.
func mockServiceAcct(tokenURL string) ([]byte, error) {
	b, err := ioutil.ReadFile("testdata/service_account.json")
	if err != nil {
		return nil, err
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}
	parsed["token_uri"] = tokenURL
	return json.Marshal(parsed)
}

// initMockTokenServer starts a mock HTTP server that Apps can invoke during tests to obtain
// OAuth2 access tokens.
func initMockTokenServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"access_token": "mock-token",
			"scope": "user",
			"token_type": "bearer",
			"expires_in": 3600
		}`))
	}))
}
