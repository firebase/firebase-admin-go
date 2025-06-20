// Copyright 2018 Google Inc. All Rights Reserved.
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

package db

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"testing"

	"firebase.google.com/go/v4/app" // Import app package
	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
)

const (
	testProjectID         = "test-project" // A project ID for app initialization
	testURL               = "https://test-db.firebaseio.com"
	testEmulatorNamespace = "test-db"
	testEmulatorBaseURL   = "http://localhost:9000"
	testEmulatorURL       = "localhost:9000?ns=test-db" // This form is for FIREBASE_DATABASE_EMULATOR_HOST
	defaultMaxRetries     = 1
)

var (
	aoClient          *Client
	client            *Client
	testAuthOverrides string
	testref           *Ref
	testUserAgent     string

	// testOpts are now app options
	testAppOpts = []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "mock-token"}),
	}
)

// Helper to create a new app for db tests
func newTestDBApp(ctx context.Context, dbURL string, authOverride map[string]interface{}) *app.App {
	conf := &app.Config{
		DatabaseURL: dbURL,
		ProjectID:   testProjectID, // Provide a consistent project ID
	}
	if authOverride != nil {
		conf.AuthOverride = &authOverride
	}

	// Mimic firebase.NewApp logic for creating an app.App
	allOpts := []option.ClientOption{option.WithScopes(internal.FirebaseScopes...)}
	allOpts = append(allOpts, testAppOpts...) // Add common test options

	appInstance, err := app.New(ctx, conf, allOpts...)
	if err != nil {
		log.Fatalf("Failed to create test app for DB: %v", err)
	}
	// For SDKVersion, we rely on appInstance.SDKVersion() which is hardcoded to 4.16.1 for now.
	// If a dynamic version was needed for tests, app.App would need a way to set it, or tests mock it.
	return appInstance
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error

	// Initialize default client
	defaultApp := newTestDBApp(ctx, testURL, map[string]interface{}{}) // Empty auth override for default client
	client, err = NewClient(ctx, defaultApp) // Pass app and rely on its DatabaseURL
	if err != nil {
		log.Fatalln(err)
	}
	retryConfig := client.hc.RetryConfig
	retryConfig.MaxRetries = defaultMaxRetries
	retryConfig.ExpBackoffFactor = 0

	// Initialize client with auth overrides
	ao := map[string]interface{}{"uid": "user1"}
	appWithAO := newTestDBApp(ctx, testURL, ao)
	aoClient, err = NewClient(ctx, appWithAO) // Pass app and rely on its DatabaseURL & AuthOverride
	if err != nil {
		log.Fatalln(err)
	}

	b, err := json.Marshal(ao)
	if err != nil {
		log.Fatalln(err)
	}
	testAuthOverrides = string(b)

	testref = client.NewRef("peter")
	// testUserAgent will use the SDKVersion from the appInstance used to create the 'client'
	// If client was created from defaultApp, it uses defaultApp.SDKVersion()
	testUserAgent = fmt.Sprintf(userAgentFormat, defaultApp.SDKVersion(), runtime.Version())
	os.Exit(m.Run())
}

