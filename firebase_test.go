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

const credEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"

func TestMain(m *testing.M) {
	// This isolates the tests from a possiblity that the default config env
	// variable is set to a valid file containing the wanted default config,
	// but we the test is not expecting it.
	configOld := overwriteEnv(firebaseEnvName, "")
	defer reinstateEnv(firebaseEnvName, configOld)
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
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status: %d; want: %d", resp.StatusCode, http.StatusOK)
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
	current := os.Getenv(credEnvVar)

	if err := os.Setenv(credEnvVar, "testdata/service_account.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(credEnvVar, current)

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
	current := os.Getenv(credEnvVar)

	if err := os.Setenv(credEnvVar, "testdata/non_existing.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(credEnvVar, current)

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
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status: %d; want: %d", resp.StatusCode, http.StatusOK)
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
func TestAutoInit(t *testing.T) {
	tests := []struct {
		name          string
		optionsConfig string
		initOptions   *Config
		wantOptions   *Config
	}{
		{
			"No environment variable, no explicit options",
			"",
			nil,
			&Config{ProjectID: "mock-project-id"}, // from default creds here and below.
		}, {
			"Environment variable set to file, no explicit options",
			"testdata/firebase_config.json",
			nil,
			&Config{
				ProjectID:     "hipster-chat-mock",
				StorageBucket: "hipster-chat.appspot.mock",
			},
		}, {
			"Environment variable set to string, no explicit options",
			`{
				"projectId": "hipster-chat-mock",
				"storageBucket": "hipster-chat.appspot.mock"
			  }`,
			nil,
			&Config{
				ProjectID:     "hipster-chat-mock",
				StorageBucket: "hipster-chat.appspot.mock",
			},
		}, {
			"Environment variable set to file with some values missing, no explicit options",
			"testdata/firebase_config_partial.json",
			nil,
			&Config{ProjectID: "hipster-chat-mock"},
		}, {
			"Environment variable set to string with some values missing, no explicit options",
			`{"projectId": "hipster-chat-mock"}`,
			nil,
			&Config{ProjectID: "hipster-chat-mock"},
		}, {
			"Environment variable set to file which is ignored as some explicit options are passed",
			"testdata/firebase_config_partial.json",
			&Config{StorageBucket: "sb1-mock"},
			&Config{
				ProjectID:     "mock-project-id",
				StorageBucket: "sb1-mock",
			},
		}, {
			"Environment variable set to string which is ignored as some explicit options are passed",
			`{"projectId": "hipster-chat-mock"}`,
			&Config{StorageBucket: "sb1-mock"},
			&Config{
				ProjectID:     "mock-project-id",
				StorageBucket: "sb1-mock",
			},
		}, {
			"Environment variable set to file which is ignored as options are explicitly empty",
			"testdata/firebase_config_partial.json",
			&Config{},
			&Config{ProjectID: "mock-project-id"},
		}, {
			"Environment variable set to file with an unknown key which is ignored, no explicit options",
			"testdata/firebase_config_invalid_key.json",
			nil,
			&Config{
				ProjectID:     "mock-project-id", // from default creds
				StorageBucket: "hipster-chat.appspot.mock",
			},
		}, {
			"Environment variable set to string with an unknown key which is ignored, no explicit options",
			`{
				"obviously_bad_key": "hipster-chat-mock",
				"storageBucket": "hipster-chat.appspot.mock"
			}`,
			nil,
			&Config{
				ProjectID:     "mock-project-id",
				StorageBucket: "hipster-chat.appspot.mock",
			},
		},
	}

	credOld := overwriteEnv(credEnvVar, "testdata/service_account.json")
	defer reinstateEnv(credEnvVar, credOld)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			overwriteEnv(firebaseEnvName, test.optionsConfig)
			app, err := NewApp(context.Background(), test.initOptions)
			if err != nil {
				t.Error(err)
			} else {
				compareConfig(app, test.wantOptions, t)
			}
		})
	}
}

func TestAutoInitInvalidFiles(t *testing.T) {
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
			"invalid JSON",
			"testdata/firebase_config_invalid.json",
			"invalid character 'b' looking for beginning of value",
		}, {
			"empty file",
			"testdata/firebase_config_empty.json",
			"unexpected end of JSON input",
		},
	}
	credOld := overwriteEnv(credEnvVar, "testdata/service_account.json")
	defer reinstateEnv(credEnvVar, credOld)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			overwriteEnv(firebaseEnvName, test.filename)
			_, err := NewApp(context.Background(), nil)
			if err == nil || err.Error() != test.wantError {
				t.Errorf("got error = %s; want = %s", err, test.wantError)
			}
		})
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

// overwriteEnv overwrites env variables, used in testsing.
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

// reinstateEnv restores the enviornment variable, will usually be used deferred with overwriteEnv.
func reinstateEnv(varName, oldVal string) {
	if len(varName) > 0 {
		os.Setenv(varName, oldVal)
	} else {
		os.Unsetenv(varName)
	}
}
