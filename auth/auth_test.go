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

package auth

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	"google.golang.org/api/option"
	"google.golang.org/api/transport"

	"firebase.google.com/go/internal"
)

var creds *google.DefaultCredentials
var client *Client
var testIDToken string

func verifyCustomToken(t *testing.T, token string, expected map[string]interface{}) {
	h := &jwtHeader{}
	p := &customToken{}
	if err := decodeToken(token, client.ks, h, p); err != nil {
		t.Fatal(err)
	}

	if h.Algorithm != "RS256" {
		t.Errorf("Algorithm: %q; want: 'RS256'", h.Algorithm)
	} else if h.Type != "JWT" {
		t.Errorf("Type: %q; want: 'JWT'", h.Type)
	} else if p.Aud != firebaseAudience {
		t.Errorf("Audience: %q; want: %q", p.Aud, firebaseAudience)
	}

	for k, v := range expected {
		if p.Claims[k] != v {
			t.Errorf("Claim[%q]: %v; want: %v", k, p.Claims[k], v)
		}
	}
}

func getIDToken(p mockIDTokenPayload) string {
	return getIDTokenWithKid("mock-key-id-1", p)
}

func getIDTokenWithKid(kid string, p mockIDTokenPayload) string {
	pCopy := mockIDTokenPayload{
		"aud":   client.projectID,
		"iss":   "https://securetoken.google.com/" + client.projectID,
		"iat":   time.Now().Unix() - 100,
		"exp":   time.Now().Unix() + 3600,
		"sub":   "1234567890",
		"admin": true,
	}
	for k, v := range p {
		pCopy[k] = v
	}
	h := defaultHeader()
	h.KeyID = kid
	token, _ := encodeToken(h, pCopy, client.pk)
	return token
}

type mockIDTokenPayload map[string]interface{}

func (p mockIDTokenPayload) decode(s string) error {
	return decode(s, &p)
}

type mockKeySource struct {
	keys []*publicKey
	err  error
}

func (t *mockKeySource) Keys() ([]*publicKey, error) {
	return t.keys, t.err
}

func TestMain(m *testing.M) {
	var err error
	opt := option.WithCredentialsFile("../testdata/service_account.json")
	creds, err = transport.Creds(context.Background(), opt)
	if err != nil {
		os.Exit(1)
	}

	client, err = NewClient(&internal.AuthConfig{
		Creds:     creds,
		ProjectID: "mock-project-id",
	})
	if err != nil {
		os.Exit(1)
	}
	client.ks = &fileKeySource{FilePath: "../testdata/public_certs.json"}

	testIDToken = getIDToken(nil)
	os.Exit(m.Run())
}

func TestCustomToken(t *testing.T) {
	token, err := client.CustomToken("user1")
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(t, token, nil)
}

func TestCustomTokenWithClaims(t *testing.T) {
	claims := map[string]interface{}{
		"foo":     "bar",
		"premium": true,
		"count":   float64(123),
	}
	token, err := client.CustomTokenWithClaims("user1", claims)
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(t, token, claims)
}

func TestCustomTokenWithNilClaims(t *testing.T) {
	token, err := client.CustomTokenWithClaims("user1", nil)
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(t, token, nil)
}

func TestCustomTokenError(t *testing.T) {
	cases := []struct {
		name   string
		uid    string
		claims map[string]interface{}
	}{
		{"EmptyName", "", nil},
		{"LongUid", strings.Repeat("a", 129), nil},
		{"ReservedClaims", "uid", map[string]interface{}{"sub": "1234"}},
	}

	for _, tc := range cases {
		token, err := client.CustomTokenWithClaims(tc.uid, tc.claims)
		if token != "" || err == nil {
			t.Errorf("CustomTokenWithClaims(%q) = (%q, %v); want: (\"\", error)", tc.name, token, err)
		}
	}
}

func TestCustomTokenInvalidCredential(t *testing.T) {
	s, err := NewClient(&internal.AuthConfig{})
	if err != nil {
		t.Fatal(err)
	}

	token, err := s.CustomToken("user1")
	if token != "" || err == nil {
		t.Errorf("CustomTokenWithClaims() = (%q, %v); want: (\"\", error)", token, err)
	}

	token, err = s.CustomTokenWithClaims("user1", map[string]interface{}{"foo": "bar"})
	if token != "" || err == nil {
		t.Errorf("CustomTokenWithClaims() = (%q, %v); want: (\"\", error)", token, err)
	}
}

func TestVerifyIDToken(t *testing.T) {
	ft, err := client.VerifyIDToken(testIDToken)
	if err != nil {
		t.Fatal(err)
	}
	if ft.Claims["admin"] != true {
		t.Errorf("Claims['admin'] = %v; want: true", ft.Claims["admin"])
	}
	if ft.UID != ft.Subject {
		t.Errorf("UID = %q; Sub = %q; want UID = Sub", ft.UID, ft.Subject)
	}
}

func TestVerifyIDTokenError(t *testing.T) {
	var now int64 = 1000
	cases := []struct {
		name  string
		token string
	}{
		{"NoKid", getIDTokenWithKid("", nil)},
		{"WrongKid", getIDTokenWithKid("foo", nil)},
		{"BadAudience", getIDToken(mockIDTokenPayload{"aud": "bad-audience"})},
		{"BadIssuer", getIDToken(mockIDTokenPayload{"iss": "bad-issuer"})},
		{"EmptySubject", getIDToken(mockIDTokenPayload{"sub": ""})},
		{"IntSubject", getIDToken(mockIDTokenPayload{"sub": 10})},
		{"LongSubject", getIDToken(mockIDTokenPayload{"sub": strings.Repeat("a", 129)})},
		{"FutureToken", getIDToken(mockIDTokenPayload{"iat": time.Unix(now+1, 0)})},
		{"ExpiredToken", getIDToken(mockIDTokenPayload{
			"iat": time.Unix(now-10, 0),
			"exp": time.Unix(now-1, 0),
		})},
		{"EmptyToken", ""},
		{"BadFormatToken", "foobar"},
	}

	clk = &mockClock{now: time.Unix(now, 0)}
	defer func() {
		clk = &systemClock{}
	}()
	for _, tc := range cases {
		if _, err := client.VerifyIDToken(tc.token); err == nil {
			t.Errorf("VerifyyIDToken(%q) = nil; want error", tc.name)
		}
	}
}

func TestNoProjectID(t *testing.T) {
	c, err := NewClient(&internal.AuthConfig{Creds: creds})
	if err != nil {
		t.Fatal(err)
	}
	c.ks = client.ks
	if _, err := c.VerifyIDToken(testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCustomTokenVerification(t *testing.T) {
	token, err := client.CustomToken("user1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.VerifyIDToken(token); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCertificateRequestError(t *testing.T) {
	ks := client.ks
	client.ks = &mockKeySource{nil, errors.New("mock error")}
	defer func() {
		client.ks = ks
	}()
	if _, err := client.VerifyIDToken(testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}