func TestNewClient(t *testing.T) {
	cases := []*struct {
		Name              string
		AppDBURL          string // URL configured in app.Config
		ArgDBURL          string // URL passed as argument to NewClient
		EnvEmulatorURL    string
		ExpectedBaseURL   string
		ExpectedNamespace string
		ExpectError       bool
	}{
		{Name: "prod url from app", AppDBURL: testURL, ExpectedBaseURL: testURL, ExpectedNamespace: ""},
		{Name: "prod url from arg", ArgDBURL: testURL, ExpectedBaseURL: testURL, ExpectedNamespace: ""},
		{Name: "emulator from app", AppDBURL: testEmulatorBaseURL + "/?ns=" + testEmulatorNamespace, ExpectedBaseURL: testEmulatorBaseURL, ExpectedNamespace: testEmulatorNamespace},
		{Name: "emulator from arg", ArgDBURL: testEmulatorBaseURL + "/?ns=" + testEmulatorNamespace, ExpectedBaseURL: testEmulatorBaseURL, ExpectedNamespace: testEmulatorNamespace},
		{Name: "emulator from env", EnvEmulatorURL: testEmulatorURL, AppDBURL: "https://should_be_overridden.firebaseio.com", ExpectedBaseURL: testEmulatorBaseURL, ExpectedNamespace: testEmulatorNamespace},
		{Name: "emulator from env takes precedence over arg", EnvEmulatorURL: testEmulatorURL, ArgDBURL: "http://another-emulator:9000/?ns=other", ExpectedBaseURL: testEmulatorBaseURL, ExpectedNamespace: testEmulatorNamespace},
		{Name: "emulator from arg missing ns", ArgDBURL: "http://localhost:9000", ExpectError: true}, // parseEmulatorHost expects namespace
		{Name: "emulator from app missing ns", AppDBURL: "http://localhost:9000", ExpectError: true},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			if tc.EnvEmulatorURL != "" {
				originalEnv := os.Getenv(emulatorDatabaseEnvVar)
				os.Setenv(emulatorDatabaseEnvVar, tc.EnvEmulatorURL)
				defer os.Setenv(emulatorDatabaseEnvVar, originalEnv)
			} else {
				// Ensure env var is not set if not part of test case
				originalEnv := os.Getenv(emulatorDatabaseEnvVar)
				os.Unsetenv(emulatorDatabaseEnvVar)
				defer os.Setenv(emulatorDatabaseEnvVar, originalEnv)
			}

			appInstance := newTestDBApp(ctx, tc.AppDBURL, nil) // AuthOverride not relevant for this test part

			var c *Client
			var err error
			if tc.ArgDBURL != "" {
				c, err = NewClient(ctx, appInstance, tc.ArgDBURL)
			} else {
				c, err = NewClient(ctx, appInstance)
			}

			if err != nil {
				if tc.ExpectError {
					return // Expected error
				}
				t.Fatalf("NewClient() error = %v; want nil", err)
			}
			if tc.ExpectError {
				t.Fatalf("NewClient() error = nil; want error")
			}

			if c.dbURLConfig.BaseURL != tc.ExpectedBaseURL {
				t.Errorf("NewClient().dbURLConfig.BaseURL = %q; want = %q", c.dbURLConfig.BaseURL, tc.ExpectedBaseURL)
			}
			if c.dbURLConfig.Namespace != tc.ExpectedNamespace {
				t.Errorf("NewClient().dbURLConfig.Namespace = %q; want = %q", c.dbURLConfig.Namespace, tc.ExpectedNamespace)
			}
			if c.hc == nil {
				t.Errorf("NewClient().hc = nil; want non-nil")
			}
			// Auth override check is separate, ensure it's default here (empty for nil app authOverride)
			expectedAuthOverride := ""
			if appInstance.AuthOverride() != nil { // if test app somehow got a default override
				b, _ := json.Marshal(appInstance.AuthOverride())
				expectedAuthOverride = string(b)
			}
			if c.authOverride != expectedAuthOverride {
				t.Errorf("NewClient().authOverride = %q; want default (empty or from app default if any) %q", c.authOverride, expectedAuthOverride)
			}
		})
	}
}

