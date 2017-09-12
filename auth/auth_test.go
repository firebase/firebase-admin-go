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

var client *Client
var testIDToken string

func TestMain(m *testing.M) {
	var (
		err   error
		ks    keySource
		ctx   context.Context
		creds *google.DefaultCredentials
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
		opt := option.WithCredentialsFile("../testdata/service_account.json")
		creds, err = transport.Creds(context.Background(), opt)
		if err != nil {
			log.Fatalln(err)
		}

		ks = &fileKeySource{FilePath: "../testdata/public_certs.json"}
	}

	client, err = NewClient(&internal.AuthConfig{
		Ctx:       ctx,
		Creds:     creds,
		ProjectID: "mock-project-id",
	})
	if err != nil {
		log.Fatalln(err)
	}
	client.ks = ks

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
	s, err := NewClient(&internal.AuthConfig{Ctx: context.Background()})
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
	c, err := NewClient(&internal.AuthConfig{Ctx: context.Background()})
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

func verifyCustomToken(t *testing.T, token string, expected map[string]interface{}) {
	h := &jwtHeader{}
	p := &customToken{}
	if err := decodeToken(token, client.ks, h, p); err != nil {
		t.Fatal(err)
	}

	email, err := client.snr.Email()
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
	h := defaultHeader()
	h.KeyID = kid
	token, err := encodeToken(client.snr, h, pCopy)
	if err != nil {
		log.Fatalln(err)
	}
	return token
}

type mockIDTokenPayload map[string]interface{}

func (p mockIDTokenPayload) decode(s string) error {
	return decode(s, &p)
}

// mockKeySource provides access to a set of in-memory public keys.
type mockKeySource struct {
	keys []*publicKey
	err  error
}

func (t *mockKeySource) Keys() ([]*publicKey, error) {
	return t.keys, t.err
}

// fileKeySource loads a set of public keys from the local file system.
type fileKeySource struct {
	FilePath   string
	CachedKeys []*publicKey
}

func (f *fileKeySource) Keys() ([]*publicKey, error) {
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
func (k aeKeySource) Keys() ([]*publicKey, error) {
	return k.keys, nil
}
