package appcheck

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"firebase.google.com/go/v4/app" // Import app package
	// "firebase.google.com/go/v4/internal" // No longer needed for AppCheckConfig
	"github.com/MicahParks/keyfunc" // Needed for monkey-patching JWKS in some tests if http client isn't sufficient
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/option" // For creating test app with options
)

const testProjectID = "project_id"

// Helper to create a new app.App for AppCheck tests
func newTestAppCheckApp(ctx context.Context) *app.App {
	// AppCheck client doesn't require special options beyond what app.New provides by default for http client.
	// It does require a ProjectID.
	appInstance, err := app.New(ctx, &app.Config{ProjectID: testProjectID}, option.WithScopes( /* any specific scopes if needed, else default */ ))
	if err != nil {
		log.Fatalf("Error creating test app for AppCheck: %v", err)
	}
	return appInstance
}


func TestVerifyTokenHasValidClaims(t *testing.T) {
	ts, err := setupFakeJWKS()
	if err != nil {
		t.Fatalf("Error setting up fake JWKS server: %v", err)
	}
	defer ts.Close()

	privateKey, err := loadPrivateKey()
	if err != nil {
		t.Fatalf("Error loading private key: %v", err)
	}

	originalJWKSUrl := JWKSUrl // Backup original
	JWKSUrl = ts.URL           // Point to mock server
	defer func() { JWKSUrl = originalJWKSUrl }() // Restore

	ctx := context.Background()
	appInstance := newTestAppCheckApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Errorf("Error creating NewClient: %v", err)
	}
	// JWKS refresh might happen in background, ensure client has time to fetch from mock if NewClient doesn't block on it.
	// However, keyfunc.Get in NewClient is blocking, so this should be fine.
	// If client.jwks is nil, it means keyfunc.Get failed, which NewClient should have errored on.
	if client.jwks == nil && err == nil {
		// Attempt a manual refresh if initial failed silently and NewClient didn't error
		// This might be needed if the test server wasn't up when NewClient's keyfunc.Get ran.
		// Or, more simply, ensure test server (ts) is started before NewClient is called.
		// The current setup does this, so client.jwks should be populated.
		// Forcing a refresh for test stability if http client was an issue:
		newJwks, refreshErr := keyfunc.Get(ts.URL, keyfunc.Options{Ctx: ctx, Client: http.DefaultClient})
		if refreshErr != nil {
			t.Fatalf("Manual JWKS refresh failed: %v", refreshErr)
		}
		client.jwks = newJwks
	}


	type appCheckClaims struct {
		Aud []string `json:"aud"`
		jwt.RegisteredClaims
	}

	mockTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	jwt.TimeFunc = func() time.Time {
		return mockTime
	}
	defer func() { jwt.TimeFunc = time.Now }() // Restore time function

	tokenTests := []struct {
		name      string // Added name for better test output
		claims    *appCheckClaims
		wantErr   error
		wantToken *DecodedAppCheckToken
	}{
		{
			name: "ValidToken",
			claims: &appCheckClaims{
				[]string{"projects/12345678", "projects/" + testProjectID}, // Use testProjectID
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					Subject:   "12345678:app:ID",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			wantErr: nil,
			wantToken: &DecodedAppCheckToken{
				Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
				Subject:   "12345678:app:ID",
				Audience:  []string{"projects/12345678", "projects/" + testProjectID},
				ExpiresAt: mockTime.Add(time.Hour),
				IssuedAt:  mockTime,
				AppID:     "12345678:app:ID",
				Claims:    map[string]interface{}{},
			},
		}, {
			name: "ValidTokenWithExtraClaims",
			claims: &appCheckClaims{
				[]string{"projects/12345678", "projects/" + testProjectID},
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					Subject:   "12345678:app:ID",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
					NotBefore: jwt.NewNumericDate(mockTime.Add(-1 * time.Hour)),
				}},
			wantErr: nil,
			wantToken: &DecodedAppCheckToken{
				Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
				Subject:   "12345678:app:ID",
				Audience:  []string{"projects/12345678", "projects/" + testProjectID},
				ExpiresAt: mockTime.Add(time.Hour),
				IssuedAt:  mockTime,
				AppID:     "12345678:app:ID",
				Claims: map[string]interface{}{
					"nbf": float64(mockTime.Add(-1 * time.Hour).Unix()),
				},
			},
		}, {
			name: "WrongAudience",
			claims: &appCheckClaims{
				[]string{"projects/0000000", "projects/another_project_id"}, // Does not contain testProjectID
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					Subject:   "12345678:app:ID",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			wantErr:   ErrTokenAudience,
			wantToken: nil,
		}, {
			name: "WrongIssuer",
			claims: &appCheckClaims{
				[]string{"projects/12345678", "projects/" + testProjectID},
				jwt.RegisteredClaims{
					Issuer:    "https://not-firebaseappcheck.googleapis.com/12345678",
					Subject:   "12345678:app:ID",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			wantErr:   ErrTokenIssuer,
			wantToken: nil,
		}, {
			name: "EmptySubject",
			claims: &appCheckClaims{
				[]string{"projects/12345678", "projects/" + testProjectID},
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					Subject:   "",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			wantErr:   ErrTokenSubject,
			wantToken: nil,
		}, {
			name: "MissingSubject",
			claims: &appCheckClaims{
				[]string{"projects/12345678", "projects/" + testProjectID},
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			wantErr:   ErrTokenSubject,
			wantToken: nil,
		},
	}

	for _, tc := range tokenTests {
		t.Run(tc.name, func(t *testing.T) {
			jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, tc.claims)
			jwtToken.Header["kid"] = "FGQdnRlzAmKyKr6-Hg_kMQrBkj_H6i6ADnBQz4OI6BU" // From mock.jwks.json

			token, err := jwtToken.SignedString(privateKey)
			if err != nil {
				t.Fatalf("error generating JWT: %v", err)
			}

			gotToken, gotErr := client.VerifyToken(token)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Errorf("VerifyToken() error = %v, want %v", gotErr, tc.wantErr)
			}
			if diff := cmp.Diff(tc.wantToken, gotToken); diff != "" {
				t.Errorf("VerifyToken() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestVerifyTokenMustExist(t *testing.T) {
	ts, err := setupFakeJWKS()
	if err != nil {
		t.Fatalf("Error setting up fake JWK server: %v", err)
	}
	defer ts.Close()

	originalJWKSUrl := JWKSUrl
	JWKSUrl = ts.URL
	defer func() { JWKSUrl = originalJWKSUrl }()

	ctx := context.Background()
	appInstance := newTestAppCheckApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Errorf("Error creating NewClient: %v", err)
	}

	for _, token := range []string{"", "-", "."} {
		gotToken, gotErr := client.VerifyToken(token)
		if gotErr == nil {
			t.Errorf("VerifyToken(%s) expected error, got nil", token)
		}
		if gotToken != nil {
			t.Errorf("Expected nil, got token %v", gotToken)
		}
	}
}

func TestVerifyTokenNotExpired(t *testing.T) {
	ts, err := setupFakeJWKS()
	if err != nil {
		t.Fatalf("Error setting up fake JWKS server: %v", err)
	}
	defer ts.Close()

	privateKey, err := loadPrivateKey()
	if err != nil {
		t.Fatalf("Error loading private key: %v", err)
	}

	originalJWKSUrl := JWKSUrl
	JWKSUrl = ts.URL
	defer func() { JWKSUrl = originalJWKSUrl }()

	ctx := context.Background()
	appInstance := newTestAppCheckApp(ctx)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Errorf("Error creating NewClient: %v", err)
	}
	// Ensure JWKS is loaded if there was any issue with http client in NewClient
	if client.jwks == nil && err == nil {
		newJwks, refreshErr := keyfunc.Get(ts.URL, keyfunc.Options{Ctx: ctx, Client: http.DefaultClient})
		if refreshErr != nil {t.Fatalf("Manual JWKS refresh failed: %v", refreshErr)}
		client.jwks = newJwks
	}


	mockTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	jwt.TimeFunc = func() time.Time {
		return mockTime
	}
	defer func() { jwt.TimeFunc = time.Now }()


	tokenTests := []struct {
		name      string // Added name
		expiresAt time.Time
		wantErr   bool
	}{
		{"FutureExpiry", mockTime.Add(time.Hour), false},
		{"PastExpiry", mockTime.Add(-1 * time.Hour), true},
	}

	for _, tc := range tokenTests {
		t.Run(tc.name, func(t *testing.T){
			claims := struct {
				Aud []string `json:"aud"`
				jwt.RegisteredClaims
			}{
				[]string{"projects/12345678", "projects/" + testProjectID},
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					Subject:   "12345678:app:ID",
					ExpiresAt: jwt.NewNumericDate(tc.expiresAt),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				},
			}

			jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			jwtToken.Header["kid"] = "FGQdnRlzAmKyKr6-Hg_kMQrBkj_H6i6ADnBQz4OI6BU"

			token, err := jwtToken.SignedString(privateKey)
			if err != nil {
				t.Fatalf("error generating JWT: %v", err)
			}

			_, gotErr := client.VerifyToken(token)
			if tc.wantErr && gotErr == nil {
				t.Errorf("Expected an error, got none")
			} else if !tc.wantErr && gotErr != nil {
				t.Errorf("Expected no error, got %v", gotErr)
			}
		})
	}
}

func setupFakeJWKS() (*httptest.Server, error) {
	jwks, err := os.ReadFile("../testdata/mock.jwks.json")
	if err != nil {
		return nil, err
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(jwks)
	}))
	return ts, nil
}

func loadPrivateKey() (*rsa.PrivateKey, error) {
	pk, err := os.ReadFile("../testdata/appcheck_pk.pem")
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(pk)
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing private key")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