func TestNewClientAuthOverrides(t *testing.T) {
	cases := []*struct {
		Name              string
		AppAuthOverride   map[string]interface{}
		AppDBURL          string
		ArgDBURL          string // URL passed to NewClient
		ExpectedBaseURL   string
		ExpectedNamespace string
	}{
		{Name: "prod - app override", AppAuthOverride: map[string]interface{}{"uid": "user1"}, AppDBURL: testURL, ExpectedBaseURL: testURL, ExpectedNamespace: ""},
		{Name: "prod - no override", AppAuthOverride: nil, AppDBURL: testURL, ExpectedBaseURL: testURL, ExpectedNamespace: ""},
		{Name: "emulator - app override", AppAuthOverride: map[string]interface{}{"uid": "user2"}, AppDBURL: testEmulatorBaseURL + "/?ns=" + testEmulatorNamespace, ExpectedBaseURL: testEmulatorBaseURL, ExpectedNamespace: testEmulatorNamespace},
		{Name: "emulator - arg URL with app override", AppAuthOverride: map[string]interface{}{"uid": "user3"}, ArgDBURL: testEmulatorBaseURL + "/?ns=" + testEmulatorNamespace, ExpectedBaseURL: testEmulatorBaseURL, ExpectedNamespace: testEmulatorNamespace},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			// Set emulator env var to ensure it doesn't interfere unless specifically tested
			originalEnv := os.Getenv(emulatorDatabaseEnvVar)
			os.Unsetenv(emulatorDatabaseEnvVar)
			defer os.Setenv(emulatorDatabaseEnvVar, originalEnv)

			appInstance := newTestDBApp(ctx, tc.AppDBURL, tc.AppAuthOverride)

			var c *Client
			var err error
			if tc.ArgDBURL != "" {
				c, err = NewClient(ctx, appInstance, tc.ArgDBURL)
			} else {
				c, err = NewClient(ctx, appInstance)
			}
			if err != nil {
				t.Fatal(err)
			}

			if c.dbURLConfig.BaseURL != tc.ExpectedBaseURL {
				t.Errorf("NewClient(%v).baseURL = %q; want = %q", tc.Name, c.dbURLConfig.BaseURL, tc.ExpectedBaseURL)
			}
			if c.dbURLConfig.Namespace != tc.ExpectedNamespace {
				t.Errorf("NewClient(%v).Namespace = %q; want = %q", tc.Name, c.dbURLConfig.Namespace, tc.ExpectedNamespace)
			}
			if c.hc == nil {
				t.Errorf("NewClient(%v).hc = nil; want non-nil", tc.Name)
			}

			b, err := json.Marshal(tc.AppAuthOverride) // AuthOverride comes from app
			if err != nil {
				t.Fatal(err)
			}
			if c.authOverride != string(b) {
				t.Errorf("NewClient(%v).authOverride = %q; want = %q", tc.Name, c.authOverride, string(b))
			}
		})
	}
}

func TestValidURLS(t *testing.T) {
	cases := []string{
		"https://test-db.firebaseio.com",
		"https://test-db.firebasedatabase.app",
	}
	for _, tcURL := range cases {
		t.Run(tcURL, func(t *testing.T){
			ctx := context.Background()
			appInstance := newTestDBApp(ctx, "", nil) // DB URL will be passed as arg
			c, err := NewClient(ctx, appInstance, tcURL)
			if err != nil {
				t.Fatal(err)
			}
			if c.dbURLConfig.BaseURL != tcURL {
				t.Errorf("NewClient(%v).url = %q; want = %q", tcURL, c.dbURLConfig.BaseURL, tcURL)
			}
		})
	}
}

func TestInvalidURL(t *testing.T) {
	cases := []string{
		"",    // Handled by NewClient's check for empty targetURL if appDBURL is also empty
		"foo", // Malformed
		"http://db.firebaseio.com", // Not HTTPS for production
		// "http://firebase.google.com", // Not a DB URL
		// "http://localhost:9000", // Emulator URL missing namespace, handled by parseURLConfig via NewClient
	}
	ctx := context.Background()

	// Test case for "" URL with no app default
	appWithoutDBURL := newTestDBApp(ctx, "", nil)
	c, err := NewClient(ctx, appWithoutDBURL) // No URL arg, app has no URL
	if c != nil || err == nil || err.Error() != "database URL must be specified in app config or as an argument" {
		t.Errorf("NewClient with no URL = (%v, %v); want = (nil, 'database URL must be specified...')", c, err)
	}


	for _, tcURL := range cases {
		if tcURL == "" { continue } // Already tested above
		t.Run(tcURL, func(t *testing.T){
			appInstance := newTestDBApp(ctx, "", nil) // AppDBURL doesn't matter if ArgDBURL is provided
			c, err := NewClient(ctx, appInstance, tcURL)
			if c != nil || err == nil {
				t.Errorf("NewClient(%q) = (%v, %v); want = (nil, error)", tcURL, c, err)
			}
		})
	}

	// Emulator URL missing namespace
	t.Run("EmulatorMissingNamespace", func(t *testing.T){
		appInstance := newTestDBApp(ctx, "", nil)
		c, err := NewClient(ctx, appInstance, "http://localhost:9000")
		if c != nil || err == nil {
			t.Errorf("NewClient(http://localhost:9000) = (%v, %v); want = (nil, error)", c, err)
		}
	})
}

