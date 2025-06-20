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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"firebase.google.com/go/v4/app"
	"firebase.google.com/go/v4/errorutils"
	"firebase.google.com/go/v4/internal"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const (
	credEnvVar                 = "GOOGLE_APPLICATION_CREDENTIALS"
	testProjectID              = "mock-project-id"
	defaultIDToolkitV1Endpoint = "https://identitytoolkit.googleapis.com/v1"
	defaultIDToolkitV2Endpoint = "https://identitytoolkit.googleapis.com/v2"
)

var (
	testGetUserResponse         []byte
	testGetDisabledUserResponse []byte
	testIDToken                 string
	testSessionCookie           string
	testSigner                  cryptoSigner
	testIDTokenVerifier         *tokenVerifier
	testCookieVerifier          *tokenVerifier

	appOptsWithServiceAcct = []option.ClientOption{
		option.WithCredentialsFile("../testdata/service_account.json"),
	}
	appOptsWithTokenSource = []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{
			AccessToken: "test.token",
		}),
	}
	testClock = &internal.MockClock{Timestamp: time.Now()}
)

func newTestApp(ctx context.Context, projectID string, saID string, opts ...option.ClientOption) *app.App {
	appConfig := &app.Config{}
	if projectID != "" {
		appConfig.ProjectID = projectID
	}
	if saID != "" {
		appConfig.ServiceAccountID = saID
	}

	allOpts := []option.ClientOption{option.WithScopes(internal.FirebaseScopes...)}
	allOpts = append(allOpts, opts...)

	newApp, err := app.New(ctx, appConfig, allOpts...)
	if err != nil {
		log.Fatalf("Failed to create test app: %v", err)
	}
	return newApp
}

type authTestRequestData struct {
	Method     string
	Path       string
	Header     http.Header
	Body       []byte
	RequestURI string
}

func newAuthTestRequestData(r *http.Request, t *testing.T) authTestRequestData {
	var body []byte
	var err error
	if r.Body != nil {
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}
		r.Body.Close()
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	}
	return authTestRequestData{
		Method:     r.Method,
		Path:       r.URL.Path,
		Header:     r.Header,
		Body:       body,
		RequestURI: r.RequestURI,
	}
}

type mockEchoServer struct {
	*httptest.Server
	Client           *Client
	App              *app.App
	Rbody            []byte
	Req              []authTestRequestData
	Resp             []interface{}
	Force            error
	Status           int
	CustomHandler    http.HandlerFunc
}

func echoServer(respBodyBytes []byte, t *testing.T) *mockEchoServer {
	return echoServerWithParam(respBodyBytes, nil, t)
}

