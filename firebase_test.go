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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
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
}

func TestClientOptions(t *testing.T) {
	ts := initMockTokenServer()
	defer ts.Close()

	b, err := mockServiceAcct(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

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
}

func TestRefreshTokenWithEnvVar(t *testing.T) {
	verify := func(varName string) {
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
			t.Errorf("[env=%s] Project ID: %q; want: mock-project-id", varName, app.projectID)
		}
	}
	for _, varName := range []string{"GCLOUD_PROJECT", "GOOGLE_CLOUD_PROJECT"} {
		verify(varName)
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
}

func TestAppDefaultWithInvalidFile(t *testing.T) {
	current := os.Getenv(credEnvVar)

	if err := os.Setenv(credEnvVar, "testdata/non_existing.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(credEnvVar, current)

	app, err := NewApp(context.Background(), nil)
	if app == nil || err != nil {
		t.Fatalf("NewApp() = (%v, %v); want = (app, nil)", app, err)
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
		if app == nil || err != nil {
			t.Fatalf("NewApp() = (%v, %v); want = (app, nil)", app, err)
		}
	}
}

func TestExplicitNoAuth(t *testing.T) {
	ctx := context.Background()
	app, err := NewApp(ctx, nil, option.WithoutAuthentication())
	if app == nil || err != nil {
		t.Fatalf("NewApp() = (%v, %v); want = (app, nil)", app, err)
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

func TestDatabase(t *testing.T) {
	ctx := context.Background()
	conf := &Config{DatabaseURL: "https://mock-db.firebaseio.com"}
	app, err := NewApp(ctx, conf, option.WithCredentialsFile("testdata/service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	if app.authOverride == nil || len(app.authOverride) != 0 {
		t.Errorf("AuthOverrides = %v; want = empty map", app.authOverride)
	}
	if c, err := app.Database(ctx); c == nil || err != nil {
		t.Errorf("Database() = (%v, %v); want (db, nil)", c, err)
	}
	url := "https://other-mock-db.firebaseio.com"
	if c, err := app.DatabaseWithURL(ctx, url); c == nil || err != nil {
		t.Errorf("Database() = (%v, %v); want (db, nil)", c, err)
	}
}

func TestDatabaseAuthOverrides(t *testing.T) {
	cases := []map[string]interface{}{
		nil,
		{},
		{"uid": "user1"},
	}
	for _, tc := range cases {
		ctx := context.Background()
		conf := &Config{
			AuthOverride: &tc,
			DatabaseURL:  "https://mock-db.firebaseio.com",
		}
		app, err := NewApp(ctx, conf, option.WithCredentialsFile("testdata/service_account.json"))
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(app.authOverride, tc) {
			t.Errorf("AuthOverrides = %v; want = %v", app.authOverride, tc)
		}
		if c, err := app.Database(ctx); c == nil || err != nil {
			t.Errorf("Database() = (%v, %v); want (db, nil)", c, err)
		}
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
	verify := func(varName string) {
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
			t.Errorf("[env=%s] Firestore() = (%v, %v); want (auth, nil)", varName, c, err)
		}
	}
	for _, varName := range []string{"GCLOUD_PROJECT", "GOOGLE_CLOUD_PROJECT"} {
		verify(varName)
	}
}

func TestFirestoreWithNoProjectID(t *testing.T) {
	unsetVariable := func(varName string) string {
		current := os.Getenv(varName)
		if err := os.Setenv(varName, ""); err != nil {
			t.Fatal(err)
		}
		return current
	}

	for _, varName := range []string{"GCLOUD_PROJECT", "GOOGLE_CLOUD_PROJECT"} {
		if current := unsetVariable(varName); current != "" {
			defer os.Setenv(varName, current)
		}
	}

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

func TestMessaging(t *testing.T) {
	ctx := context.Background()
	app, err := NewApp(ctx, nil, option.WithCredentialsFile("testdata/service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	if c, err := app.Messaging(ctx); c == nil || err != nil {
		t.Errorf("Messaging() = (%v, %v); want (iid, nil)", c, err)
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
	var nullMap map[string]interface{}
	uidMap := map[string]interface{}{"uid": "test"}
	tests := []struct {
		name          string
		optionsConfig string
		initOptions   *Config
		wantOptions   *Config
	}{
		{
			"<env=nil,opts=nil>",
			"",
			nil,
			&Config{ProjectID: "mock-project-id"}, // from default creds here and below.
		},
		{
			"<env=file,opts=nil>",
			"testdata/firebase_config.json",
			nil,
			&Config{
				DatabaseURL:   "https://auto-init.database.url",
				ProjectID:     "auto-init-project-id",
				StorageBucket: "auto-init.storage.bucket",
			},
		},
		{
			"<env=string,opts=nil>",
			`{
				"databaseURL": "https://auto-init.database.url",
				"projectId": "auto-init-project-id",
				"storageBucket": "auto-init.storage.bucket"
			  }`,
			nil,
			&Config{
				DatabaseURL:   "https://auto-init.database.url",
				ProjectID:     "auto-init-project-id",
				StorageBucket: "auto-init.storage.bucket",
			},
		},
		{
			"<env=file_missing_fields,opts=nil>",
			"testdata/firebase_config_partial.json",
			nil,
			&Config{ProjectID: "auto-init-project-id"},
		},
		{
			"<env=string_missing_fields,opts=nil>",
			`{"projectId": "auto-init-project-id"}`,
			nil,
			&Config{ProjectID: "auto-init-project-id"},
		},
		{
			"<env=file,opts=non-empty>",
			"testdata/firebase_config_partial.json",
			&Config{StorageBucket: "sb1-mock"},
			&Config{
				ProjectID:     "mock-project-id",
				StorageBucket: "sb1-mock",
			},
		}, {
			"<env=string,opts=non-empty>",
			`{"projectId": "auto-init-project-id"}`,
			&Config{StorageBucket: "sb1-mock"},
			&Config{
				ProjectID:     "mock-project-id", // from default creds
				StorageBucket: "sb1-mock",
			},
		},
		{
			"<env=file,opts=empty>",
			"testdata/firebase_config_partial.json",
			&Config{},
			&Config{ProjectID: "mock-project-id"},
		},
		{
			"<env=string,opts=empty>",
			`{"projectId": "auto-init-project-id"}`,
			&Config{},
			&Config{ProjectID: "mock-project-id"},
		},
		{
			"<env=file_unknown_key,opts=nil>",
			"testdata/firebase_config_invalid_key.json",
			nil,
			&Config{
				ProjectID:     "mock-project-id", // from default creds
				StorageBucket: "auto-init.storage.bucket",
			},
		},
		{
			"<env=string_unknown_key,opts=nil>",
			`{
				"obviously_bad_key": "mock-project-id",
				"storageBucket": "auto-init.storage.bucket"
			}`,
			nil,
			&Config{
				ProjectID:     "mock-project-id",
				StorageBucket: "auto-init.storage.bucket",
			},
		},
		{
			"<env=string_null_auth_override,opts=nil>",
			`{
				"databaseURL": "https://auto-init.database.url",
				"projectId": "auto-init-project-id",
				"databaseAuthVariableOverride": null
			}`,
			nil,
			&Config{
				DatabaseURL:  "https://auto-init.database.url",
				ProjectID:    "auto-init-project-id",
				AuthOverride: &nullMap,
			},
		},
		{
			"<env=string_auth_override,opts=nil>",
			`{
				"databaseURL": "https://auto-init.database.url",
				"projectId": "auto-init-project-id",
				"databaseAuthVariableOverride": {"uid": "test"}
			}`,
			nil,
			&Config{
				DatabaseURL:  "https://auto-init.database.url",
				ProjectID:    "auto-init-project-id",
				AuthOverride: &uidMap,
			},
		},
	}

	credOld := overwriteEnv(credEnvVar, "testdata/service_account.json")
	defer reinstateEnv(credEnvVar, credOld)

	for _, test := range tests {
		t.Run(fmt.Sprintf("NewApp(%s)", test.name), func(t *testing.T) {
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
			"NonexistingFile",
			"testdata/no_such_file.json",
			"open testdata/no_such_file.json: no such file or directory",
		},
		{
			"InvalidJSON",
			"testdata/firebase_config_invalid.json",
			"invalid character 'b' looking for beginning of value",
		},
		{
			"EmptyFile",
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
				t.Errorf("%s got error = %s; want = %s", test.name, err, test.wantError)
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
	if got.dbURL != want.DatabaseURL {
		t.Errorf("app.dbURL = %q; want = %q", got.dbURL, want.DatabaseURL)
	}
	if want.AuthOverride != nil {
		if !reflect.DeepEqual(got.authOverride, *want.AuthOverride) {
			t.Errorf("app.ao = %#v; want = %#v", got.authOverride, *want.AuthOverride)
		}
	} else if !reflect.DeepEqual(got.authOverride, defaultAuthOverrides) {
		t.Errorf("app.ao = %#v; want = nil", got.authOverride)
	}
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

// reinstateEnv restores the environment variable, will usually be used deferred with overwriteEnv.
func reinstateEnv(varName, oldVal string) {
	if len(varName) > 0 {
		os.Setenv(varName, oldVal)
	} else {
		os.Unsetenv(varName)
	}
}