func TestInvalidAuthOverride(t *testing.T) {
	ctx := context.Background()
	// AuthOverride comes from app.Config now.
	// The error would occur during app.New if JSON marshaling of AuthOverride fails.
	// Or, if NewClient tried to re-marshal, which it does.
	appInstance := newTestDBApp(ctx, testURL, map[string]interface{}{"uid": func() {}})

	c, err := NewClient(ctx, appInstance) // This should fail due to marshaling AuthOverride from appInstance
	if c != nil || err == nil {
		t.Errorf("NewClient() with invalid auth override in app = (%v, %v); want = (nil, error)", c, err)
	}
}


func TestNewRef(t *testing.T) {
	cases := []struct {
		Path     string
		WantPath string
		WantKey  string
	}{
		{"", "/", ""},
		{"/", "/", ""},
		{"foo", "/foo", "foo"},
		{"/foo", "/foo", "foo"},
		{"foo/bar", "/foo/bar", "bar"},
		{"/foo/bar", "/foo/bar", "bar"},
		{"/foo/bar/", "/foo/bar", "bar"},
	}
	for _, tc := range cases {
		r := client.NewRef(tc.Path) // client is initialized in TestMain
		if r.client == nil {
			t.Errorf("NewRef(%q).client = nil; want = %v", tc.Path, r.client)
		}
		if r.Path != tc.WantPath {
			t.Errorf("NewRef(%q).Path = %q; want = %q", tc.Path, r.Path, tc.WantPath)
		}
		if r.Key != tc.WantKey {
			t.Errorf("NewRef(%q).Key = %q; want = %q", tc.Path, r.Key, tc.WantKey)
		}
	}
}

func TestParent(t *testing.T) {
	cases := []struct {
		Path      string
		HasParent bool
		Want      string
	}{
		{"", false, ""},
		{"/", false, ""},
		{"foo", true, ""},
		{"/foo", true, ""},
		{"foo/bar", true, "foo"},
		{"/foo/bar", true, "foo"},
		{"/foo/bar/", true, "foo"},
	}
	for _, tc := range cases {
		r := client.NewRef(tc.Path).Parent() // client is initialized in TestMain
		if tc.HasParent {
			if r == nil {
				t.Fatalf("Parent(%q) = nil; want = Ref(%q)", tc.Path, tc.Want)
			}
			if r.client == nil {
				t.Errorf("Parent(%q).client = nil; want = %v", tc.Path, client)
			}
			if r.Key != tc.Want {
				t.Errorf("Parent(%q).Key = %q; want = %q", tc.Path, r.Key, tc.Want)
			}
		} else if r != nil {
			t.Fatalf("Parent(%q) = %v; want = nil", tc.Path, r)
		}
	}
}

func TestChild(t *testing.T) {
	r := client.NewRef("/test") // client is initialized in TestMain
	cases := []struct {
		Path   string
		Want   string
		Parent string
	}{
		{"", "/test", "/"},
		{"foo", "/test/foo", "/test"},
		{"/foo", "/test/foo", "/test"},
		{"foo/", "/test/foo", "/test"},
		{"/foo/", "/test/foo", "/test"},
		{"//foo//", "/test/foo", "/test"},
		{"foo/bar", "/test/foo/bar", "/test/foo"},
		{"/foo/bar", "/test/foo/bar", "/test/foo"},
		{"foo/bar/", "/test/foo/bar", "/test/foo"},
		{"/foo/bar/", "/test/foo/bar", "/test/foo"},
		{"//foo/bar", "/test/foo/bar", "/test/foo"},
		{"foo//bar/", "/test/foo/bar", "/test/foo"},
		{"foo/bar//", "/test/foo/bar", "/test/foo"},
	}
	for _, tc := range cases {
		c := r.Child(tc.Path)
		if c.Path != tc.Want {
			t.Errorf("Child(%q) = %q; want = %q", tc.Path, c.Path, tc.Want)
		}
		if c.Parent().Path != tc.Parent {
			t.Errorf("Child(%q).Parent() = %q; want = %q", tc.Path, c.Parent().Path, tc.Parent)
		}
	}
}