func echoServerWithParam(respBodyBytes []byte, headerParam map[string]string, t *testing.T) *mockEchoServer {
	var parsedResp interface{}
	if respBodyBytes != nil && len(respBodyBytes) > 0 {
		if err := json.Unmarshal(respBodyBytes, &parsedResp); err != nil {
			t.Logf("echoServer: could not unmarshal respBodyBytes as JSON: %v. Storing as string.", err)
			parsedResp = string(respBodyBytes)
		}
	} else if respBodyBytes == nil {
        parsedResp = "{}"
    } else {
		parsedResp = string(respBodyBytes)
	}

	s := &mockEchoServer{
		Resp: []interface{}{parsedResp},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.CustomHandler != nil {
			s.CustomHandler(w,r)
			return
		}

		reqData := newAuthTestRequestData(r, t)
		s.Req = append(s.Req, reqData)
		s.Rbody = reqData.Body

		if s.Force != nil {
			http.Error(w, s.Force.Error(), http.StatusInternalServerError)
			return
		}

		currentRespData := s.Resp[0]
		if len(s.Resp) > 1 {
			s.Resp = s.Resp[1:]
		}

		var out []byte
		var err error
		if respStr, isStr := currentRespData.(string); isStr {
			out = []byte(respStr)
		} else {
			out, err = json.Marshal(currentRespData)
			if err != nil {
				t.Fatalf("Failed to marshal response in mock server: %v", err)
			}
		}

		for k, v := range headerParam {
			w.Header().Set(k, v)
		}

		statusToReturn := http.StatusOK
		if s.Status != 0 {
			statusToReturn = s.Status
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusToReturn)
		w.Write(out)
	})
	s.Server = httptest.NewServer(handler)

	ctx := context.Background()
	appOptions := append(appOptsWithTokenSource, option.WithEndpoint(s.Server.URL))
	s.App = newTestApp(ctx, testProjectID, "", appOptions...)

	var err error
	s.Client, err = NewClient(ctx, s.App)
	if err != nil {
		s.Server.Close()
		t.Fatalf("Failed to create auth client for echo server: %v", err)
	}

	if s.Client.baseClient != nil {
		s.Client.baseClient.userManagementEndpoint = s.Server.URL
		s.Client.baseClient.providerConfigEndpoint = s.Server.URL
		s.Client.baseClient.projectMgtEndpoint = s.Server.URL
		if s.Client.TenantManager != nil {
			s.Client.TenantManager.endpoint = s.Server.URL
		}
		s.Client.baseClient.idTokenVerifier = testIDTokenVerifier
		s.Client.baseClient.cookieVerifier = testCookieVerifier
		s.Client.baseClient.clock = testClock
	} else {
		s.Server.Close()
		t.Fatal("auth.Client.baseClient is nil after NewClient")
	}
	return s
}

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

	testGetDisabledUserResponse, err = ioutil.ReadFile("../testdata/get_disabled_user.json")
	logFatal(err)

	testIDToken = getIDToken(nil)
	testSessionCookie = getSessionCookie(nil)
	os.Exit(m.Run())
}

func TestNewClientWithServiceAccountCredentials(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestApp(ctx, testProjectID, "", appOptsWithServiceAcct...)

	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := client.signer.(*serviceAccountSigner); !ok {
		t.Errorf("NewClient().signer = %#v; want = serviceAccountSigner", client.signer)
	}
	if err := checkIDTokenVerifier(client.idTokenVerifier, appInstance.ProjectID()); err != nil {
		t.Errorf("NewClient().idTokenVerifier: %v", err)
	}
	if err := checkCookieVerifier(client.cookieVerifier, appInstance.ProjectID()); err != nil {
		t.Errorf("NewClient().cookieVerifier: %v", err)
	}
	if err := checkBaseClient(client, appInstance.ProjectID(), appInstance.SDKVersion()); err != nil {
		t.Errorf("NewClient().baseClient: %v", err)
	}
	if client.clock != internal.SystemClock {
		t.Errorf("NewClient().clock = %v; want = SystemClock", client.clock)
	}
}

func TestNewClientWithoutCredentials(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestApp(ctx, "", "", appOptsWithTokenSource...)

	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := client.signer.(*iamSigner); !ok {
		t.Errorf("NewClient().signer = %#v; want = iamSigner", client.signer)
	}
	if err := checkIDTokenVerifier(client.idTokenVerifier, appInstance.ProjectID()); err != nil {
		t.Errorf("NewClient().idTokenVerifier = %v; want = nil", err)
	}
	if err := checkCookieVerifier(client.cookieVerifier, appInstance.ProjectID()); err != nil {
		t.Errorf("NewClient().cookieVerifier: %v", err)
	}
	if err := checkBaseClient(client, appInstance.ProjectID(), appInstance.SDKVersion()); err != nil {
		t.Errorf("NewClient().baseClient: %v", err)
	}
	if client.clock != internal.SystemClock {
		t.Errorf("NewClient().clock = %v; want = SystemClock", client.clock)
	}
}

