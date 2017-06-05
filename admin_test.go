package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"
)

func TestAppFromServiceAcctFile(t *testing.T) {
	app, err := AppFromServiceAcctFile(context.Background(), nil, "testdata/service_account.json")
	if err != nil {
		t.Fatal(err)
	}

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

// initMockServer starts a mock HTTP server that Credential implementations can invoke during tests to obtain OAuth2
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
