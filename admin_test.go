package admin

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"google.golang.org/api/transport"

	"encoding/json"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/option"
)

func TestAppFromServiceAcctFile(t *testing.T) {
	app, err := AppFromServiceAcctFile(context.Background(), nil, "testdata/service_account.json")
	if err != nil {
		t.Fatal(err)
	}
	verifyServiceAcct(t, app)
}

func TestAppFromServiceAcct(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/service_account.json")
	if err != nil {
		t.Fatal(err)
	}
	app, err := AppFromServiceAcct(context.Background(), nil, b)
	if err != nil {
		t.Fatal(err)
	}
	verifyServiceAcct(t, app)
}

func TestClientOptions(t *testing.T) {
	ts := initMockServer()
	defer ts.Close()
	b, err := mockServiceAcct(ts.URL)

	ctx := context.Background()
	app, err := AppFromServiceAcct(ctx, nil, b)
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
	if bearer != "Bearer mock-token" {
		t.Errorf("Bearer token: %q; want: %q", bearer, "Bearer mock-token")
	}
}

func TestAppFromInvalidServiceAcctFile(t *testing.T) {
	invalidFiles := []string{
		"testdata",
		"testdata/plain_text.txt",
		"testdata/refresh_token.json",
		"testdata/invalid_service_account.json",
	}

	ctx := context.Background()
	for _, tc := range invalidFiles {
		app, err := AppFromServiceAcctFile(ctx, nil, tc)
		if app != nil || err == nil {
			t.Errorf("AppFromServiceAcctFile(%q) = (%v, %v); want: (nil, error)", tc, app, err)
		}
	}
}

func TestAppFromRefreshTokenFile(t *testing.T) {
	app, err := AppFromRefreshTokenFile(context.Background(), nil, "testdata/refresh_token.json")
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

func TestAppFromRefreshToken(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/refresh_token.json")
	if err != nil {
		t.Fatal(err)
	}
	app, err := AppFromRefreshToken(context.Background(), nil, b)
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

func TestAppFromRefreshTokenFileWithConfig(t *testing.T) {
	config := &Config{ProjectID: "mock-project-id"}
	app, err := AppFromRefreshTokenFile(context.Background(), config, "testdata/refresh_token.json")
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

func TestAppFromRefreshTokenWithEnvVar(t *testing.T) {
	varName := "GCLOUD_PROJECT"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, "mock-project-id"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

	app, err := AppFromRefreshTokenFile(context.Background(), nil, "testdata/refresh_token.json")
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

func TestAppFromInvalidRefreshTokenFile(t *testing.T) {
	invalidFiles := []string{
		"testdata",
		"testdata/plain_text.txt",
		"testdata/service_account.json",
	}

	ctx := context.Background()
	for _, tc := range invalidFiles {
		app, err := AppFromRefreshTokenFile(ctx, nil, tc)
		if app != nil || err == nil {
			t.Errorf("AppFromRefreshTokenFile(%q) = (%v, %v); want: (nil, error)", tc, app, err)
		}
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

	if len(app.opts) != 2 {
		t.Errorf("Client opts: %d; want: 2", len(app.opts))
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

func TestAuth(t *testing.T) {
	app, err := NewApp(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if c, err := app.Auth(); c == nil || err != nil {
		t.Errorf("Auth() = (%v, %v); want (auth, nil)", c, err)
	}
}

func TestCustomTokenSource(t *testing.T) {
	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, "testdata/service_account.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

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
	want := jwt.Config{
		Email:        "mock-email@mock-project.iam.gserviceaccount.com",
		PrivateKeyID: "mock-key-id-1",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
	}
	got := app.jwtConf
	if want.Email != got.Email {
		t.Errorf("Email: %q; want: %q", got.Email, want.Email)
	}
	if want.PrivateKeyID != got.PrivateKeyID {
		t.Errorf("PrivateKeyID: %q; want: %q", got.PrivateKeyID, want.PrivateKeyID)
	}
	if want.TokenURL != got.TokenURL {
		t.Errorf("TokenURL: %q; want: %q", got.TokenURL, want.TokenURL)
	}
	if !reflect.DeepEqual(firebaseScopes, got.Scopes) {
		t.Errorf("Scopes: %v; want: %v", got.Scopes, firebaseScopes)
	}
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

// initMockServer starts a mock HTTP server that Apps can invoke during tests to obtain OAuth2
// access tokens.
func initMockServer() *httptest.Server {
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
