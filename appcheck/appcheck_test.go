package appcheck

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"firebase.google.com/go/v4/internal"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-cmp/cmp"
)

func TestVerifyTokenHasValidClaims(t *testing.T) {
	pk, err := ioutil.ReadFile("../testdata/appcheck_pk.pem")
	if err != nil {
		t.Fatalf("Failed to read private key: %v", err)
	}
	block, _ := pem.Decode(pk)
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	jwks, err := ioutil.ReadFile("../testdata/mock.jwks.json")
	if err != nil {
		t.Fatalf("Failed to read JWKS: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(jwks)
	}))
	defer ts.Close()

	conf := &internal.AppCheckConfig{
		ProjectID: "project_id",
		JWKSUrl:   ts.URL,
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
		wantToken *VerifiedToken
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
			&VerifiedToken{
				Iss:   "https://firebaseappcheck.googleapis.com/12345678",
				Sub:   "12345678:app:ID",
				Aud:   []string{"projects/12345678", "projects/project_id"},
				Exp:   mockTime.Add(time.Hour),
				Iat:   mockTime,
				AppID: "12345678:app:ID",
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
		// Create an App Check token.
		jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, tc.claims)
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
	jwks, err := ioutil.ReadFile("../testdata/mock.jwks.json")
	if err != nil {
		t.Fatalf("Failed to read JWKS: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(jwks)
	}))
	defer ts.Close()

	conf := &internal.AppCheckConfig{
		ProjectID: "project_id",
		JWKSUrl:   ts.URL,
	}

	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Errorf("Error creating NewClient: %v", err)
	}

	gotToken, gotErr := client.VerifyToken("")
	if gotErr == nil {
		t.Errorf("Expected error, got nil")
	}
	if gotToken != nil {
		t.Errorf("Expected nil, got token %v", gotToken)
	}
}
