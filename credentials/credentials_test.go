package credentials

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

func TestCert(t *testing.T) {
	f, err := os.Open("testdata/service_account.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cred, err := NewCert(f)
	if err != nil {
		t.Fatal(err)
	}

	want := jwt.Config{
		Email:        "mock-email@mock-project.iam.gserviceaccount.com",
		PrivateKeyID: "mock-key-id-1",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
	}
	got := cred.(*certificate).Config
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

	// Start a mock token server, and point the Credential to that. AccessToken will contact the server to obtain
	// OAuth2 tokens.
	ts := initMockServer()
	defer ts.Close()
	got.TokenURL = ts.URL

	token, expiry, err := cred.AccessToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "mock-token" {
		t.Errorf("Token: %q; want: %q", token, "mock-token")
	}

	expiresIn := int64(expiry.Sub(time.Now()) / time.Minute)
	if expiresIn < 55 || expiresIn > 60 {
		t.Errorf("Invalid expiry duration: %d", expiresIn)
	}
}

func TestCertWithInvalidFile(t *testing.T) {
	invalidFiles := []string{
		"testdata",
		"testdata/plain_text.txt",
		"testdata/refresh_token.json",
		"testdata/invalid_service_account.json",
	}

	for _, tc := range invalidFiles {
		f, err := os.Open(tc)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		cred, err := NewCert(f)
		if cred != nil || err == nil {
			t.Errorf("NewCert(%q) = (%v, %v); want: (nil, error)", tc, cred, err)
		}
	}
}

func TestRefreshToken(t *testing.T) {
	f, err := os.Open("testdata/refresh_token.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cred, err := NewRefreshToken(f)
	if err != nil {
		t.Fatal(err)
	}

	c := cred.(*refreshToken)
	want := oauth2.Config{
		ClientID:     "mock.apps.googleusercontent.com",
		ClientSecret: "mock-secret",
	}
	got := c.Config
	if want.ClientID != got.ClientID {
		t.Errorf("ClientID: %q; want: %q", got.ClientID, want.ClientID)
	}
	if want.ClientSecret != got.ClientSecret {
		t.Errorf("ClientSecret: %q; want: %q", got.ClientSecret, want.ClientSecret)
	}
	if !reflect.DeepEqual(firebaseScopes, got.Scopes) {
		t.Errorf("Scopes: %v; want: %v", got.Scopes, firebaseScopes)
	}
	if c.Token.RefreshToken != "mock-refresh-token" {
		t.Errorf("RefreshToken: %q; want: %q", c.Token.RefreshToken, "mock-refresh-token")
	}

	// Start a mock token server, and point the Credential to that. AccessToken will contact the server to obtain
	// OAuth2 tokens.
	ts := initMockServer()
	defer ts.Close()
	got.Endpoint.TokenURL = ts.URL

	token, expiry, err := cred.AccessToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token != "mock-token" {
		t.Errorf("Token: %q; want: %q", token, "mock-token")
	}

	expiresIn := int64(expiry.Sub(time.Now()) / time.Minute)
	if expiresIn < 55 || expiresIn > 60 {
		t.Errorf("Invalid expiry duration: %d", expiresIn)
	}
}

func TestRefreshTokenWithInvalidFile(t *testing.T) {
	invalidFiles := []string{
		"testdata",
		"testdata/plain_text.txt",
		"testdata/service_account.json",
	}

	for _, tc := range invalidFiles {
		f, err := os.Open(tc)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		cred, err := NewRefreshToken(f)
		if cred != nil || err == nil {
			t.Errorf("NewRefreshToken(%q) = (%v, %v); want: (nil, error)", tc, cred, err)
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
	cred, err := NewAppDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}

	want := time.Now().Add(time.Hour)
	c := cred.(*appDefault)
	c.Credential.TokenSource = &testTokenSource{"mock-token", want}

	token, expiry, err := cred.AccessToken(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if token != "mock-token" {
		t.Errorf("Token: %q; want: %q", token, "mock-token")
	}
	if expiry != want {
		t.Errorf("Expiry: %v; want %v", expiry, want)
	}
}

func TestAppDefaultWithInvalidFile(t *testing.T) {
	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, "testdata/non_existing.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

	cred, err := NewAppDefault(context.Background())
	if cred != nil || err == nil {
		t.Errorf("NewAppDefault() = (%v, %v); want: (nil, error)", cred, err)
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
