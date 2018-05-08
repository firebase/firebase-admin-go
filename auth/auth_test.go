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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	"golang.org/x/oauth2/google"

	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest"

	"firebase.google.com/go/internal"
)

var (
	client                *Client
	ctx                   context.Context
	testIDToken           string
	testGetUserResponse   []byte
	testListUsersResponse []byte
)

var defaultTestOpts = []option.ClientOption{
	option.WithCredentialsFile("../testdata/service_account.json"),
}

func TestMain(m *testing.M) {
	var (
		err   error
		ks    keySource
		creds *google.DefaultCredentials
		opts  []option.ClientOption
	)
	if appengine.IsDevAppServer() {
		aectx, aedone, err := aetest.NewContext()
		if err != nil {
			log.Fatalln(err)
		}
		ctx = aectx
		defer aedone()

		ks, err = newAEKeySource(ctx)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		ctx = context.Background()
		opts = defaultTestOpts
		creds, err = transport.Creds(ctx, opts...)
		if err != nil {
			log.Fatalln(err)
		}

		ks = &fileKeySource{FilePath: "../testdata/public_certs.json"}
	}
	client, err = NewClient(ctx, &internal.AuthConfig{
		Creds:     creds,
		Opts:      opts,
		ProjectID: "mock-project-id",
	})
	if err != nil {
		log.Fatalln(err)
	}
	client.ks = ks

	testGetUserResponse, err = ioutil.ReadFile("../testdata/get_user.json")
	if err != nil {
		log.Fatalln(err)
	}

	testListUsersResponse, err = ioutil.ReadFile("../testdata/list_users.json")
	if err != nil {
		log.Fatalln(err)
	}

	testIDToken = getIDToken(nil)
	os.Exit(m.Run())
}

func TestNewClientInvalidCredentials(t *testing.T) {
	creds := &google.DefaultCredentials{
		JSON: []byte("foo"),
	}
	conf := &internal.AuthConfig{Creds: creds}
	if c, err := NewClient(ctx, conf); c != nil || err == nil {
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
	if c, err := NewClient(ctx, conf); c != nil || err == nil {
		t.Errorf("NewClient() = (%v,%v); want = (nil, error)", c, err)
	}
}

func TestCustomToken(t *testing.T) {
	token, err := client.CustomToken(ctx, "user1")
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(ctx, token, nil, t)
}

func TestCustomTokenWithClaims(t *testing.T) {
	claims := map[string]interface{}{
		"foo":     "bar",
		"premium": true,
		"count":   float64(123),
	}
	token, err := client.CustomTokenWithClaims(ctx, "user1", claims)
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(ctx, token, claims, t)
}

func TestCustomTokenWithNilClaims(t *testing.T) {
	token, err := client.CustomTokenWithClaims(ctx, "user1", nil)
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(ctx, token, nil, t)
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
		token, err := client.CustomTokenWithClaims(ctx, tc.uid, tc.claims)
		if token != "" || err == nil {
			t.Errorf("CustomTokenWithClaims(%q) = (%q, %v); want = (\"\", error)", tc.name, token, err)
		}
	}
}

func TestCustomTokenInvalidCredential(t *testing.T) {
	// AuthConfig with nil Creds
	conf := &internal.AuthConfig{Opts: defaultTestOpts}
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

	ft, err := s.Client.VerifyIDTokenAndCheckRevoked(ctx, testIDToken)
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

	ft, err := s.Client.VerifyIDToken(ctx, tok)
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

	p, err := s.Client.VerifyIDTokenAndCheckRevoked(ctx, tok)
	we := "ID token has been revoked"
	if p != nil || err == nil || err.Error() != we || !IsIDTokenRevoked(err) {
		t.Errorf("VerifyIDTokenAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, %v)",
			p, err, nil, we)
	}
}

func TestVerifyIDToken(t *testing.T) {
	ft, err := client.VerifyIDToken(ctx, testIDToken)
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

func TestVerifyIDTokenInvalidSignature(t *testing.T) {
	parts := strings.Split(testIDToken, ".")
	token := fmt.Sprintf("%s:%s:invalidsignature", parts[0], parts[1])
	if ft, err := client.VerifyIDToken(ctx, token); ft != nil || err == nil {
		t.Errorf("VerifyiDToken('invalid-signature') = (%v, %v); want = (nil, error)", ft, err)
	}
}

func TestVerifyIDTokenError(t *testing.T) {
	now := time.Now().Unix()
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
		{"FutureToken", getIDToken(mockIDTokenPayload{"iat": now + 1000})},
		{"ExpiredToken", getIDToken(mockIDTokenPayload{
			"iat": now - 1000,
			"exp": now - 100,
		})},
		{"EmptyToken", ""},
		{"BadFormatToken", "foobar"},
	}

	for _, tc := range cases {
		if _, err := client.VerifyIDToken(ctx, tc.token); err == nil {
			t.Errorf("VerifyIDToken(%q) = nil; want error", tc.name)
		}
	}
}

func TestNoProjectID(t *testing.T) {
	// AuthConfig with empty ProjectID
	conf := &internal.AuthConfig{Opts: defaultTestOpts}
	c, err := NewClient(ctx, conf)
	if err != nil {
		t.Fatal(err)
	}
	c.ks = client.ks
	if _, err := c.VerifyIDToken(ctx, testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCustomTokenVerification(t *testing.T) {
	token, err := client.CustomToken(ctx, "user1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.VerifyIDToken(ctx, token); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCertificateRequestError(t *testing.T) {
	ks := client.ks
	client.ks = &mockKeySource{nil, errors.New("mock error")}
	defer func() {
		client.ks = ks
	}()
	if _, err := client.VerifyIDToken(ctx, testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func verifyCustomToken(ctx context.Context, token string, expected map[string]interface{}, t *testing.T) {
	h := &jwtHeader{}
	p := &customToken{}
	if err := decodeToken(ctx, token, client.ks, h, p); err != nil {
		t.Fatal(err)
	}

	email, err := client.snr.Email(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if h.Algorithm != "RS256" {
		t.Errorf("Algorithm: %q; want: 'RS256'", h.Algorithm)
	} else if h.Type != "JWT" {
		t.Errorf("Type: %q; want: 'JWT'", h.Type)
	} else if p.Aud != firebaseAudience {
		t.Errorf("Audience: %q; want: %q", p.Aud, firebaseAudience)
	} else if p.Iss != email {
		t.Errorf("Issuer: %q; want: %q", p.Iss, email)
	} else if p.Sub != email {
		t.Errorf("Subject: %q; want: %q", p.Sub, email)
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
	h := jwtHeader{Algorithm: "RS256", Type: "JWT"}
	h.KeyID = kid
	token, err := encodeToken(ctx, client.snr, h, pCopy)
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

// aeKeySource provides access to the public keys associated with App Engine apps. This
// is used in tests to verify custom tokens and mock ID tokens when they are signed with
// App Engine private keys.
type aeKeySource struct {
	keys []*publicKey
}

func newAEKeySource(ctx context.Context) (keySource, error) {
	certs, err := appengine.PublicCertificates(ctx)
	if err != nil {
		return nil, err
	}
	keys := make([]*publicKey, len(certs))
	for i, cert := range certs {
		pk, err := parsePublicKey("mock-key-id-1", cert.Data)
		if err != nil {
			return nil, err
		}
		keys[i] = pk
	}
	return aeKeySource{keys}, nil
}

// Keys returns the RSA Public Keys managed by App Engine.
func (k aeKeySource) Keys(ctx context.Context) ([]*publicKey, error) {
	return k.keys, nil
}
