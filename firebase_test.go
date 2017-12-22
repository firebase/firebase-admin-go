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
	"log"
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

func TestMain(m *testing.M) {
	// This isolates the tests from a possiblity that the
	// default config env variable is set to a valid file containing the
	// wanted default config
	configOld := overwriteEnv(FirebaseEnvName, "")
	defer reinstateEnv(FirebaseEnvName, configOld)
	os.Exit(m.Run())
}

func TestServiceAcctFile(t *testing.T) {
	app, err := NewApp(context.Background(), nil, option.WithCredentialsFile("testdata/service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	if app.projectID != "mock-project-id" {
		t.Errorf("Project ID: %q; want: %q", app.projectID, "mock-project-id")
	}
	if len(app.opts) != 2 {
		t.Errorf("Client opts: %d; want: 2", len(app.opts))
	}
	if app.creds == nil {
		t.Error("Credentials: nil; want creds")
	} else if len(app.creds.JSON) == 0 {
		t.Error("JSON: empty; want; non-empty")
	}
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
	if app.creds == nil {
		t.Error("Credentials: nil; want creds")
	} else if len(app.creds.JSON) == 0 {
		t.Error("JSON: empty; want; non-empty")
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
	if app.creds == nil {
		t.Error("Credentials: nil; want creds")
	} else if len(app.creds.JSON) == 0 {
		t.Error("JSON: empty; want; non-empty")
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
	if app.creds == nil {
		t.Error("Credentials: nil; want creds")
	} else if len(app.creds.JSON) == 0 {
		t.Error("JSON: empty; want; non-empty")
	}
}

func TestAppDefault(t *testing.T) {
	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, "testdata/service_account.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

	app, err := NewApp(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(app.opts) != 1 {
		t.Errorf("Client opts: %d; want: 1", len(app.opts))
	}
	if app.creds == nil {
		t.Error("Credentials: nil; want creds")
	} else if len(app.creds.JSON) == 0 {
		t.Error("JSON: empty; want; non-empty")
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
	ctx := context.Background()
	app, err := NewApp(ctx, nil, option.WithCredentialsFile("testdata/service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	if c, err := app.Auth(ctx); c == nil || err != nil {
		t.Errorf("Auth() = (%v, %v); want (auth, nil)", c, err)
	}
}

func TestStorage(t *testing.T) {
	ctx := context.Background()
	app, err := NewApp(ctx, nil, option.WithCredentialsFile("testdata/service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	if c, err := app.Storage(ctx); c == nil || err != nil {
		t.Errorf("Storage() = (%v, %v); want (auth, nil)", c, err)
	}
}

func TestFirestore(t *testing.T) {
	ctx := context.Background()
	app, err := NewApp(ctx, nil, option.WithCredentialsFile("testdata/service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	if c, err := app.Firestore(ctx); c == nil || err != nil {
		t.Errorf("Firestore() = (%v, %v); want (auth, nil)", c, err)
	}
}

func TestFirestoreWithProjectID(t *testing.T) {
	varName := "GCLOUD_PROJECT"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, ""); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

	ctx := context.Background()
	config := &Config{ProjectID: "project-id"}
	app, err := NewApp(ctx, config, option.WithCredentialsFile("testdata/refresh_token.json"))
	if err != nil {
		t.Fatal(err)
	}

	if c, err := app.Firestore(ctx); c == nil || err != nil {
		t.Errorf("Firestore() = (%v, %v); want (auth, nil)", c, err)
	}
}

func TestFirestoreWithNoProjectID(t *testing.T) {
	varName := "GCLOUD_PROJECT"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, ""); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

	ctx := context.Background()
	app, err := NewApp(ctx, nil, option.WithCredentialsFile("testdata/refresh_token.json"))
	if err != nil {
		t.Fatal(err)
	}

	if c, err := app.Firestore(ctx); c != nil || err == nil {
		t.Errorf("Firestore() = (%v, %v); want (nil, error)", c, err)
	}
}

func TestInstanceID(t *testing.T) {
	ctx := context.Background()
	app, err := NewApp(ctx, nil, option.WithCredentialsFile("testdata/service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	if c, err := app.InstanceID(ctx); c == nil || err != nil {
		t.Errorf("InstanceID() = (%v, %v); want (iid, nil)", c, err)
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

func TestAutoInitNoEnvVar(t *testing.T) {
	configOld := overwriteEnv(FirebaseEnvName, "")
	defer reinstateEnv(FirebaseEnvName, configOld)

	err := os.Unsetenv(FirebaseEnvName)
	if err != nil {
		t.Fatal(err)
	}

	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	credOld := overwriteEnv(varName, "testdata/service_account.json")
	defer reinstateEnv(varName, credOld)

	app, err := NewApp(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := &Config{
		ProjectID:     "mock-project-id", // from default credentials
		StorageBucket: "",
	}
	compareConfig(app, want, t)
}

func TestAutoInitPartialOverride(t *testing.T) {
	configOld := overwriteEnv(FirebaseEnvName, "testdata/firebase_config_partial.json")
	defer reinstateEnv(FirebaseEnvName, configOld)

	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	credOld := overwriteEnv(varName, "testdata/service_account.json")
	defer reinstateEnv(varName, credOld)

	app, err := NewApp(context.Background(),
		&Config{
			StorageBucket: "sb1-mock",
		})
	if err != nil {
		t.Fatal(err)
	}

	want := &Config{
		ProjectID:     "hipster-chat-mock",
		StorageBucket: "sb1-mock",
	}
	compareConfig(app, want, t)
}

func TestAutoInitPartialOverrideWithoutEnv(t *testing.T) {
	configOld := overwriteEnv(FirebaseEnvName, "testdata/firebase_config_partial.json")
	defer reinstateEnv(FirebaseEnvName, configOld)
	os.Unsetenv(FirebaseEnvName)

	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	credOld := overwriteEnv(varName, "testdata/service_account.json")
	defer reinstateEnv(varName, credOld)

	app, err := NewApp(context.Background(),
		&Config{
			ProjectID: "pid1-mock",
		})
	if err != nil {
		t.Fatal(err)
	}

	want := &Config{
		ProjectID: "pid1-mock",
	}
	compareConfig(app, want, t)
}
func TestAutoInit(t *testing.T) {
	tests := []struct {
		name         string
		confFilename string
		initOptions  *Config
		wantOptions  *Config
	}{
		{
			"no environment var, no options",
			"",
			nil,
			&Config{},
		}, {
			"env var set, options not",
			"testdata/firebase_config.json",
			nil,
			&Config{
				ProjectID:     "hipster-chat-mock",
				StorageBucket: "hipster-chat.appspot.mock",
			},
		},
	}

	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	credOld := overwriteEnv(varName, "testdata/service_account.json")
	defer reinstateEnv(varName, credOld)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			overwriteEnv(FirebaseEnvName, test.confFilename)
			app, err := NewApp(context.Background(), test.initOptions)
			if err != nil {
				t.Error(err)
			} else {
				compareConfig(app, test.wantOptions, t)
			}
		})
	}
}

func TestAutoInitBadFiles(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		wantError string
	}{
		{
			"nonexistant file",
			"testdata/no_such_file.json",
			"open testdata/no_such_file.json: no such file or directory",
		}, {
			"JSON with bad key",
			"testdata/firebase_config_bad_key.json",
			"unexpected field project1d in JSON config file",
		}, {
			"invalid JSON",
			"testdata/firebase_config_bad.json",
			"invalid character 'b' looking for beginning of value",
		}, {
			"empty file",
			"testdata/firebase_config_empty.json",
			"unexpected end of JSON input",
		},
	}
	configOld := overwriteEnv(FirebaseEnvName, "")
	defer reinstateEnv(FirebaseEnvName, configOld)

	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	credOld := overwriteEnv(varName, "testdata/service_account.json")
	defer reinstateEnv(varName, credOld)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			overwriteEnv(FirebaseEnvName, test.filename)
			_, err := NewApp(context.Background(), &Config{})
			if err == nil || err.Error() != test.wantError {
				t.Errorf("got error = %s; want = %s", err, test.wantError)
			}
		})
	}
}

func TestAutoInitNilOptionsNoConfig(t *testing.T) {
	configOld := overwriteEnv(FirebaseEnvName, "")
	defer reinstateEnv(FirebaseEnvName, configOld)
	os.Unsetenv(FirebaseEnvName)

	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	credOld := overwriteEnv(varName, "testdata/service_account.json")
	defer reinstateEnv(varName, credOld)

	app, err := NewApp(context.Background(), nil)
	if err != nil {
		t.Errorf("got error = %s; want nil", err)
	}

	want := &Config{
		ProjectID:     "mock-project-id", // from default credentials
		StorageBucket: "",
	}
	compareConfig(app, want, t)
}

func TestAutoInitNilOptionsWithConfig(t *testing.T) {
	configOld := overwriteEnv(FirebaseEnvName, "testdata/firebase_config.json")
	defer reinstateEnv(FirebaseEnvName, configOld)

	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	credOld := overwriteEnv(varName, "testdata/service_account.json")
	defer reinstateEnv(varName, credOld)

	app, err := NewApp(context.Background(), nil)
	if err != nil {
		t.Errorf("got error = %s; want nil", err)
	}

	want := &Config{
		ProjectID:     "hipster-chat-mock",
		StorageBucket: "hipster-chat.appspot.mock",
	}
	compareConfig(app, want, t)
	//for _, pair := range os.Environ() {
	//	fmt.Println(pair)
	//}
	//	t.Error("fff")
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

func overwriteEnv(varName, newVal string) string {
	oldVal := os.Getenv(varName)
	if newVal == "" {
		if err := os.Unsetenv(varName); err != nil {
			log.Fatal(err)
		}
	} else if err := os.Setenv(varName, newVal); err != nil {
		log.Fatal(err)
	}
	return oldVal
}

func reinstateEnv(varName, oldVal string) {
	if len(varName) > 0 {
		os.Setenv(varName, oldVal)
	} else {
		os.Unsetenv(varName)
	}
}

func compareConfig(got *App, want *Config, t *testing.T) {
	if got.projectID != want.ProjectID {
		t.Errorf("app.projectID = %q; want = %q", got.projectID, want.ProjectID)
	}
	if got.storageBucket != want.StorageBucket {
		t.Errorf("app.storageBucket = %q; want = %q", got.storageBucket, want.StorageBucket)
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
