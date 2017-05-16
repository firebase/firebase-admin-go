package auth

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/firebase/firebase-admin-go/credentials"
	"github.com/firebase/firebase-admin-go/internal"
)

var cred credentials.Credential
var impl Auth
var testIDToken string
var testKeys keySource

func verifyCustomToken(t *testing.T, token string, expected map[string]interface{}) {
	h := &jwtHeader{}
	p := &customToken{}
	if err := decodeToken(token, testKeys, h, p); err != nil {
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
	signer := cred.(Signer)
	pid := cred.(ProjectMember).ProjectID()
	pCopy := mockIDTokenPayload{
		"aud":   pid,
		"iss":   "https://securetoken.google.com/" + pid,
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
	token, _ := encodeToken(h, pCopy, signer)
	return token
}

type mockIDTokenPayload map[string]interface{}

func (p mockIDTokenPayload) decode(s string) error {
	return decode(s, &p)
}

type mockCredential struct{}

func (c *mockCredential) AccessToken(ctx context.Context) (string, time.Time, error) {
	return "mock-token", time.Now().Add(time.Hour), nil
}

type mockKeySource struct {
	keys []*publicKey
	err  error
}

func (t *mockKeySource) Keys() ([]*publicKey, error) {
	return t.keys, t.err
}

func TestMain(m *testing.M) {
	file, err := os.Open("../credentials/testdata/service_account.json")
	if err != nil {
		os.Exit(1)
	}
	defer file.Close()

	if cred, err = credentials.NewCert(file); err != nil {
		os.Exit(1)
	}
	impl = New(&internal.AppConf{Cred: cred})
	testIDToken = getIDToken(nil)
	testKeys = &fileKeySource{FilePath: "../credentials/testdata/public_certs.json"}
	os.Exit(m.Run())
}

func TestCustomToken(t *testing.T) {
	token, err := impl.CustomToken("user1")
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
	token, err := impl.CustomTokenWithClaims("user1", claims)
	if err != nil {
		t.Fatal(err)
	}
	verifyCustomToken(t, token, claims)
}

func TestCustomTokenWithNilClaims(t *testing.T) {
	token, err := impl.CustomTokenWithClaims("user1", nil)
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
		token, err := impl.CustomTokenWithClaims(tc.uid, tc.claims)
		if token != "" || err == nil {
			t.Errorf("CustomTokenWithClaims(%q) = (%q, %v); want: (\"\", error)", tc.name, token, err)
		}
	}
}

func TestCustomTokenInvalidCredential(t *testing.T) {
	c := &mockCredential{}
	s := New(&internal.AppConf{Cred: c})

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
	keys = testKeys
	ft, err := impl.VerifyIDToken(testIDToken)
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
	keys = testKeys
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
		if _, err := impl.VerifyIDToken(tc.token); err == nil {
			t.Errorf("VerifyyIDToken(%q) = nil; want error", tc.name)
		}
	}
}

func TestProjectIDEnvVariable(t *testing.T) {
	keys = testKeys
	projectID := os.Getenv(gcloudProject)
	defer os.Setenv(gcloudProject, projectID)

	if err := os.Setenv(gcloudProject, cred.(ProjectMember).ProjectID()); err != nil {
		t.Fatal(err)
	}
	c := &mockCredential{}
	a := New(&internal.AppConf{Cred: c})
	ft, err := a.VerifyIDToken(testIDToken)
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

func TestNoProjectID(t *testing.T) {
	keys = testKeys
	projectID := os.Getenv(gcloudProject)
	defer os.Setenv(gcloudProject, projectID)

	if err := os.Setenv(gcloudProject, ""); err != nil {
		t.Fatal(err)
	}
	c := &mockCredential{}
	s := New(&internal.AppConf{Cred: c})
	if _, err := s.VerifyIDToken(testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCustomTokenVerification(t *testing.T) {
	keys = testKeys
	token, err := impl.CustomToken("user1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := impl.VerifyIDToken(token); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}

func TestCertificateRequestError(t *testing.T) {
	keys = &mockKeySource{nil, errors.New("mock error")}
	if _, err := impl.VerifyIDToken(testIDToken); err == nil {
		t.Error("VeridyIDToken() = nil; want error")
	}
}
