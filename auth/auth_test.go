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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/internal"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

var (
	testClient          *Client
	testIDToken         string
	testGetUserResponse []byte
	optsWithServiceAcct = []option.ClientOption{
		option.WithCredentialsFile("../testdata/service_account.json"),
	}
	optsWithTokenSource = []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{
			AccessToken: "test.token",
		}),
	}
	testClock     = &internal.MockClock{Timestamp: time.Now()}
	testKeySource = &fileKeySource{FilePath: "../testdata/public_certs.json"}
)

func TestMain(m *testing.M) {
	creds, err := transport.Creds(context.Background(), optsWithServiceAcct...)
	if err != nil {
		log.Fatalln(err)
	}

	testClient, err = NewClient(context.Background(), &internal.AuthConfig{
		Creds:     creds,
		Opts:      optsWithServiceAcct,
		ProjectID: "mock-project-id",
	})
	if err != nil {
		log.Fatalln(err)
	}
	testClient.keySource = testKeySource
	testClient.clock = testClock

	testGetUserResponse, err = ioutil.ReadFile("../testdata/get_user.json")
	if err != nil {
		log.Fatalln(err)
	}
	testIDToken = getIDToken(nil)
	os.Exit(m.Run())
}

func TestNewClientServiceAccountSigner(t *testing.T) {
	if _, ok := testClient.signer.(*serviceAccountSigner); !ok {
		t.Errorf("AuthClient.signer = %#v; want = serviceAccountSigner", testClient.signer)
	}
}

func TestNewClientIAMSigner(t *testing.T) {
	conf := &internal.AuthConfig{
		Opts: optsWithTokenSource,
	}
	c, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Errorf("NewClient() = (%v,%v); want = (nil, error)", c, err)
	}
	if _, ok := c.signer.(*iamSigner); !ok {
		t.Errorf("AuthClient.signer = %#v; want = iamSigner", c.signer)
	}
}

func TestNewClientServiceAccountID(t *testing.T) {
	conf := &internal.AuthConfig{
		Opts:             optsWithTokenSource,
		ServiceAccountID: "explicit-service-account",
	}
	c, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Errorf("NewClient() = (%v,%v); want = (nil, error)", c, err)
	}
	if _, ok := c.signer.(*iamSigner); !ok {
		t.Errorf("AuthClient.signer = %#v; want = iamSigner", c.signer)
	}
	email, err := c.signer.Email(context.Background())
	if email != conf.ServiceAccountID || err != nil {
		t.Errorf("Email() = (%q, %v); want = (%q, nil)", email, err, conf.ServiceAccountID)
	}
}

func TestNewClientInvalidCredentials(t *testing.T) {
	creds := &google.DefaultCredentials{
		JSON: []byte("not json"),
	}
	conf := &internal.AuthConfig{Creds: creds}
	if c, err := NewClient(context.Background(), conf); c != nil || err == nil {
		t.Errorf("NewClient() = (%v,%v); want = (nil, error)", c, err)
	}
}

func TestNewClientInvalidPrivateKey(t *testing.T) {
	sa := map[string]interface{}{
		"private_key":  "foo",
		"client_email": "bar@test.com",
	}
	b, err := json.Marshal(sa)
	if err != nil {
		t.Fatal(err)
	}
	creds := &google.DefaultCredentials{JSON: b}
	conf := &internal.AuthConfig{Creds: creds}
	if c, err := NewClient(context.Background(), conf); c != nil || err == nil {
		t.Errorf("NewClient() = (%v,%v); want = (nil, error)", c, err)
	}
}

func TestCustomToken(t *testing.T) {
	token, err := testClient.CustomToken(context.Background(), "user1")
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(context.Background(), token, nil, t)
}

func TestCustomTokenWithClaims(t *testing.T) {
	claims := map[string]interface{}{
		"foo":     "bar",
		"premium": true,
		"count":   float64(123),
	}
	token, err := testClient.CustomTokenWithClaims(context.Background(), "user1", claims)
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(context.Background(), token, claims, t)
}

func TestCustomTokenWithNilClaims(t *testing.T) {
	token, err := testClient.CustomTokenWithClaims(context.Background(), "user1", nil)
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(context.Background(), token, nil, t)
}

func TestCustomTokenError(t *testing.T) {
	cases := []struct {
		name   string
		uid    string
		claims map[string]interface{}
	}{
		{"EmptyName", "", nil},
		{"LongUid", strings.Repeat("a", 129), nil},
		{"ReservedClaim", "uid", map[string]interface{}{"sub": "1234"}},
		{"ReservedClaims", "uid", map[string]interface{}{"sub": "1234", "aud": "foo"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := testClient.CustomTokenWithClaims(context.Background(), tc.uid, tc.claims)
			if token != "" || err == nil {
				t.Errorf("CustomTokenWithClaims(%q) = (%q, %v); want = (\"\", error)", tc.name, token, err)
			}
		})
	}
}