func checkOnlyRequest(t *testing.T, got []*testReq, want *testReq) {
	checkAllRequests(t, got, []*testReq{want})
}

func checkAllRequests(t *testing.T, got []*testReq, want []*testReq) {
	if len(got) != len(want) {
		t.Errorf("Request Count = %d; want = %d", len(got), len(want))
	} else {
		for i, r := range got {
			checkRequest(t, r, want[i])
		}
	}
}

func checkRequest(t *testing.T, got, want *testReq) {
	if h := got.Header.Get("Authorization"); h != "Bearer mock-token" { // Assumes testAppOpts provides this token
		t.Errorf("Authorization = %q; want = %q", h, "Bearer mock-token")
	}
	if h := got.Header.Get("User-Agent"); h != testUserAgent { // testUserAgent is set in TestMain
		t.Errorf("User-Agent = %q; want = %q", h, testUserAgent)
	}

	if got.Method != want.Method {
		t.Errorf("Method = %q; want = %q", got.Method, want.Method)
	}

	if got.Path != want.Path {
		t.Errorf("Path = %q; want = %q", got.Path, want.Path)
	}
	if len(want.Query) != len(got.Query) {
		t.Errorf("QueryParam = %v; want = %v", got.Query, want.Query)
	}
	for k, v := range want.Query {
		if got.Query[k] != v {
			t.Errorf("QueryParam(%v) = %v; want = %v", k, got.Query[k], v)
		}
	}
	for k, v := range want.Header {
		if got.Header.Get(k) != v[0] {
			t.Errorf("Header(%q) = %q; want = %q", k, got.Header.Get(k), v[0])
		}
	}
	if want.Body != nil {
		if h := got.Header.Get("Content-Type"); h != "application/json" {
			t.Errorf("User-Agent = %q; want = %q", h, "application/json") // This error message seems to be a copy-paste mistake from User-Agent check
		}
		var wi, gi interface{}
		if err := json.Unmarshal(want.Body, &wi); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(got.Body, &gi); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gi, wi) {
			t.Errorf("Body = %v; want = %v", gi, wi)
		}
	} else if len(got.Body) != 0 {
		t.Errorf("Body = %v; want empty", got.Body)
	}
}

type testReq struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
	Query  map[string]string
}

func newTestReq(r *http.Request) (*testReq, error) {
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(r.RequestURI)
	if err != nil {
		return nil, err
	}

	query := make(map[string]string)
	for k, v := range u.Query() {
		query[k] = v[0]
	}
	return &testReq{
		Method: r.Method,
		Path:   u.Path,
		Header: r.Header,
		Body:   b,
		Query:  query,
	}, nil
}

type mockServer struct {
	Resp   interface{}
	Header map[string]string
	Status int
	Reqs   []*testReq
	srv    *httptest.Server
}

func (s *mockServer) Start(c *Client) *httptest.Server { // c is db.Client
	if s.srv != nil {
		return s.srv
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr, _ := newTestReq(r)
		s.Reqs = append(s.Reqs, tr)

		for k, v := range s.Header {
			w.Header().Set(k, v)
		}

		printVal := r.URL.Query().Get("print") // Renamed from 'print' to avoid conflict
		if s.Status != 0 {
			w.WriteHeader(s.Status)
		} else if printVal == "silent" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		b, _ := json.Marshal(s.Resp)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	s.srv = httptest.NewServer(handler)
	// The db.Client 'c' passed here will have its dbURLConfig.BaseURL updated.
	// This is fine as 'c' would be the client under test.
	c.dbURLConfig.BaseURL = s.srv.URL
	return s.srv
}

type person struct {
	Name string `json:"name"`
	Age  int32  `json:"age"`
}

func serialize(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
