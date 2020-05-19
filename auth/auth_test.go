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
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/internal"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const (
	credEnvVar    = "GOOGLE_APPLICATION_CREDENTIALS"
	testProjectID = "mock-project-id"
	testVersion   = "test-version"
)

var (
	testGetUserResponse []byte
	testIDToken         string
	testSessionCookie   string
	testSigner          cryptoSigner
	testIDTokenVerifier *tokenVerifier
	testCookieVerifier  *tokenVerifier

	optsWithServiceAcct = []option.ClientOption{
		option.WithCredentialsFile("../testdata/service_account.json"),
	}
	optsWithTokenSource = []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{
			AccessToken: "test.token",
		}),
	}
	testClock = &internal.MockClock{Timestamp: time.Now()}
)

func TestMain(m *testing.M) {
	var err error
	testSigner, err = signerForTests(context.Background())
	logFatal(err)

	testIDTokenVerifier, err = idTokenVerifierForTests(context.Background())
	logFatal(err)

	testCookieVerifier, err = cookieVerifierForTests(context.Background())
	logFatal(err)

	testGetUserResponse, err = ioutil.ReadFile("../testdata/get_user.json")
	logFatal(err)

	testIDToken = getIDToken(nil)
	testSessionCookie = getSessionCookie(nil)
	os.Exit(m.Run())
}