func TestCustomTokenInvalidCredential(t *testing.T) {
	ctx := context.Background()
	conf := &internal.AuthConfig{
		Creds: nil,
		Opts:  optsWithTokenSource,
	}
	s, err := NewClient(ctx, conf)
	if err != nil {
		t.Fatal(err)
	}

	token, err := s.CustomToken(ctx, "user1")
	if token != "" || err == nil {
		t.Errorf("CustomTokenWithClaims() = (%q, %v); want = (\"\", error)", token, err)
	}

	token, err = s.CustomTokenWithClaims(ctx, "user1", map[string]interface{}{"foo": "bar"})
	if token != "" || err == nil {
		t.Errorf("CustomTokenWithClaims() = (%q, %v); want = (\"\", error)", token, err)
	}
}

func TestVerifyIDTokenAndCheckRevokedValid(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	ft, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), testIDToken)
	if err != nil {
		t.Error(err)
	}
	if ft.Claims["admin"] != true {
		t.Errorf("Claims['admin'] = %v; want = true", ft.Claims["admin"])
	}
	if ft.UID != ft.Subject {
		t.Errorf("UID = %q; Sub = %q; want UID = Sub", ft.UID, ft.Subject)
	}
}

func TestVerifyIDTokenAndCheckRevokedDoNotCheck(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	tok := getIDToken(mockIDTokenPayload{"uid": "uid", "iat": 1970}) // old token

	ft, err := s.Client.VerifyIDToken(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	if ft.Claims["admin"] != true {
		t.Errorf("Claims['admin'] = %v; want = true", ft.Claims["admin"])
	}
	if ft.UID != ft.Subject {
		t.Errorf("UID = %q; Sub = %q; want UID = Sub", ft.UID, ft.Subject)
	}
}

func TestVerifyIDTokenAndCheckRevokedInvalidated(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	tok := getIDToken(mockIDTokenPayload{"uid": "uid", "iat": 1970}) // old token

	p, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), tok)
	we := "ID token has been revoked"
	if p != nil || err == nil || err.Error() != we || !IsIDTokenRevoked(err) {
		t.Errorf("VerifyIDTokenAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, %v)",
			p, err, nil, we)
	}
}

func TestVerifyIDToken(t *testing.T) {
	ft, err := testClient.VerifyIDToken(context.Background(), testIDToken)
	if err != nil {
		t.Fatal(err)
	}
	if ft.Claims["admin"] != true {
		t.Errorf("Claims['admin'] = %v; want = true", ft.Claims["admin"])
	}
	if ft.UID != ft.Subject {
		t.Errorf("UID = %q; Sub = %q; want UID = Sub", ft.UID, ft.Subject)
	}
}

func TestVerifyIDTokenClockSkew(t *testing.T) {
	now := testClock.Now().Unix()
	cases := []struct {
		name  string
		token string
	}{
		{"FutureToken", getIDToken(mockIDTokenPayload{"iat": now + clockSkewSeconds - 1})},
		{"ExpiredToken", getIDToken(mockIDTokenPayload{
			"iat": now - 1000,
			"exp": now - clockSkewSeconds + 1,
		})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ft, err := testClient.VerifyIDToken(context.Background(), tc.token)
			if err != nil {
				t.Errorf("VerifyIDToken(%q) = (%q, %v); want = (token, nil)", tc.name, ft, err)
			}
			if ft.Claims["admin"] != true {
				t.Errorf("Claims['admin'] = %v; want = true", ft.Claims["admin"])
			}
			if ft.UID != ft.Subject {
				t.Errorf("UID = %q; Sub = %q; want UID = Sub", ft.UID, ft.Subject)
			}
		})
	}
}

func TestVerifyIDTokenInvalidSignature(t *testing.T) {
	parts := strings.Split(testIDToken, ".")
	token := fmt.Sprintf("%s:%s:invalidsignature", parts[0], parts[1])
	if ft, err := testClient.VerifyIDToken(context.Background(), token); ft != nil || err == nil {
		t.Errorf("VerifyiDToken('invalid-signature') = (%v, %v); want = (nil, error)", ft, err)
	}
}

func TestVerifyIDTokenError(t *testing.T) {
	now := testClock.Now().Unix()
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
		{"FutureToken", getIDToken(mockIDTokenPayload{"iat": now + clockSkewSeconds + 1})},
		{"ExpiredToken", getIDToken(mockIDTokenPayload{
			"iat": now - 1000,
			"exp": now - clockSkewSeconds - 1,
		})},
		{"EmptyToken", ""},
		{"BadFormatToken", "foobar"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := testClient.VerifyIDToken(context.Background(), tc.token); err == nil {
				t.Errorf("VerifyIDToken(%q) = nil; want error", tc.name)
			}
		})
	}
}