func TestNewClientWithServiceAccountID(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestApp(ctx, "", "explicit-service-account", appOptsWithTokenSource...)

	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := client.signer.(*iamSigner); !ok {
		t.Errorf("NewClient().signer = %#v; want = iamSigner", client.signer)
	}
	if err := checkIDTokenVerifier(client.idTokenVerifier, appInstance.ProjectID()); err != nil {
		t.Errorf("NewClient().idTokenVerifier = %v; want = nil", err)
	}
	if err := checkCookieVerifier(client.cookieVerifier, appInstance.ProjectID()); err != nil {
		t.Errorf("NewClient().cookieVerifier: %v", err)
	}
	if err := checkBaseClient(client, appInstance.ProjectID(), appInstance.SDKVersion()); err != nil {
		t.Errorf("NewClient().baseClient: %v", err)
	}
	if client.clock != internal.SystemClock {
		t.Errorf("NewClient().clock = %v; want = SystemClock", client.clock)
	}

	email, err := client.signer.Email(context.Background())
	if email != appInstance.ServiceAccountID() || err != nil {
		t.Errorf("Email() = (%q, %v); want = (%q, nil)", email, err, appInstance.ServiceAccountID())
	}
}

func TestNewClientWithUserCredentials(t *testing.T) {
	creds := &google.DefaultCredentials{
		JSON: []byte(`{
			"client_id": "test-client",
			"client_secret": "test-secret"
		}`),
	}
	ctx := context.Background()
	appInstance := newTestApp(ctx, "", "", option.WithCredentials(creds))

	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := client.signer.(*iamSigner); !ok {
		t.Errorf("NewClient().signer = %#v; want = iamSigner", client.signer)
	}
	if err := checkIDTokenVerifier(client.idTokenVerifier, appInstance.ProjectID()); err != nil {
		t.Errorf("NewClient().idTokenVerifier = %v; want = nil", err)
	}
	if err := checkCookieVerifier(client.cookieVerifier, appInstance.ProjectID()); err != nil {
		t.Errorf("NewClient().cookieVerifier: %v", err)
	}
	if err := checkBaseClient(client, appInstance.ProjectID(), appInstance.SDKVersion()); err != nil {
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
	ctx := context.Background()
	appInstance, appErr := app.New(ctx, nil, option.WithCredentials(creds))
	if appErr == nil {
		if c, err := NewClient(ctx, appInstance); c != nil || err == nil {
			t.Errorf("NewClient() with bad JSON creds in app = (%v,%v); want = (nil, error)", c, err)
		}
	} else {
		t.Logf("App creation failed as expected with bad JSON creds: %v", appErr)
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
	ctx := context.Background()
	appInstance, appErr := app.New(ctx, nil, option.WithCredentials(creds))
	if appErr == nil {
		if c, err := NewClient(ctx, appInstance); c != nil || err == nil {
			t.Errorf("NewClient() with invalid private key in app = (%v,%v); want = (nil, error)", c, err)
		}
	} else {
		t.Logf("App creation failed as expected with invalid private key: %v", appErr)
	}
}

func TestNewClientAppDefaultCredentialsWithInvalidFile(t *testing.T) {
	current := os.Getenv(credEnvVar)

	if err := os.Setenv(credEnvVar, "../testdata/non_existing.json"); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv(credEnvVar, current)

	ctx := context.Background()
	appInstance, appErr := app.New(ctx, nil)
	if appErr == nil {
		if c, err := NewClient(ctx, appInstance); c != nil || err == nil {
			t.Errorf("NewClient() with non-existing ADC file = (%v, %v); want (nil, error)", c, err)
		}
	} else {
		t.Logf("App creation failed as expected with non-existing ADC file: %v", appErr)
	}
}

func TestNewClientInvalidCredentialFile(t *testing.T) {
	invalidFiles := []string{
		"testdata",
		"testdata/plain_text.txt",
	}

	ctx := context.Background()
	for _, testCase := range invalidFiles {
		appInstance, appErr := app.New(ctx, nil, option.WithCredentialsFile(testCase))
		if appErr == nil {
			if c, err := NewClient(ctx, appInstance); c != nil || err == nil {
				t.Errorf("NewClient() with invalid cred file %s = (%v, %v); want (nil, error)", testCase, c, err)
			}
		} else {
			t.Logf("App creation failed for %s as expected: %v", testCase, appErr)
		}
	}
}

func TestNewClientExplicitNoAuth(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestApp(ctx, "", "", option.WithoutAuthentication())
	if c, err := NewClient(ctx, appInstance); c == nil || err != nil {
		t.Errorf("NewClient() with NoAuth = (%v, %v); want (client, nil)", c, err)
	}
}

func TestNewClientEmulatorHostEnvVar(t *testing.T) {
	emulatorHost := "localhost:9099"
	idToolkitV1Endpoint := "http://localhost:9099/identitytoolkit.googleapis.com/v1"
	idToolkitV2Endpoint := "http://localhost:9099/identitytoolkit.googleapis.com/v2"

	os.Setenv(emulatorHostEnvVar, emulatorHost)
	defer os.Unsetenv(emulatorHostEnvVar)

	ctx := context.Background()
	appInstance := newTestApp(ctx, "", "", )
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	baseClient := client.baseClient
	if baseClient.userManagementEndpoint != idToolkitV1Endpoint {
		t.Errorf("baseClient.userManagementEndpoint = %q; want = %q", baseClient.userManagementEndpoint, idToolkitV1Endpoint)
	}
	if baseClient.providerConfigEndpoint != idToolkitV2Endpoint {
		t.Errorf("baseClient.providerConfigEndpoint = %q; want = %q", baseClient.providerConfigEndpoint, idToolkitV2Endpoint)
	}
	if baseClient.tenantMgtEndpoint != idToolkitV2Endpoint {
		t.Errorf("baseClient.tenantMgtEndpoint = %q; want = %q", baseClient.tenantMgtEndpoint, idToolkitV2Endpoint)
	}
	if _, ok := baseClient.signer.(emulatedSigner); !ok {
		t.Errorf("baseClient.signer = %#v; want = %#v", baseClient.signer, emulatedSigner{})
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
	appInstance := newTestApp(ctx, testProjectID, "", appOptsWithTokenSource...)
	s, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}

	if iamS, ok := s.signer.(*iamSigner); ok && iamS.httpClient != nil {
		iamS.httpClient.RetryConfig = nil
	} else {
		t.Log("Skipping RetryConfig nil assignment as signer is not an iamSigner with an HTTPClient for this test setup, or signer is nil.")
	}

	token, err := s.CustomToken(ctx, "user1")
	if token != "" || err == nil {
		t.Errorf("CustomToken() with potentially failing signer = (%q, %v); want = (\"\", error)", token, err)
	}

	token, err = s.CustomTokenWithClaims(ctx, "user1", map[string]interface{}{"foo": "bar"})
	if token != "" || err == nil {
		t.Errorf("CustomTokenWithClaims() with potentially failing signer = (%q, %v); want = (\"\", error)", token, err)
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

	ft, err := client.VerifyIDToken(context.Background(), token)
	if ft != nil || !IsIDTokenInvalid(err) {
		t.Errorf("VerifyIDToken('invalid-signature') = (%v, %v); want = (nil, IDTokenInvalid)", ft, err)
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
			if !IsIDTokenInvalid(err) || !strings.HasPrefix(err.Error(), tc.want) {
				t.Errorf("VerifyIDToken(%q) = %v; want = %q", tc.name, err, tc.want)
			}
			if tc.name == "ExpiredToken" && !IsIDTokenExpired(err) {
				t.Errorf("VerifyIDToken(%q) = %v; want = IDTokenExpired", tc.name, err)
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
	_, err = client.VerifyIDToken(context.Background(), token)
	if !IsIDTokenInvalid(err) {
		t.Errorf("VerifyIDToken(InvalidAlgorithm) = nil; want = IDTokenInvalid")
	}
}

func TestVerifyIDTokenWithNoProjectID(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestApp(ctx, "", "", appOptsWithTokenSource...)
	c, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	if c == nil || c.idTokenVerifier == nil {
		t.Fatalf("Client or its idTokenVerifier is nil after NewClient. App ProjectID: '%s'", appInstance.ProjectID())
	}

	originalKeySource := c.idTokenVerifier.keySource
	c.idTokenVerifier.keySource = testIDTokenVerifier.keySource

	_, verifyErr := c.VerifyIDToken(context.Background(), testIDToken)
	if verifyErr == nil {
		t.Errorf("VerifyIDToken() with no app project ID = nil; want error because audience check should fail or verifier setup should reflect no project ID.")
	} else {
		t.Logf("VerifyIDToken() with no app project ID got error as expected: %v", verifyErr)
	}
	c.idTokenVerifier.keySource = originalKeySource
}


func TestVerifyIDTokenUnsigned(t *testing.T) {
	token := getEmulatedIDToken(nil)

	client := &Client{
		baseClient: &baseClient{
			idTokenVerifier: testIDTokenVerifier,
		},
	}
	_, err := client.VerifyIDToken(context.Background(), token)
	if !IsIDTokenInvalid(err) {
		t.Errorf("VerifyIDToken(Unsigned) = %v; want = IDTokenInvalid", err)
	}
}

func TestEmulatorVerifyIDToken(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.idTokenVerifier = testIDTokenVerifier
		s.Client.baseClient.isEmulator = true
	} else {
		t.Fatal("echoServer did not initialize client or baseClient properly.")
	}


	token := getEmulatedIDToken(nil)
	ft, err := s.Client.VerifyIDToken(context.Background(), token)
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

func TestEmulatorVerifyIDTokenExpiredError(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.idTokenVerifier = testIDTokenVerifier
		s.Client.baseClient.isEmulator = true
	} else {
		t.Fatal("echoServer did not initialize client or baseClient properly.")
	}


	now := testClock.Now().Unix()
	token := getEmulatedIDToken(mockIDTokenPayload{
		"iat": now - 1000,
		"exp": now - clockSkewSeconds - 1,
	})

	_, err := s.Client.VerifyIDToken(context.Background(), token)
	if !IsIDTokenExpired(err) {
		t.Errorf("VerifyIDToken(Expired) = %v; want = IDTokenExpired", err)
	}
}

func TestEmulatorVerifyIDTokenUnreachableEmulator(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestApp(ctx, testProjectID, "", appOptsWithTokenSource...)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient.Client.Transport = eConnRefusedTransport{}
	client.httpClient.RetryConfig = nil
	client.isEmulator = true

	token := getEmulatedIDToken(nil)
	_, err = client.VerifyIDToken(context.Background(), token)
	if err == nil || !errorutils.IsUnavailable(err) || !strings.HasPrefix(err.Error(), "failed to establish a connection") {
		t.Errorf("VerifyIDToken(UnreachableEmulator) = %v; want = Unavailable", err)
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

	if _, err := client.VerifyIDToken(context.Background(), token); !IsIDTokenInvalid(err) {
		t.Error("VeridyIDToken() = nil; want = IDTokenInvalid")
	}
}

func TestCertificateRequestError(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestApp(ctx, testProjectID, "", appOptsWithServiceAcct...)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client == nil || client.idTokenVerifier == nil {
		t.Fatal("Client or idTokenVerifier is nil")
	}

	originalKeySource := client.idTokenVerifier.keySource
	client.idTokenVerifier.keySource = &mockKeySource{nil, errors.New("mock error")}
	defer func() { client.idTokenVerifier.keySource = originalKeySource }()

	if _, err := client.VerifyIDToken(context.Background(), testIDToken); !IsCertificateFetchFailed(err) {
		t.Errorf("VerifyIDToken() with failing keySource = %v; want = CertificateFetchFailed", err)
	}
}


func TestVerifyIDTokenAndCheckRevoked(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.idTokenVerifier = testIDTokenVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

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

	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.idTokenVerifier = testIDTokenVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

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

func TestInvalidTokenDoesNotCheckRevokedOrDisabled(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.idTokenVerifier = testIDTokenVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	ft, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), "")
	if ft != nil || !IsIDTokenInvalid(err) || IsIDTokenRevoked(err) || IsUserDisabled(err) {
		t.Errorf("VerifyIDTokenAndCheckRevoked() = (%v, %v); want = (nil, IDTokenInvalid)", ft, err)
	}
	if len(s.Req) != 0 {
		t.Errorf("Revocation checks = %d; want = 0", len(s.Req))
	}
}

func TestVerifyIDTokenAndCheckRevokedError(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	revokedToken := getIDToken(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.idTokenVerifier = testIDTokenVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	p, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), revokedToken)
	we := "ID token has been revoked"
	if p != nil || !IsIDTokenRevoked(err) || !IsIDTokenInvalid(err) || err.Error() != we {
		t.Errorf("VerifyIDTokenAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, %v)",
			p, err, nil, we)
	}
}

func TestVerifyIDTokenAndCheckDisabledError(t *testing.T) {
	s := echoServer(testGetDisabledUserResponse, t)
	defer s.Close()
	revokedToken := getIDToken(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.idTokenVerifier = testIDTokenVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	p, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), revokedToken)
	we := "user has been disabled"
	if p != nil || !IsUserDisabled(err) || !IsIDTokenInvalid(err) || err.Error() != we {
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
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.idTokenVerifier = testIDTokenVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	p, err := s.Client.VerifyIDTokenAndCheckRevoked(context.Background(), revokedToken)
	if p != nil || !IsUserNotFound(err) {
		t.Errorf("VerifyIDTokenAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, UserNotFound)", p, err, nil)
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
			if !IsSessionCookieInvalid(err) || !strings.HasPrefix(err.Error(), tc.want) {
				t.Errorf("VerifySessionCookie(%q) = %v; want = %q", tc.name, err, tc.want)
			}
			if tc.name == "ExpiredToken" && !IsSessionCookieExpired(err) {
				t.Errorf("VerifySessionCookie(%q) = %v; want = SessionCookieExpired", tc.name, err)
			}
		})
	}
}

func TestVerifySessionCookieDoesNotCheckRevoked(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	revokedCookie := getSessionCookie(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.cookieVerifier = testCookieVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

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
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.cookieVerifier = testCookieVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

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
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.cookieVerifier = testCookieVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	ft, err := s.Client.VerifySessionCookieAndCheckRevoked(context.Background(), "")
	if ft != nil || !IsSessionCookieInvalid(err) {
		t.Errorf("VerifySessionCookieAndCheckRevoked() = (%v, %v); want = (nil, SessionCookieInvalid)", ft, err)
	}
	if len(s.Req) != 0 {
		t.Errorf("Revocation checks = %d; want = 0", len(s.Req))
	}
}

func TestVerifySessionCookieAndCheckRevokedError(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	revokedCookie := getSessionCookie(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.cookieVerifier = testCookieVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	p, err := s.Client.VerifySessionCookieAndCheckRevoked(context.Background(), revokedCookie)
	we := "session cookie has been revoked"
	if p != nil || !IsSessionCookieRevoked(err) || !IsSessionCookieInvalid(err) || err.Error() != we {
		t.Errorf("VerifySessionCookieAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, %v)",
			p, err, nil, we)
	}
}

func TestVerifySessionCookieAndCheckDisabledError(t *testing.T) {
	s := echoServer(testGetDisabledUserResponse, t)
	defer s.Close()
	revokedCookie := getSessionCookie(mockIDTokenPayload{"uid": "uid", "iat": 1970})
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.cookieVerifier = testCookieVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	p, err := s.Client.VerifySessionCookieAndCheckRevoked(context.Background(), revokedCookie)
	we := "user has been disabled"
	if p != nil || !IsUserDisabled(err) || !IsSessionCookieInvalid(err) || err.Error() != we {
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
	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.cookieVerifier = testCookieVerifier
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	p, err := s.Client.VerifySessionCookieAndCheckRevoked(context.Background(), revokedCookie)
	if p != nil || !IsUserNotFound(err) {
		t.Errorf("VerifySessionCookieAndCheckRevoked(ctx, token) =(%v, %v); want = (%v, UserNotFound)", p, err, nil)
	}
}

func TestVerifySessionCookieUnsigned(t *testing.T) {
	token := getEmulatedSessionCookie(nil)

	client := &Client{
		baseClient: &baseClient{
			cookieVerifier: testCookieVerifier,
		},
	}
	_, err := client.VerifySessionCookie(context.Background(), token)
	if !IsSessionCookieInvalid(err) {
		t.Errorf("VerifySessionCookie(Unsigned) = %v; want = IDTokenInvalid", err)
	}
}

func TestEmulatorVerifySessionCookie(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.cookieVerifier = testCookieVerifier
		s.Client.baseClient.isEmulator = true
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	token := getEmulatedSessionCookie(nil)
	ft, err := s.Client.VerifySessionCookie(context.Background(), token)
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

func TestEmulatorVerifySessionCookieExpiredError(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	if s.Client != nil && s.Client.baseClient != nil {
		s.Client.baseClient.cookieVerifier = testCookieVerifier
		s.Client.baseClient.isEmulator = true
	} else {
		t.Fatal("echoServer client not properly initialized")
	}

	now := testClock.Now().Unix()
	token := getEmulatedSessionCookie(mockIDTokenPayload{
		"iat": now - 1000,
		"exp": now - clockSkewSeconds - 1,
	})

	_, err := s.Client.VerifySessionCookie(context.Background(), token)
	if !IsSessionCookieExpired(err) {
		t.Errorf("VerifySessionCookie(Expired) = %v; want = IDTokenExpired", err)
	}
}

func TestEmulatorVerifySessionCookieUnreachableEmulator(t *testing.T) {
	ctx := context.Background()
	appInstance := newTestApp(ctx, testProjectID, "", appOptsWithTokenSource...)
	client, err := NewClient(ctx, appInstance)
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient.Client.Transport = eConnRefusedTransport{}
	client.httpClient.RetryConfig = nil
	client.isEmulator = true

	token := getEmulatedSessionCookie(nil)
	_, err = client.VerifySessionCookie(context.Background(), token)
	if err == nil || !errorutils.IsUnavailable(err) || !strings.HasPrefix(err.Error(), "failed to establish a connection") {
		t.Errorf("VerifyIDToken(UnreachableEmulator) = %v; want = Unavailable", err)
	}
}

func signerForTests(ctx context.Context) (cryptoSigner, error) {
	creds, err := transport.Creds(ctx, appOptsWithServiceAcct...)
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

type eConnRefusedTransport struct{}

func (eConnRefusedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, syscall.ECONNREFUSED
}

func getSessionCookie(p mockIDTokenPayload) string {
	return getSessionCookieWithSigner(testSigner, p)
}

func getEmulatedSessionCookie(p mockIDTokenPayload) string {
	return getSessionCookieWithSigner(emulatedSigner{}, p)
}

func getSessionCookieWithSigner(signer cryptoSigner, p mockIDTokenPayload) string {
	pCopy := map[string]interface{}{
		"iss": "https://session.firebase.google.com/" + testProjectID,
	}
	for k, v := range p {
		pCopy[k] = v
	}
	return getIDTokenWithSigner(signer, pCopy)
}

func getIDTokenWithSigner(signer cryptoSigner, p mockIDTokenPayload) string {
	return getIDTokenWithSignerAndKid(signer, "mock-key-id-1", p)
}

func getIDToken(p mockIDTokenPayload) string {
	return getIDTokenWithSigner(testSigner, p)
}

func getIDTokenWithKid(kid string, p mockIDTokenPayload) string {
	return getIDTokenWithSignerAndKid(testSigner, kid, p)
}

func getEmulatedIDToken(p mockIDTokenPayload) string {
	return getIDTokenWithSignerAndKid(emulatedSigner{}, "mock-key-id-1", p)
}

func getIDTokenWithSignerAndKid(signer cryptoSigner, kid string, p mockIDTokenPayload) string {
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
			Algorithm: signer.Algorithm(),
			Type:      "JWT",
			KeyID:     kid,
		},
		payload: pCopy,
	}
	token, err := info.Token(context.Background(), signer)
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
	if tv.invalidTokenCode != idTokenInvalid {
		return fmt.Errorf("invalidTokenCode = %q; want = %q", tv.invalidTokenCode, idTokenInvalid)
	}
	if tv.expiredTokenCode != idTokenExpired {
		return fmt.Errorf("expiredTokenCode = %q; want = %q", tv.expiredTokenCode, idTokenExpired)
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
	if tv.invalidTokenCode != sessionCookieInvalid {
		return fmt.Errorf("invalidTokenCode = %q; want = %q", tv.invalidTokenCode, sessionCookieInvalid)
	}
	if tv.expiredTokenCode != sessionCookieExpired {
		return fmt.Errorf("expiredTokenCode = %q; want = %q", tv.expiredTokenCode, sessionCookieExpired)
	}
	return nil
}

func checkBaseClient(client *Client, wantProjectID string, sdkVersion string) error {
	baseClient := client.baseClient
	if baseClient.userManagementEndpoint != defaultIDToolkitV1Endpoint {
		return fmt.Errorf("userManagementEndpoint = %q; want = %q", baseClient.userManagementEndpoint, defaultIDToolkitV1Endpoint)
	}
	if baseClient.providerConfigEndpoint != defaultIDToolkitV2Endpoint {
		return fmt.Errorf("providerConfigEndpoint = %q; want = %q", baseClient.providerConfigEndpoint, defaultIDToolkitV2Endpoint)
	}
	if baseClient.tenantMgtEndpoint != defaultIDToolkitV2Endpoint {
		return fmt.Errorf("tenantMgtEndpoint = %q; want = %q", baseClient.tenantMgtEndpoint, defaultIDToolkitV2Endpoint)
	}
	if baseClient.projectID != wantProjectID {
		return fmt.Errorf("projectID = %q; want = %q", baseClient.projectID, wantProjectID)
	}

	req, err := http.NewRequest(http.MethodGet, "https://firebase.google.com", nil)
	if err != nil {
		return err
	}

	for _, opt := range baseClient.httpClient.Opts {
		opt(req)
	}
	versionHeader := req.Header.Get("X-Client-Version")
	wantVersionHeader := fmt.Sprintf("Go/Admin/%s", sdkVersion)
	if versionHeader != wantVersionHeader {
		return fmt.Errorf("X-Client-Version header = %q; want = %q", versionHeader, wantVersionHeader)
	}

	xGoogAPIClientHeader := internal.GetMetricsHeader(sdkVersion)
	if h := req.Header.Get("x-goog-api-client"); h != xGoogAPIClientHeader {
		return fmt.Errorf("x-goog-api-client header = %q; want = %q", h, xGoogAPIClientHeader)
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
		return fmt.Errorf("Subject: %q; want = %q", payload.Sub, email)
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
