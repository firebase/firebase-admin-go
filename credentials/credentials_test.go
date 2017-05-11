package credentials

import (
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

	expected := jwt.Config{
		Email:        "mock-email@mock-project.iam.gserviceaccount.com",
		PrivateKeyID: "mock-key-id-1",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
	}
	got := cred.(*certificate).Config
	if expected.Email != got.Email {
		t.Errorf("Expected: %s, Got: %s", expected.Email, got.Email)
	}
	if expected.PrivateKeyID != got.PrivateKeyID {
		t.Errorf("Expected: %s, Got: %s", expected.PrivateKeyID, got.PrivateKeyID)
	}
	if expected.TokenURL != got.TokenURL {
		t.Errorf("Expected: %s, Got: %s", expected.TokenURL, got.TokenURL)
	}
	if !reflect.DeepEqual(firebaseScopes, got.Scopes) {
		t.Errorf("Expected: %v, Got: %v", firebaseScopes, got.Scopes)
	}

	ts := initMockServer()
	defer ts.Close()
	got.TokenURL = ts.URL

	token, expiry, err := cred.AccessToken()
	if err != nil {
		t.Fatal(err)
	}
	if token != "mock-token" {
		t.Errorf("Expected: mock-token, Got: %s", token)
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
		if cred != nil {
			t.Errorf("Expected nil, Got: %v", cred)
		}
		if err == nil {
			t.Error("Expected error, Got nil")
		}
	}
}

func TestRefreshToken(t *testing.T) {
	cred, err := NewRefreshToken("testdata/refresh_token.json")
	if err != nil {
		t.Fatal(err)
	}

	c := cred.(*refreshToken)
	expected := oauth2.Config{
		ClientID:     "mock.apps.googleusercontent.com",
		ClientSecret: "mock-secret",
	}
	got := c.Config
	if expected.ClientID != got.ClientID {
		t.Errorf("Expected: %s, Got: %s", expected.ClientID, got.ClientID)
	}
	if expected.ClientSecret != got.ClientSecret {
		t.Errorf("Expected: %s, Got: %s", expected.ClientSecret, got.ClientSecret)
	}
	if !reflect.DeepEqual(firebaseScopes, got.Scopes) {
		t.Errorf("Expected: %v, Got: %v", firebaseScopes, got.Scopes)
	}
	if c.Token.RefreshToken != "mock-refresh-token" {
		t.Errorf("Expected: %s, Got: %s", "mock-refresh-token", c.Token.RefreshToken)
	}

	ts := initMockServer()
	defer ts.Close()
	got.Endpoint.TokenURL = ts.URL

	token, expiry, err := cred.AccessToken()
	if err != nil {
		t.Fatal(err)
	}
	if token != "mock-token" {
		t.Errorf("Expected: mock-token, Got: %s", token)
	}

	expiresIn := int64(expiry.Sub(time.Now()) / time.Minute)
	if expiresIn < 55 || expiresIn > 60 {
		t.Errorf("Invalid expiry duration: %d", expiresIn)
	}
}

func TestRefreshTokenWithInvalidFile(t *testing.T) {
	invalidFiles := []string{
		"testdata",
		"testdata/non_existing.json",
		"testdata/plain_text.txt",
		"testdata/service_account.json",
	}

	for _, tc := range invalidFiles {
		cred, err := NewRefreshToken(tc)
		if cred != nil {
			t.Errorf("Expected nil, Got: %v", cred)
		}
		if err == nil {
			t.Error("Expected error, Got nil")
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

	cred, err := NewAppDefault()
	if err != nil {
		t.Fatal(err)
	}

	c := cred.(*appDefault)
	ts := initMockServer()
	defer ts.Close()
	deadline := time.Now().Add(time.Hour)
	c.Credential.TokenSource = &testTokenSource{"mock-token", deadline}

	token, expiry, err := cred.AccessToken()
	if err != nil {
		t.Fatal(err)
	}
	if token != "mock-token" {
		t.Errorf("Expected: mock-token, Got: %s", token)
	}
	if expiry != deadline {
		t.Errorf("Expected %v, got %v", deadline, expiry)
	}
}

func TestAppDefaultWithInvalidFile(t *testing.T) {
	varName := "GOOGLE_APPLICATION_CREDENTIALS"
	current := os.Getenv(varName)

	if err := os.Setenv(varName, "testdata/non_existing.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(varName, current)

	cred, err := NewAppDefault()
	if cred != nil {
		t.Errorf("Expected nil, Got: %v", cred)
	}
	if err == nil {
		t.Error("Expected error, Got nil")
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