func TestVerifyIDTokenInvalidAlgorithm(t *testing.T) {
	var payload mockIDTokenPayload
	segments := strings.Split(testIDToken, ".")
	if err := decode(segments[1], &payload); err != nil {
		t.Fatal(err)
	}
	info := &jwtInfo{
		header: jwtHeader{
			Algorithm: "HS256",
			Type:      "JWT",
			KeyID:     "mock-key-id-1",
		},
		payload: payload,
	}
	token, err := info.Token(context.Background(), testClient.signer)
	if err != nil {
		log.Fatalln(err)
	}
	if _, err := testClient.VerifyIDToken(context.Background(), token); err == nil {
		t.Errorf("VerifyIDToken(InvalidAlgorithm) = nil; want error")
	}
}

func TestVerifyIDTokenWithNoProjectID(t *testing.T) {
	conf := &internal.AuthConfig{
		ProjectID: "",
		Opts:      optsWithTokenSource,
	}
	c, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatal(err)
	}
	c.keySource = testKeySource
	if _, err := c.VerifyIDToken(context.Background(), testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCustomTokenVerification(t *testing.T) {
	token, err := testClient.CustomToken(context.Background(), "user1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := testClient.VerifyIDToken(context.Background(), token); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCertificateRequestError(t *testing.T) {
	testClient.keySource = &mockKeySource{nil, errors.New("mock error")}
	defer func() {
		testClient.keySource = testKeySource
	}()
	if _, err := testClient.VerifyIDToken(context.Background(), testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func verifyCustomToken(ctx context.Context, token string, expected map[string]interface{}, t *testing.T) {
	if err := verifyToken(ctx, token, testClient.keySource); err != nil {
		t.Fatal(err)
	}
	var (
		header  jwtHeader
		payload customToken
	)
	segments := strings.Split(token, ".")
	if err := decode(segments[0], &header); err != nil {
		t.Fatal(err)
	}
	if err := decode(segments[1], &payload); err != nil {
		t.Fatal(err)
	}

	email, err := testClient.signer.Email(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if header.Algorithm != "RS256" {
		t.Errorf("Algorithm: %q; want: 'RS256'", header.Algorithm)
	} else if header.Type != "JWT" {
		t.Errorf("Type: %q; want: 'JWT'", header.Type)
	} else if payload.Aud != firebaseAudience {
		t.Errorf("Audience: %q; want: %q", payload.Aud, firebaseAudience)
	} else if payload.Iss != email {
		t.Errorf("Issuer: %q; want: %q", payload.Iss, email)
	} else if payload.Sub != email {
		t.Errorf("Subject: %q; want: %q", payload.Sub, email)
	}

	now := testClock.Now().Unix()
	if payload.Exp != now+3600 {
		t.Errorf("Exp: %d; want: %d", payload.Exp, now+3600)
	}
	if payload.Iat != now {
		t.Errorf("Iat: %d; want: %d", payload.Iat, now)
	}

	for k, v := range expected {
		if payload.Claims[k] != v {
			t.Errorf("Claim[%q]: %v; want: %v", k, payload.Claims[k], v)
		}
	}
}

func getIDToken(p mockIDTokenPayload) string {
	return getIDTokenWithKid("mock-key-id-1", p)
}

func getIDTokenWithKid(kid string, p mockIDTokenPayload) string {
	pCopy := mockIDTokenPayload{
		"aud":   testClient.projectID,
		"iss":   "https://securetoken.google.com/" + testClient.projectID,
		"iat":   testClock.Now().Unix() - 100,
		"exp":   testClock.Now().Unix() + 3600,
		"sub":   "1234567890",
		"admin": true,
	}
	for k, v := range p {
		pCopy[k] = v
	}

	info := &jwtInfo{
		header: jwtHeader{
			Algorithm: "RS256",
			Type:      "JWT",
			KeyID:     kid,
		},
		payload: pCopy,
	}
	token, err := info.Token(context.Background(), testClient.signer)
	if err != nil {
		log.Fatalln(err)
	}
	return token
}

type mockIDTokenPayload map[string]interface{}

func (p mockIDTokenPayload) decodeFrom(s string) error {
	return decode(s, &p)
}

// mockKeySource provides access to a set of in-memory public keys.
type mockKeySource struct {
	keys []*publicKey
	err  error
}

func (k *mockKeySource) Keys(ctx context.Context) ([]*publicKey, error) {
	return k.keys, k.err
}

// fileKeySource loads a set of public keys from the local file system.
type fileKeySource struct {
	FilePath   string
	CachedKeys []*publicKey
}

func (f *fileKeySource) Keys(ctx context.Context) ([]*publicKey, error) {
	if f.CachedKeys == nil {
		certs, err := ioutil.ReadFile(f.FilePath)
		if err != nil {
			return nil, err
		}
		f.CachedKeys, err = parsePublicKeys(certs)
		if err != nil {
			return nil, err
		}
	}
	return f.CachedKeys, nil
}