func TestNewClientWithServiceAccountCredentials(t *testing.T) {
	creds, err := transport.Creds(context.Background(), optsWithServiceAcct...)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(context.Background(), &internal.AuthConfig{
		Opts:      optsWithServiceAcct,
		ProjectID: creds.ProjectID,
		Version:   testVersion,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := client.signer.(*serviceAccountSigner); !ok {
		t.Errorf("NewClient().signer = %#v; want = serviceAccountSigner", client.signer)
	}
	if err := checkIDTokenVerifier(client.idTokenVerifier, creds.ProjectID); err != nil {
		t.Errorf("NewClient().idTokenVerifier: %v", err)
	}
	if err := checkCookieVerifier(client.cookieVerifier, creds.ProjectID); err != nil {
		t.Errorf("NewClient().cookieVerifier: %v", err)
	}
	if err := checkBaseClient(client, creds.ProjectID); err != nil {
		t.Errorf("NewClient().baseClient: %v", err)
	}
	if client.clock != internal.SystemClock {
		t.Errorf("NewClient().clock = %v; want = SystemClock", client.clock)
	}
}

func TestNewClientWithoutCredentials(t *testing.T) {
	conf := &internal.AuthConfig{
		Opts:    optsWithTokenSource,
		Version: testVersion,
	}
	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := client.signer.(*iamSigner); !ok {
		t.Errorf("NewClient().signer = %#v; want = iamSigner", client.signer)
	}
	if err := checkIDTokenVerifier(client.idTokenVerifier, ""); err != nil {
		t.Errorf("NewClient().idTokenVerifier = %v; want = nil", err)
	}
	if err := checkCookieVerifier(client.cookieVerifier, ""); err != nil {
		t.Errorf("NewClient().cookieVerifier: %v", err)
	}
	if err := checkBaseClient(client, ""); err != nil {
		t.Errorf("NewClient().baseClient: %v", err)
	}
	if client.clock != internal.SystemClock {
		t.Errorf("NewClient().clock = %v; want = SystemClock", client.clock)
	}
}

func TestNewClientWithServiceAccountID(t *testing.T) {
	conf := &internal.AuthConfig{
		Opts:             optsWithTokenSource,
		ServiceAccountID: "explicit-service-account",
		Version:          testVersion,
	}
	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := client.signer.(*iamSigner); !ok {
		t.Errorf("NewClient().signer = %#v; want = iamSigner", client.signer)
	}
	if err := checkIDTokenVerifier(client.idTokenVerifier, ""); err != nil {
		t.Errorf("NewClient().idTokenVerifier = %v; want = nil", err)
	}
	if err := checkCookieVerifier(client.cookieVerifier, ""); err != nil {
		t.Errorf("NewClient().cookieVerifier: %v", err)
	}
	if err := checkBaseClient(client, ""); err != nil {
		t.Errorf("NewClient().baseClient: %v", err)
	}
	if client.clock != internal.SystemClock {
		t.Errorf("NewClient().clock = %v; want = SystemClock", client.clock)
	}

	email, err := client.signer.Email(context.Background())
	if email != conf.ServiceAccountID || err != nil {
		t.Errorf("Email() = (%q, %v); want = (%q, nil)", email, err, conf.ServiceAccountID)
	}
}

func TestNewClientWithUserCredentials(t *testing.T) {
	creds := &google.DefaultCredentials{
		JSON: []byte(`{
			"client_id": "test-client",
			"client_secret": "test-secret"
		}`),
	}
	conf := &internal.AuthConfig{
		Opts:    []option.ClientOption{option.WithCredentials(creds)},
		Version: testVersion,
	}
	client, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := client.signer.(*iamSigner); !ok {
		t.Errorf("NewClient().signer = %#v; want = iamSigner", client.signer)
	}
	if err := checkIDTokenVerifier(client.idTokenVerifier, ""); err != nil {
		t.Errorf("NewClient().idTokenVerifier = %v; want = nil", err)
	}
	if err := checkCookieVerifier(client.cookieVerifier, ""); err != nil {
		t.Errorf("NewClient().cookieVerifier: %v", err)
	}
	if err := checkBaseClient(client, ""); err != nil {
		t.Errorf("NewClient().baseClient: %v", err)
	}
	if client.clock != internal.SystemClock {
		t.Errorf("NewClient().clock = %v; want = SystemClock", client.clock)
	}
}

func TestNewClientWithMalformedCredentials(t *testing.T) {
	creds := &google.DefaultCredentials{
		JSON: []byte("not json"),
	}
	conf := &internal.AuthConfig{
		Opts: []option.ClientOption{
			option.WithCredentials(creds),
		},
	}
	if c, err := NewClient(context.Background(), conf); c != nil || err == nil {
		t.Errorf("NewClient() = (%v,%v); want = (nil, error)", c, err)
	}
}

func TestNewClientWithInvalidPrivateKey(t *testing.T) {
	sa := map[string]interface{}{
		"private_key":  "not-a-private-key",
		"client_email": "foo@bar",
	}
	b, err := json.Marshal(sa)
	if err != nil {
		t.Fatal(err)
	}
	creds := &google.DefaultCredentials{JSON: b}
	conf := &internal.AuthConfig{
		Opts: []option.ClientOption{
			option.WithCredentials(creds),
		},
	}
	if c, err := NewClient(context.Background(), conf); c != nil || err == nil {
		t.Errorf("NewClient() = (%v,%v); want = (nil, error)", c, err)
	}
}

func TestNewClientAppDefaultCredentialsWithInvalidFile(t *testing.T) {
	current := os.Getenv(credEnvVar)

	if err := os.Setenv(credEnvVar, "../testdata/non_existing.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(credEnvVar, current)

	conf := &internal.AuthConfig{}
	if c, err := NewClient(context.Background(), conf); c != nil || err == nil {
		t.Errorf("Auth() = (%v, %v); want (nil, error)", c, err)
	}
}

func TestNewClientInvalidCredentialFile(t *testing.T) {
	invalidFiles := []string{
		"testdata",
		"testdata/plain_text.txt",
	}

	ctx := context.Background()
	for _, tc := range invalidFiles {
		conf := &internal.AuthConfig{
			Opts: []option.ClientOption{
				option.WithCredentialsFile(tc),
			},
		}
		if c, err := NewClient(ctx, conf); c != nil || err == nil {
			t.Errorf("Auth() = (%v, %v); want (nil, error)", c, err)
		}
	}
}

func TestNewClientExplicitNoAuth(t *testing.T) {
	ctx := context.Background()
	conf := &internal.AuthConfig{
		Opts: []option.ClientOption{
			option.WithoutAuthentication(),
		},
	}
	if c, err := NewClient(ctx, conf); c == nil || err != nil {
		t.Errorf("Auth() = (%v, %v); want (auth, nil)", c, err)
	}
}

func TestCustomToken(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{
			signer: testSigner,
			clock:  testClock,
		},
	}
	token, err := client.CustomToken(context.Background(), "user1")
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCustomToken(context.Background(), token, nil, ""); err != nil {
		t.Fatal(err)
	}
}

func TestCustomTokenWithClaims(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{
			signer: testSigner,
			clock:  testClock,
		},
	}
	claims := map[string]interface{}{
		"foo":     "bar",
		"premium": true,
		"count":   float64(123),
	}
	token, err := client.CustomTokenWithClaims(context.Background(), "user1", claims)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCustomToken(context.Background(), token, claims, ""); err != nil {
		t.Fatal(err)
	}
}

func TestCustomTokenWithNilClaims(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{
			signer: testSigner,
			clock:  testClock,
		},
	}
	token, err := client.CustomTokenWithClaims(context.Background(), "user1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCustomToken(context.Background(), token, nil, ""); err != nil {
		t.Fatal(err)
	}
}

