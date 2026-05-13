// Copyright 2026 Google LLC All Rights Reserved.
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

package phonenumberverification

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/v4/internal"
	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name       string
		cont       context.Context
		conf       *internal.PhoneNumberVerificationConfig
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "Valid Client",
			cont: context.Background(),
			conf: &internal.PhoneNumberVerificationConfig{
				ProjectID: "project_id",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, err := NewClient(tt.cont, tt.conf)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient error is nil.  Want err")
				}
			} else {
				if err != nil {
					t.Errorf("New client error is not nil.  Want nil")
				}
			}

		})
	}
}

func TestVerifyToken(t *testing.T) {
	// Set up a valid EC key pair (P-256) matching the service's ES256 algorithm
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := &privateKey.PublicKey
	kid := "test-key-id"

	// Create a mock JWKS containing the public key
	jwksJSON, err := createJWKSJSON(publicKey, kid)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize the keyfunc from the JSON
	jwks, err := keyfunc.NewJSON(jwksJSON)
	if err != nil {
		t.Fatal(err)
	}

	// Create the Client manually with the mock JWKS
	projectID := "my-project-id"
	client := &Client{
		projectID: projectID,
		jwks:      jwks,
	}

	// Common claims for valid tokens
	validIssuer := issuerPrefix + "some-issuer-suffix" // Needs to start with issuerPrefix
	validAudience := issuerPrefix + projectID
	validSub := "+15555550100"

	tests := []struct {
		name          string
		projectID     string
		validAudience string
		client        *Client
		token         func() string
		wantErr       bool
		wantErrMsg    string
		wantPhone     string
	}{
		{
			name:          "Valid Token",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, &kid, jwt.MapClaims{
					"iss": validIssuer,
					"aud": []string{validAudience},
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr:   false,
			wantPhone: validSub,
		},
		{
			name: "No project ID",
			client: &Client{
				projectID: "",
				jwks:      jwks,
			},
			validAudience: issuerPrefix + "",
			token: func() string {
				return generateToken(t, privateKey, &kid, jwt.MapClaims{
					"iss": validIssuer,
					"aud": []string{validAudience},
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr:    true,
			wantErrMsg: ErrProjectIDRequired.Error(),
		},
		{
			name:          "Empty token",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return ""
			},
			wantErr:    true,
			wantErrMsg: ErrEmptyToken.Error(),
		},
		{
			name:          "Expired Token",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, &kid, jwt.MapClaims{
					"iss": validIssuer,
					"aud": []string{validAudience},
					"sub": validSub,
					"iat": time.Now().Add(-2 * time.Hour).Unix(),
					"exp": time.Now().Add(-1 * time.Hour).Unix(),
				})
			},
			wantErr:    true,
			wantErrMsg: "Token is expired",
		},
		{
			name:          "Wrong Audience",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, &kid, jwt.MapClaims{
					"iss": validIssuer,
					"aud": []string{"wrong-audience"},
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr:    true,
			wantErrMsg: ErrTokenAudience.Error(),
		},
		{
			name:          "Wrong Issuer (Prefix)",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, &kid, jwt.MapClaims{
					"iss": "https://wrong.googleapis.com/",
					"aud": []string{validAudience},
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr:    true,
			wantErrMsg: ErrTokenIssuer.Error(),
		},
		{
			name:          "Missing Subject",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, &kid, jwt.MapClaims{
					"iss": validIssuer,
					"aud": []string{validAudience},
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr:    true,
			wantErrMsg: ErrTokenSubject.Error(),
		},
		{
			name:          "Wrong Algorithm",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				// Sign with HS256 instead of ES256
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"iss": validIssuer,
					"aud": []string{validAudience},
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
				token.Header["kid"] = kid
				s, _ := token.SignedString([]byte("secret"))
				return s
			},
			wantErr:    true,
			wantErrMsg: ErrIncorrectAlgorithm.Error(),
		},
		{
			name:          "Nil Key ID",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, nil, jwt.MapClaims{
					"iss": validIssuer,
					"aud": []string{validAudience},
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr:    true,
			wantErrMsg: ErrTokenHeaderKid.Error(),
		},
		{
			name:          "Unknown Key ID",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, StringPtr("unknown-kid"), jwt.MapClaims{
					"iss": validIssuer,
					"aud": []string{validAudience},
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr: true,
			// Error message depends on keyfunc implementation, but usually complains about missing key
		},
		{
			name:          "Wrong header type",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				// Sign with HS256 instead of ES256
				token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
					"iss": validIssuer,
					"aud": []string{validAudience},
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
				token.Header["kid"] = kid
				token.Header["typ"] = "Wrong header type"
				s, _ := token.SignedString(privateKey)
				return s
			},
			wantErr:    true,
			wantErrMsg: ErrTokenType.Error(),
		},
		{
			name:          "Token claims error",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				token := jwt.NewWithClaims(jwt.SigningMethodES256, nil)
				token.Header["kid"] = kid
				s, _ := token.SignedString(privateKey)
				return s
			},
			wantErr: true,
		},
		{
			name:          "Token expired at error",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, &kid, jwt.MapClaims{
					"iss": validIssuer,
					"aud": validAudience,
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": 0,
				})
			},
			wantErr:   true,
			wantPhone: validSub,
		},
		{
			name:          "Token issued at error",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, &kid, jwt.MapClaims{
					"iss": validIssuer,
					"aud": validAudience,
					"sub": validSub,
					"iat": 0,
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr:   true,
			wantPhone: validSub,
		},
		{
			name:          "Valid Token with single string audience",
			client:        client,
			validAudience: issuerPrefix + projectID,
			token: func() string {
				return generateToken(t, privateKey, &kid, jwt.MapClaims{
					"iss": validIssuer,
					"aud": validAudience,
					"sub": validSub,
					"iat": time.Now().Unix(),
					"exp": time.Now().Add(time.Hour).Unix(),
				})
			},
			wantErr:   false,
			wantPhone: validSub,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString := tt.token()
			got, err := tt.client.VerifyToken(tokenString)

			if tt.wantErr {
				if err == nil {
					t.Errorf("VerifyToken() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("VerifyToken() error = %v, want error containing %v", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("VerifyToken() unexpected error = %v", err)
			}

			if got.PhoneNumber != tt.wantPhone {
				t.Errorf("VerifyToken() PhoneNumber = %v, want %v", got.PhoneNumber, tt.wantPhone)
			}

			// Verify other fields
			if got.Subject != tt.wantPhone {
				t.Errorf("VerifyToken() Subject = %v, want %v", got.Subject, tt.wantPhone)
			}
			if len(got.Audience) == 0 || got.Audience[0] != validAudience {
				t.Errorf("VerifyToken() Audience = %v, want %v", got.Audience, validAudience)
			}
		})
	}
}

// Helper to create a JWKS JSON byte slice from an ECDSA public key
func createJWKSJSON(pub *ecdsa.PublicKey, kid string) ([]byte, error) {
	// P-256 coordinates are 32 bytes
	var x, y [32]byte
	pub.X.FillBytes(x[:])
	pub.Y.FillBytes(y[:])

	jwk := map[string]interface{}{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(x[:]),
		"y":   base64.RawURLEncoding.EncodeToString(y[:]),
		"kid": kid,
		"alg": "ES256",
		"use": "sig",
	}

	jwks := map[string]interface{}{
		"keys": []interface{}{jwk},
	}

	return json.Marshal(jwks)
}

// Helper to generate a signed JWT string
func generateToken(t *testing.T, privateKey *ecdsa.PrivateKey, kid *string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid
	s, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}
	return s
}
func StringPtr(s string) *string { return &s }
