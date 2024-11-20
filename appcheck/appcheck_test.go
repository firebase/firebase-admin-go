package appcheck

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"firebase.google.com/go/v4/internal"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-cmp/cmp"
)

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

	JWKSUrl = ts.URL
	conf := &internal.AppCheckConfig{
		ProjectID: "project_id",
	}

	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Errorf("Error creating NewClient: %v", err)
	}

	type appCheckClaims struct {
		Aud []string `json:"aud"`
		jwt.RegisteredClaims
	}

	mockTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	jwt.TimeFunc = func() time.Time {
		return mockTime
	}

	tokenTests := []struct {
		claims    *appCheckClaims
		wantErr   error
		wantToken *DecodedAppCheckToken
	}{
		{
			&appCheckClaims{
				[]string{"projects/12345678", "projects/project_id"},
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					Subject:   "12345678:app:ID",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			nil,
			&DecodedAppCheckToken{
				Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
				Subject:   "12345678:app:ID",
				Audience:  []string{"projects/12345678", "projects/project_id"},
				ExpiresAt: mockTime.Add(time.Hour),
				IssuedAt:  mockTime,
				AppID:     "12345678:app:ID",
				Claims:    map[string]interface{}{},
			},
		}, {
			&appCheckClaims{
				[]string{"projects/12345678", "projects/project_id"},
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					Subject:   "12345678:app:ID",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
					// A field our AppCheckToken does not use.
					NotBefore: jwt.NewNumericDate(mockTime.Add(-1 * time.Hour)),
				}},
			nil,
			&DecodedAppCheckToken{
				Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
				Subject:   "12345678:app:ID",
				Audience:  []string{"projects/12345678", "projects/project_id"},
				ExpiresAt: mockTime.Add(time.Hour),
				IssuedAt:  mockTime,
				AppID:     "12345678:app:ID",
				Claims: map[string]interface{}{
					"nbf": float64(mockTime.Add(-1 * time.Hour).Unix()),
				},
			},
		}, {
			&appCheckClaims{
				[]string{"projects/0000000", "projects/another_project_id"},
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					Subject:   "12345678:app:ID",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			ErrTokenAudience,
			nil,
		}, {
			&appCheckClaims{
				[]string{"projects/12345678", "projects/project_id"},
				jwt.RegisteredClaims{
					Issuer:    "https://not-firebaseappcheck.googleapis.com/12345678",
					Subject:   "12345678:app:ID",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			ErrTokenIssuer,
			nil,
		}, {
			&appCheckClaims{
				[]string{"projects/12345678", "projects/project_id"},
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					Subject:   "",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			ErrTokenSubject,
			nil,
		}, {
			&appCheckClaims{
				[]string{"projects/12345678", "projects/project_id"},
				jwt.RegisteredClaims{
					Issuer:    "https://firebaseappcheck.googleapis.com/12345678",
					ExpiresAt: jwt.NewNumericDate(mockTime.Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(mockTime),
				}},
			ErrTokenSubject,
			nil,
		},
	}

	for _, tc := range tokenTests {
		// Create an App Check-style token.
		jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, tc.claims)

		// kid matches the key ID in testdata/mock.jwks.json,
		// which is the public key matching to the private key
		// in testdata/appcheck_pk.pem.
		jwtToken.Header["kid"] = "FGQdnRlzAmKyKr6-Hg_kMQrBkj_H6i6ADnBQz4OI6BU"

		token, err := jwtToken.SignedString(privateKey)
		if err != nil {
			t.Fatalf("error generating JWT: %v", err)
		}

		// Verify the token.
		gotToken, gotErr := client.VerifyToken(token)
		if !errors.Is(gotErr, tc.wantErr) {
			t.Errorf("Expected error %v, got %v", tc.wantErr, gotErr)
			continue
		}
		if diff := cmp.Diff(tc.wantToken, gotToken); diff != "" {
			t.Errorf("VerifyToken mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestVerifyTokenMustExist(t *testing.T) {
	ts, err := setupFakeJWKS()
	if err != nil {
		t.Fatalf("Error setting up fake JWK server: %v", err)
	}
	defer ts.Close()

	JWKSUrl = ts.URL
	conf := &internal.AppCheckConfig{
		ProjectID: "project_id",
	}

	client, err := NewClient(context.Background(), conf)
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

	JWKSUrl = ts.URL
	conf := &internal.AppCheckConfig{
		ProjectID: "project_id",
	}

	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Errorf("Error creating NewClient: %v", err)
	}

	mockTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	jwt.TimeFunc = func() time.Time {
		return mockTime
	}

	tokenTests := []struct {
		expiresAt time.Time
		wantErr   bool
	}{
		// Expire in the future is OK.
		{mockTime.Add(time.Hour), false},
		// Expire in the past is not OK.
		{mockTime.Add(-1 * time.Hour), true},
	}

	for _, tc := range tokenTests {
		claims := struct {
			Aud []string `json:"aud"`
			jwt.RegisteredClaims
		}{
			[]string{"projects/12345678", "projects/project_id"},
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
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