func TestCustomTokenForTenant(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{
			tenantID: "tenantID",
			signer:   testSigner,
			clock:    testClock,
		},
	}
	claims := map[string]interface{}{
		"foo":     "bar",
		"premium": true,
	}
	token, err := client.CustomTokenWithClaims(context.Background(), "user1", claims)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCustomToken(context.Background(), token, claims, "tenantID"); err != nil {
		t.Fatal(err)
	}
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

	client := &baseClient{
		signer: testSigner,
		clock:  testClock,
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := client.CustomTokenWithClaims(context.Background(), tc.uid, tc.claims)
			if token != "" || err == nil {
				t.Errorf("CustomTokenWithClaims(%q) = (%q, %v); want = (\"\", error)", tc.name, token, err)
			}
		})
	}
}

func TestCustomTokenInvalidCredential(t *testing.T) {
	ctx := context.Background()
	conf := &internal.AuthConfig{
		Opts: optsWithTokenSource,
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

func TestVerifyIDToken(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{
			idTokenVerifier: testIDTokenVerifier,
		},
	}

	ft, err := client.VerifyIDToken(context.Background(), testIDToken)
	if err != nil {
		t.Fatal(err)
	}

	now := testClock.Now().Unix()
	if ft.AuthTime != now-100 {
		t.Errorf("AuthTime = %d; want = %d", ft.AuthTime, now-100)
	}
	if ft.Firebase.SignInProvider != "custom" {
		t.Errorf("SignInProvider = %q; want = %q", ft.Firebase.SignInProvider, "custom")
	}
	if ft.Firebase.Tenant != "" {
		t.Errorf("Tenant = %q; want = %q", ft.Firebase.Tenant, "")
	}
	if ft.Claims["admin"] != true {
		t.Errorf("Claims['admin'] = %v; want = true", ft.Claims["admin"])
	}
	if ft.UID != ft.Subject {
		t.Errorf("UID = %q; Sub = %q; want UID = Sub", ft.UID, ft.Subject)
	}
}

func TestVerifyIDTokenFromTenant(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{
			idTokenVerifier: testIDTokenVerifier,
		},
	}

	idToken := getIDToken(mockIDTokenPayload{
		"firebase": map[string]interface{}{
			"tenant":           "tenantID",
			"sign_in_provider": "custom",
		},
	})
	ft, err := client.VerifyIDToken(context.Background(), idToken)
	if err != nil {
		t.Fatal(err)
	}

	now := testClock.Now().Unix()
	if ft.AuthTime != now-100 {
		t.Errorf("AuthTime = %d; want = %d", ft.AuthTime, now-100)
	}
	if ft.Firebase.SignInProvider != "custom" {
		t.Errorf("SignInProvider = %q; want = %q", ft.Firebase.SignInProvider, "custom")
	}
	if ft.Firebase.Tenant != "tenantID" {
		t.Errorf("Tenant = %q; want = %q", ft.Firebase.Tenant, "tenantID")
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

	client := &Client{
		baseClient: &baseClient{
			idTokenVerifier: testIDTokenVerifier,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ft, err := client.VerifyIDToken(context.Background(), tc.token)
			if err != nil {
				t.Fatalf("VerifyIDToken(%q) = (%q, %v); want = (token, nil)", tc.name, ft, err)
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
	client := &Client{
		baseClient: &baseClient{
			idTokenVerifier: testIDTokenVerifier,
		},
	}
	parts := strings.Split(testIDToken, ".")
	token := fmt.Sprintf("%s:%s:invalidsignature", parts[0], parts[1])

	if ft, err := client.VerifyIDToken(context.Background(), token); ft != nil || err == nil {
		t.Errorf("VerifyIDToken('invalid-signature') = (%v, %v); want = (nil, error)", ft, err)
	}
}

func TestVerifyIDTokenError(t *testing.T) {
	now := testClock.Now().Unix()
	cases := []struct {
		name, token, want string
	}{
		{
			name:  "NoKid",
			token: getIDTokenWithKid("", nil),
			want:  "ID token has no 'kid' header",
		},
		{
			name:  "WrongKid",
			token: getIDTokenWithKid("foo", nil),
			want:  "failed to verify token signature",
		},
		{
			name:  "BadAudience",
			token: getIDToken(mockIDTokenPayload{"aud": "bad-audience"}),
			want: `ID token has invalid 'aud' (audience) claim; expected "mock-project-id" but ` +
				`got "bad-audience"; make sure the ID token comes from the same Firebase ` +
				`project as the credential used to authenticate this SDK; see ` +
				`https://firebase.google.com/docs/auth/admin/verify-id-tokens for details on how ` +
				`to retrieve a valid ID token`,
		},
		{
			name:  "BadIssuer",
			token: getIDToken(mockIDTokenPayload{"iss": "bad-issuer"}),
			want: `ID token has invalid 'iss' (issuer) claim; expected ` +
				`"https://securetoken.google.com/mock-project-id" but got "bad-issuer"; make sure the ` +
				`ID token comes from the same Firebase project as the credential used to authenticate ` +
				`this SDK; see https://firebase.google.com/docs/auth/admin/verify-id-tokens for ` +
				`details on how to retrieve a valid ID token`,
		},
		{
			name:  "EmptySubject",
			token: getIDToken(mockIDTokenPayload{"sub": ""}),
			want:  "ID token has empty 'sub' (subject) claim",
		},
		{
			name:  "NonStringSubject",
			token: getIDToken(mockIDTokenPayload{"sub": 10}),
			want:  "json: cannot unmarshal number into Go struct field Token.sub of type string",
		},
		{
			name:  "TooLongSubject",
			token: getIDToken(mockIDTokenPayload{"sub": strings.Repeat("a", 129)}),
			want:  "ID token has a 'sub' (subject) claim longer than 128 characters",
		},
		{
			name:  "FutureToken",
			token: getIDToken(mockIDTokenPayload{"iat": now + clockSkewSeconds + 1}),
			want:  "ID token issued at future timestamp",
		},
		{
			name: "ExpiredToken",
			token: getIDToken(mockIDTokenPayload{
				"iat": now - 1000,
				"exp": now - clockSkewSeconds - 1,
			}),
			want: "ID token has expired",
		},
		{
			name:  "EmptyToken",
			token: "",
			want:  "ID token must be a non-empty string",
		},
		{
			name:  "TooFewSegments",
			token: "foo",
			want:  "incorrect number of segments",
		},
		{
			name:  "TooManySegments",
			token: "fo.ob.ar.baz",
			want:  "incorrect number of segments",
		},
		{
			name:  "MalformedToken",
			token: "foo.bar.baz",
			want:  "invalid character",
		},
	}

	client := &Client{
		baseClient: &baseClient{
			idTokenVerifier: testIDTokenVerifier,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.VerifyIDToken(context.Background(), tc.token)
			if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
				t.Errorf("VerifyIDToken(%q) = %v; want = %q", tc.name, err, tc.want)
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
	token, err := info.Token(context.Background(), testSigner)
	if err != nil {
		t.Fatal(err)
	}

	client := &Client{
		baseClient: &baseClient{
			idTokenVerifier: testIDTokenVerifier,
		},
	}
	if _, err := client.VerifyIDToken(context.Background(), token); err == nil {
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
	c.idTokenVerifier.keySource = testIDTokenVerifier.keySource
	if _, err := c.VerifyIDToken(context.Background(), testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCustomTokenVerification(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{
			idTokenVerifier: testIDTokenVerifier,
			signer:          testSigner,
			clock:           testClock,
		},
	}
	token, err := client.CustomToken(context.Background(), "user1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.VerifyIDToken(context.Background(), token); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCertificateRequestError(t *testing.T) {
	tv, err := newIDTokenVerifier(context.Background(), testProjectID)
	if err != nil {
		t.Fatal(err)
	}
	tv.keySource = &mockKeySource{nil, errors.New("mock error")}
	client := &Client{
		baseClient: &baseClient{
			idTokenVerifier: tv,
		},
	}
	if _, err := client.VerifyIDToken(context.Background(), testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestVerifyIDTokenAndCheckRevoked(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	s.Client.idTokenVerifier = testIDTokenVerifier
	ft, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), testIDToken)
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

func TestVerifyIDTokenDoesNotCheckRevoked(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	revokedToken := getIDToken(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	s.Client.idTokenVerifier = testIDTokenVerifier

	ft, err := s.Client.VerifyIDToken(context.Background(), revokedToken)
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

func TestInvalidTokenDoesNotCheckRevoked(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	s.Client.idTokenVerifier = testIDTokenVerifier

	ft, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), "")
	if ft != nil || err == nil {
		t.Errorf("VerifyIDTokenAndCheckRevoked() = (%v, %v); want = (nil, error)", ft, err)
	}
	if len(s.Req) != 0 {
		t.Errorf("Revocation checks = %d; want = 0", len(s.Req))
	}
}

func TestVerifyIDTokenAndCheckRevokedError(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	revokedToken := getIDToken(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	s.Client.idTokenVerifier = testIDTokenVerifier

	p, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), revokedToken)
	we := "ID token has been revoked"
	if p != nil || err == nil || err.Error() != we || !IsIDTokenRevoked(err) {
		t.Errorf("VerifyIDTokenAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, %v)",
			p, err, nil, we)
	}
}

func TestIDTokenRevocationCheckUserMgtError(t *testing.T) {
	resp := `{
		"kind" : "identitytoolkit#GetAccountInfoResponse",
		"users" : []
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()
	revokedToken := getIDToken(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	s.Client.idTokenVerifier = testIDTokenVerifier

	p, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), revokedToken)
	if p != nil || err == nil || !IsUserNotFound(err) {
		t.Errorf("VerifyIDTokenAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, user-not-found)", p, err, nil)
	}
}

func TestVerifySessionCookie(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{
			cookieVerifier: testCookieVerifier,
		},
	}

	ft, err := client.VerifySessionCookie(context.Background(), testSessionCookie)
	if err != nil {
		t.Fatal(err)
	}

	now := testClock.Now().Unix()
	if ft.AuthTime != now-100 {
		t.Errorf("AuthTime = %d; want = %d", ft.AuthTime, now-100)
	}
	if ft.Firebase.SignInProvider != "custom" {
		t.Errorf("SignInProvider = %q; want = %q", ft.Firebase.SignInProvider, "custom")
	}
	if ft.Firebase.Tenant != "" {
		t.Errorf("Tenant = %q; want = %q", ft.Firebase.Tenant, "")
	}
	if ft.Claims["admin"] != true {
		t.Errorf("Claims['admin'] = %v; want = true", ft.Claims["admin"])
	}
	if ft.UID != ft.Subject {
		t.Errorf("UID = %q; Sub = %q; want UID = Sub", ft.UID, ft.Subject)
	}
}

func TestVerifySessionCookieFromTenant(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{
			cookieVerifier: testCookieVerifier,
		},
	}

	cookie := getSessionCookie(mockIDTokenPayload{
		"firebase": map[string]interface{}{
			"tenant":           "tenantID",
			"sign_in_provider": "custom",
		},
	})
	ft, err := client.VerifySessionCookie(context.Background(), cookie)
	if err != nil {
		t.Fatal(err)
	}

	now := testClock.Now().Unix()
	if ft.AuthTime != now-100 {
		t.Errorf("AuthTime = %d; want = %d", ft.AuthTime, now-100)
	}
	if ft.Firebase.SignInProvider != "custom" {
		t.Errorf("SignInProvider = %q; want = %q", ft.Firebase.SignInProvider, "custom")
	}
	if ft.Firebase.Tenant != "tenantID" {
		t.Errorf("Tenant = %q; want = %q", ft.Firebase.Tenant, "tenantID")
	}
	if ft.Claims["admin"] != true {
		t.Errorf("Claims['admin'] = %v; want = true", ft.Claims["admin"])
	}
	if ft.UID != ft.Subject {
		t.Errorf("UID = %q; Sub = %q; want UID = Sub", ft.UID, ft.Subject)
	}
}

func TestVerifySessionCookieError(t *testing.T) {
	now := testClock.Now().Unix()
	cases := []struct {
		name, token, want string
	}{
		{
			name:  "BadAudience",
			token: getSessionCookie(mockIDTokenPayload{"aud": "bad-audience"}),
			want: `session cookie has invalid 'aud' (audience) claim; expected "mock-project-id" but ` +
				`got "bad-audience"; make sure the session cookie comes from the same Firebase ` +
				`project as the credential used to authenticate this SDK; see ` +
				`https://firebase.google.com/docs/auth/admin/manage-cookies for details on how ` +
				`to retrieve a valid session cookie`,
		},
		{
			name:  "BadIssuer",
			token: getSessionCookie(mockIDTokenPayload{"iss": "bad-issuer"}),
			want: `session cookie has invalid 'iss' (issuer) claim; expected ` +
				`"https://session.firebase.google.com/mock-project-id" but got "bad-issuer"; make sure the ` +
				`session cookie comes from the same Firebase project as the credential used to authenticate ` +
				`this SDK; see https://firebase.google.com/docs/auth/admin/manage-cookies for ` +
				`details on how to retrieve a valid session cookie`,
		},
		{
			name:  "EmptySubject",
			token: getSessionCookie(mockIDTokenPayload{"sub": ""}),
			want:  "session cookie has empty 'sub' (subject) claim",
		},
		{
			name:  "NonStringSubject",
			token: getSessionCookie(mockIDTokenPayload{"sub": 10}),
			want:  "json: cannot unmarshal number into Go struct field Token.sub of type string",
		},
		{
			name:  "TooLongSubject",
			token: getSessionCookie(mockIDTokenPayload{"sub": strings.Repeat("a", 129)}),
			want:  "session cookie has a 'sub' (subject) claim longer than 128 characters",
		},
		{
			name:  "FutureToken",
			token: getSessionCookie(mockIDTokenPayload{"iat": now + clockSkewSeconds + 1}),
			want:  "session cookie issued at future timestamp",
		},
		{
			name: "ExpiredToken",
			token: getSessionCookie(mockIDTokenPayload{
				"iat": now - 1000,
				"exp": now - clockSkewSeconds - 1,
			}),
			want: "session cookie has expired",
		},
		{
			name:  "EmptyToken",
			token: "",
			want:  "session cookie must be a non-empty string",
		},
		{
			name:  "TooFewSegments",
			token: "foo",
			want:  "incorrect number of segments",
		},
		{
			name:  "TooManySegments",
			token: "fo.ob.ar.baz",
			want:  "incorrect number of segments",
		},
		{
			name:  "MalformedToken",
			token: "foo.bar.baz",
			want:  "invalid character",
		},
	}

	client := &Client{
		baseClient: &baseClient{
			cookieVerifier: testCookieVerifier,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.VerifySessionCookie(context.Background(), tc.token)
			if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
				t.Errorf("VerifySessionCookie(%q) = %v; want = %q", tc.name, err, tc.want)
			}
		})
	}
}

func TestVerifySessionCookieDoesNotCheckRevoked(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	revokedCookie := getSessionCookie(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	s.Client.cookieVerifier = testCookieVerifier

	ft, err := s.Client.VerifySessionCookie(context.Background(), revokedCookie)
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

func TestVerifySessionCookieAndCheckRevoked(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	s.Client.cookieVerifier = testCookieVerifier
	ft, err := s.Client.VerifySessionCookieAndCheckRevoked(context.Background(), testSessionCookie)
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

func TestInvalidCookieDoesNotCheckRevoked(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	s.Client.cookieVerifier = testCookieVerifier

	ft, err := s.Client.VerifySessionCookieAndCheckRevoked(context.Background(), "")
	if ft != nil || err == nil {
		t.Errorf("VerifySessionCookieAndCheckRevoked() = (%v, %v); want = (nil, error)", ft, err)
	}
	if len(s.Req) != 0 {
		t.Errorf("Revocation checks = %d; want = 0", len(s.Req))
	}
}

func TestVerifySessionCookieAndCheckRevokedError(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	revokedCookie := getSessionCookie(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	s.Client.cookieVerifier = testCookieVerifier

	p, err := s.Client.VerifySessionCookieAndCheckRevoked(context.Background(), revokedCookie)
	we := "session cookie has been revoked"
	if p != nil || err == nil || err.Error() != we || !IsSessionCookieRevoked(err) {
		t.Errorf("VerifySessionCookieAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, %v)",
			p, err, nil, we)
	}
}

func TestCookieRevocationCheckUserMgtError(t *testing.T) {
	resp := `{
		"kind" : "identitytoolkit#GetAccountInfoResponse",
		"users" : []
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()
	revokedCookie := getSessionCookie(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	s.Client.cookieVerifier = testCookieVerifier

	p, err := s.Client.VerifySessionCookieAndCheckRevoked(context.Background(), revokedCookie)
	if p != nil || err == nil || !IsUserNotFound(err) {
		t.Errorf("VerifySessionCookieAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, user-not-found)", p, err, nil)
	}
}

func signerForTests(ctx context.Context) (cryptoSigner, error) {
	creds, err := transport.Creds(ctx, optsWithServiceAcct...)
	if err != nil {
		return nil, err
	}

	return signerFromCreds(creds.JSON)
}

func idTokenVerifierForTests(ctx context.Context) (*tokenVerifier, error) {
	tv, err := newIDTokenVerifier(ctx, testProjectID)
	if err != nil {
		return nil, err
	}
	ks, err := newMockKeySource("../testdata/public_certs.json")
	if err != nil {
		return nil, err
	}
	tv.keySource = ks
	tv.clock = testClock
	return tv, nil
}

func cookieVerifierForTests(ctx context.Context) (*tokenVerifier, error) {
	tv, err := newSessionCookieVerifier(ctx, testProjectID)
	if err != nil {
		return nil, err
	}
	ks, err := newMockKeySource("../testdata/public_certs.json")
	if err != nil {
		return nil, err
	}
	tv.keySource = ks
	tv.clock = testClock
	return tv, nil
}

// mockKeySource provides access to a set of in-memory public keys.
type mockKeySource struct {
	keys []*publicKey
	err  error
}

func newMockKeySource(filePath string) (*mockKeySource, error) {
	certs, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	keys, err := parsePublicKeys(certs)
	if err != nil {
		return nil, err
	}
	return &mockKeySource{
		keys: keys,
	}, nil
}

func (k *mockKeySource) Keys(ctx context.Context) ([]*publicKey, error) {
	return k.keys, k.err
}

type mockIDTokenPayload map[string]interface{}

func (p mockIDTokenPayload) decodeFrom(s string) error {
	return decode(s, &p)
}

func getSessionCookie(p mockIDTokenPayload) string {
	pCopy := map[string]interface{}{
		"iss": "https://session.firebase.google.com/" + testProjectID,
	}
	for k, v := range p {
		pCopy[k] = v
	}
	return getIDToken(pCopy)
}

func getIDToken(p mockIDTokenPayload) string {
	return getIDTokenWithKid("mock-key-id-1", p)
}

func getIDTokenWithKid(kid string, p mockIDTokenPayload) string {
	pCopy := mockIDTokenPayload{
		"aud":       testProjectID,
		"iss":       "https://securetoken.google.com/" + testProjectID,
		"iat":       testClock.Now().Unix() - 100,
		"exp":       testClock.Now().Unix() + 3600,
		"auth_time": testClock.Now().Unix() - 100,
		"sub":       "1234567890",
		"firebase": map[string]interface{}{
			"identities":       map[string]interface{}{},
			"sign_in_provider": "custom",
		},
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
	token, err := info.Token(context.Background(), testSigner)
	logFatal(err)
	return token
}

func checkIDTokenVerifier(tv *tokenVerifier, projectID string) error {
	if tv == nil {
		return errors.New("tokenVerifier not initialized")
	}
	if tv.projectID != projectID {
		return fmt.Errorf("projectID = %q; want = %q", tv.projectID, projectID)
	}
	if tv.shortName != "ID token" {
		return fmt.Errorf("shortName = %q; want = %q", tv.shortName, "ID token")
	}
	return nil
}

func checkCookieVerifier(tv *tokenVerifier, projectID string) error {
	if tv == nil {
		return errors.New("tokenVerifier not initialized")
	}
	if tv.projectID != projectID {
		return fmt.Errorf("projectID = %q; want = %q", tv.projectID, projectID)
	}
	if tv.shortName != "session cookie" {
		return fmt.Errorf("shortName = %q; want = %q", tv.shortName, "session cookie")
	}
	return nil
}

func checkBaseClient(client *Client, wantProjectID string) error {
	umc := client.baseClient
	if umc.userManagementEndpoint != idToolkitV1Endpoint {
		return fmt.Errorf("userManagementEndpoint = %q; want = %q", umc.userManagementEndpoint, idToolkitV1Endpoint)
	}
	if umc.providerConfigEndpoint != providerConfigEndpoint {
		return fmt.Errorf("providerConfigEndpoint = %q; want = %q", umc.providerConfigEndpoint, providerConfigEndpoint)
	}
	if umc.projectID != wantProjectID {
		return fmt.Errorf("projectID = %q; want = %q", umc.projectID, wantProjectID)
	}

	req, err := http.NewRequest(http.MethodGet, "https://firebase.google.com", nil)
	if err != nil {
		return err
	}

	for _, opt := range umc.httpClient.Opts {
		opt(req)
	}
	version := req.Header.Get("X-Client-Version")
	wantVersion := fmt.Sprintf("Go/Admin/%s", testVersion)
	if version != wantVersion {
		return fmt.Errorf("version = %q; want = %q", version, wantVersion)
	}

	return nil
}

func verifyCustomToken(
	ctx context.Context, token string, expected map[string]interface{}, tenantID string) error {

	if err := testIDTokenVerifier.verifySignature(ctx, token); err != nil {
		return err
	}

	var (
		header  jwtHeader
		payload customToken
	)
	segments := strings.Split(token, ".")
	if err := decode(segments[0], &header); err != nil {
		return err
	}
	if err := decode(segments[1], &payload); err != nil {
		return err
	}

	email, err := testSigner.Email(ctx)
	if err != nil {
		return err
	}

	if header.Algorithm != "RS256" {
		return fmt.Errorf("Algorithm: %q; want: 'RS256'", header.Algorithm)
	} else if header.Type != "JWT" {
		return fmt.Errorf("Type: %q; want: 'JWT'", header.Type)
	} else if payload.Aud != firebaseAudience {
		return fmt.Errorf("Audience: %q; want: %q", payload.Aud, firebaseAudience)
	} else if payload.Iss != email {
		return fmt.Errorf("Issuer: %q; want: %q", payload.Iss, email)
	} else if payload.Sub != email {
		return fmt.Errorf("Subject: %q; want: %q", payload.Sub, email)
	}

	now := testClock.Now().Unix()
	if payload.Exp != now+3600 {
		return fmt.Errorf("Exp: %d; want: %d", payload.Exp, now+3600)
	}
	if payload.Iat != now {
		return fmt.Errorf("Iat: %d; want: %d", payload.Iat, now)
	}

	for k, v := range expected {
		if payload.Claims[k] != v {
			return fmt.Errorf("Claim[%q]: %v; want: %v", k, payload.Claims[k], v)
		}
	}

	if payload.TenantID != tenantID {
		return fmt.Errorf("Tenant ID: %q; want: %q", payload.TenantID, tenantID)
	}

	return nil
}

func logFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
