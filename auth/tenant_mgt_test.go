// Copyright 2019 Google Inc. All Rights Reserved.
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
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/iterator"
)

func TestAuthForTenantEmptyTenantID(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("")
	if client != nil || err == nil {
		t.Errorf("AuthForTenant() = (%v, %v); want = (nil, error)", client, err)
	}
}

func TestTenantID(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	const want = "tenantID"
	tenantID := client.TenantID()
	if tenantID != want {
		t.Errorf("TenantID() = %q; want = %q", tenantID, want)
	}

	if client.baseClient.tenantID != want {
		t.Errorf("baseClient.tenantID = %q; want = %q", client.baseClient.tenantID, want)
	}
}

func TestTenantGetUser(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	user, err := client.GetUser(context.Background(), "ignored_id")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(user, testUser) {
		t.Errorf("GetUser() = %#v; want = %#v", user, testUser)
	}

	want := `{"localId":["ignored_id"]}`
	got := string(s.Rbody)
	if got != want {
		t.Errorf("GetUser() Req = %v; want = %v", got, want)
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:lookup"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("GetUser() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestTenantGetUserByEmail(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	user, err := client.GetUserByEmail(context.Background(), "test@email.com")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(user, testUser) {
		t.Errorf("GetUserByEmail() = %#v; want = %#v", user, testUser)
	}

	want := `{"email":["test@email.com"]}`
	got := string(s.Rbody)
	if got != want {
		t.Errorf("GetUserByEmail() Req = %v; want = %v", got, want)
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:lookup"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("GetUserByEmail() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestTenantGetUserByPhoneNumber(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	user, err := client.GetUserByPhoneNumber(context.Background(), "+1234567890")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(user, testUser) {
		t.Errorf("GetUserByPhoneNumber() = %#v; want = %#v", user, testUser)
	}

	want := `{"phoneNumber":["+1234567890"]}`
	got := string(s.Rbody)
	if got != want {
		t.Errorf("GetUserByPhoneNumber() Req = %v; want = %v", got, want)
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:lookup"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("GetUserByPhoneNumber() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestTenantListUsers(t *testing.T) {
	testListUsersResponse, err := ioutil.ReadFile("../testdata/list_users.json")
	if err != nil {
		t.Fatal(err)
	}
	s := echoServer(testListUsersResponse, t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	want := []*ExportedUserRecord{
		{UserRecord: testUser, PasswordHash: "passwordhash1", PasswordSalt: "salt1"},
		{UserRecord: testUser, PasswordHash: "passwordhash2", PasswordSalt: "salt2"},
		{UserRecord: testUser, PasswordHash: "passwordhash3", PasswordSalt: "salt3"},
	}

	testIterator := func(iter *UserIterator, token string, req string) {
		count := 0
		for i := 0; i < len(want); i++ {
			user, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(user.UserRecord, want[i].UserRecord) {
				t.Errorf("Users(%q) = %#v; want = %#v", token, user, want[i])
			}
			if user.PasswordHash != want[i].PasswordHash {
				t.Errorf("Users(%q) PasswordHash = %q; want = %q", token, user.PasswordHash, want[i].PasswordHash)
			}
			if user.PasswordSalt != want[i].PasswordSalt {
				t.Errorf("Users(%q) PasswordSalt = %q; want = %q", token, user.PasswordSalt, want[i].PasswordSalt)
			}
			count++
		}
		if count != len(want) {
			t.Errorf("Users(%q) = %d; want = %d", token, count, len(want))
		}
		if _, err := iter.Next(); err != iterator.Done {
			t.Errorf("Users(%q) = %v, want = %v", token, err, iterator.Done)
		}

		hr := s.Req[len(s.Req)-1]
		// Check the query string of the last HTTP request made.
		gotReq := hr.URL.Query().Encode()
		if gotReq != req {
			t.Errorf("Users(%q) = %q, want = %v", token, gotReq, req)
		}

		wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:batchGet"
		if hr.URL.Path != wantPath {
			t.Errorf("Users(%q) URL = %q; want = %q", token, hr.URL.Path, wantPath)
		}
	}

	testIterator(
		client.Users(context.Background(), ""),
		"",
		"maxResults=1000")
	testIterator(
		client.Users(context.Background(), "pageToken"),
		"pageToken",
		"maxResults=1000&nextPageToken=pageToken")
}

func TestTenantCreateUser(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts"
	for _, tc := range createUserCases {
		uid, err := client.createUser(context.Background(), tc.params)
		if uid != "expectedUserID" || err != nil {
			t.Errorf("createUser(%#v) = (%q, %v); want = (%q, nil)", tc.params, uid, err, "expectedUserID")
		}

		want, err := json.Marshal(tc.req)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(s.Rbody, want) {
			t.Errorf("createUser(%#v) request = %v; want = %v", tc.params, string(s.Rbody), string(want))
		}

		if s.Req[0].RequestURI != wantPath {
			t.Errorf("createUser(%#v) URL = %q; want = %q", tc.params, s.Req[0].RequestURI, wantPath)
		}
	}
}

func TestTenantUpdateUser(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SetAccountInfoResponse",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:update"
	for _, tc := range updateUserCases {
		err := client.updateUser(context.Background(), "uid", tc.params)
		if err != nil {
			t.Errorf("updateUser(%v) = %v; want = nil", tc.params, err)
		}

		tc.req["localId"] = "uid"
		want, err := json.Marshal(tc.req)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(s.Rbody, want) {
			t.Errorf("updateUser() request = %v; want = %v", string(s.Rbody), string(want))
		}

		if s.Req[0].RequestURI != wantPath {
			t.Errorf("updateUser(%#v) URL = %q; want = %q", tc.params, s.Req[0].RequestURI, wantPath)
		}
	}
}

func TestTenantRevokeRefreshTokens(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SetAccountInfoResponse",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	tc, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	before := time.Now().Unix()
	if err := tc.RevokeRefreshTokens(context.Background(), "some_uid"); err != nil {
		t.Error(err)
	}

	after := time.Now().Unix()
	var req struct {
		ValidSince string `json:"validSince"`
	}
	if err := json.Unmarshal(s.Rbody, &req); err != nil {
		t.Fatal(err)
	}

	validSince, err := strconv.ParseInt(req.ValidSince, 10, 64)
	if err != nil {
		t.Fatal(err)
	}

	if validSince > after || validSince < before {
		t.Errorf("validSince = %d, expecting time between %d and %d", validSince, before, after)
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:update"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("RevokeRefreshTokens() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestTenantSetCustomUserClaims(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SetAccountInfoResponse",
		"localId": "uid"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:update"
	for _, tc := range setCustomUserClaimsCases {
		err := client.SetCustomUserClaims(context.Background(), "uid", tc)
		if err != nil {
			t.Errorf("SetCustomUserClaims(%v) = %v; want nil", tc, err)
		}

		input := tc
		if input == nil {
			input = map[string]interface{}{}
		}
		b, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}

		m := map[string]interface{}{
			"localId":          "uid",
			"customAttributes": string(b),
		}
		want, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(s.Rbody, want) {
			t.Errorf("SetCustomUserClaims() = %v; want = %v", string(s.Rbody), string(want))
		}

		hr := s.Req[len(s.Req)-1]
		if hr.RequestURI != wantPath {
			t.Errorf("RevokeRefreshTokens() URL = %q; want = %q", hr.RequestURI, wantPath)
		}
	}
}

func TestTenantImportUsers(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	users := []*UserToImport{
		(&UserToImport{}).UID("user1"),
		(&UserToImport{}).UID("user2"),
	}
	result, err := client.ImportUsers(context.Background(), users)
	if err != nil {
		t.Fatal(err)
	}

	if result.SuccessCount != 2 || result.FailureCount != 0 {
		t.Errorf("ImportUsers() = %#v; want = {SuccessCount: 2, FailureCount: 0}", result)
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:batchCreate"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("ImportUsers() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestTenantImportUsersWithHash(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	users := []*UserToImport{
		(&UserToImport{}).UID("user1").PasswordHash([]byte("password")),
		(&UserToImport{}).UID("user2"),
	}
	result, err := client.ImportUsers(context.Background(), users, WithHash(mockHash{
		key:        "key",
		saltSep:    ",",
		rounds:     8,
		memoryCost: 14,
	}))
	if err != nil {
		t.Fatal(err)
	}

	if result.SuccessCount != 2 || result.FailureCount != 0 {
		t.Errorf("ImportUsers() = %#v; want = {SuccessCount: 2, FailureCount: 0}", result)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(s.Rbody, &got); err != nil {
		t.Fatal(err)
	}

	want := map[string]interface{}{
		"hashAlgorithm": "MOCKHASH",
		"signerKey":     "key",
		"saltSeparator": ",",
		"rounds":        float64(8),
		"memoryCost":    float64(14),
	}
	for k, v := range want {
		gv, ok := got[k]
		if !ok || gv != v {
			t.Errorf("ImportUsers() request(%q) = %v; want = %v", k, gv, v)
		}
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:batchCreate"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("ImportUsers() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestTenantDeleteUser(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"email": "",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	if err := client.DeleteUser(context.Background(), "uid"); err != nil {
		t.Errorf("DeleteUser() = %v; want = nil", err)
	}

	wantPath := "/projects/mock-project-id/tenants/tenantID/accounts:delete"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("DeleteUser() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

const wantEmailActionURL = "/projects/mock-project-id/tenants/tenantID/accounts:sendOobCode"

func TestTenantEmailVerificationLink(t *testing.T) {
	s := echoServer(testActionLinkResponse, t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	link, err := client.EmailVerificationLink(context.Background(), testEmail)
	if err != nil {
		t.Fatal(err)
	}
	if link != testActionLink {
		t.Errorf("EmailVerificationLink() = %q; want = %q", link, testActionLink)
	}

	want := map[string]interface{}{
		"requestType":   "VERIFY_EMAIL",
		"email":         testEmail,
		"returnOobLink": true,
	}
	if err := checkActionLinkRequestWithURL(want, wantEmailActionURL, s); err != nil {
		t.Fatalf("EmailVerificationLink() %v", err)
	}
}

func TestTenantPasswordResetLink(t *testing.T) {
	s := echoServer(testActionLinkResponse, t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	link, err := client.PasswordResetLink(context.Background(), testEmail)
	if err != nil {
		t.Fatal(err)
	}
	if link != testActionLink {
		t.Errorf("PasswordResetLink() = %q; want = %q", link, testActionLink)
	}

	want := map[string]interface{}{
		"requestType":   "PASSWORD_RESET",
		"email":         testEmail,
		"returnOobLink": true,
	}
	if err := checkActionLinkRequestWithURL(want, wantEmailActionURL, s); err != nil {
		t.Fatalf("PasswordResetLink() %v", err)
	}
}

func TestTenantEmailSignInLink(t *testing.T) {
	s := echoServer(testActionLinkResponse, t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	link, err := client.EmailSignInLink(context.Background(), testEmail, testActionCodeSettings)
	if err != nil {
		t.Fatal(err)
	}
	if link != testActionLink {
		t.Errorf("EmailSignInLink() = %q; want = %q", link, testActionLink)
	}

	want := map[string]interface{}{
		"requestType":   "EMAIL_SIGNIN",
		"email":         testEmail,
		"returnOobLink": true,
	}
	for k, v := range testActionCodeSettingsMap {
		want[k] = v
	}
	if err := checkActionLinkRequestWithURL(want, wantEmailActionURL, s); err != nil {
		t.Fatalf("EmailSignInLink() %v", err)
	}
}

func TestTenantOIDCProviderConfig(t *testing.T) {
	s := echoServer([]byte(oidcConfigResponse), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	oidc, err := client.OIDCProviderConfig(context.Background(), "oidc.provider")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(oidc, oidcProviderConfig) {
		t.Errorf("OIDCProviderConfig() = %#v; want = %#v", oidc, oidcProviderConfig)
	}

	req := s.Req[0]
	if req.Method != http.MethodGet {
		t.Errorf("OIDCProviderConfig() Method = %q; want = %q", req.Method, http.MethodGet)
	}

	wantURL := "/projects/mock-project-id/tenants/tenantID/oauthIdpConfigs/oidc.provider"
	if req.URL.Path != wantURL {
		t.Errorf("OIDCProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestTenantCreateOIDCProviderConfig(t *testing.T) {
	s := echoServer([]byte(oidcConfigResponse), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	options := (&OIDCProviderConfigToCreate{}).
		ID(oidcProviderConfig.ID).
		DisplayName(oidcProviderConfig.DisplayName).
		Enabled(oidcProviderConfig.Enabled).
		ClientID(oidcProviderConfig.ClientID).
		Issuer(oidcProviderConfig.Issuer)
	oidc, err := client.CreateOIDCProviderConfig(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(oidc, oidcProviderConfig) {
		t.Errorf("CreateOIDCProviderConfig() = %#v; want = %#v", oidc, oidcProviderConfig)
	}

	wantBody := map[string]interface{}{
		"displayName": oidcProviderConfig.DisplayName,
		"enabled":     oidcProviderConfig.Enabled,
		"clientId":    oidcProviderConfig.ClientID,
		"issuer":      oidcProviderConfig.Issuer,
	}
	wantURL := "/projects/mock-project-id/tenants/tenantID/oauthIdpConfigs"
	if err := checkCreateOIDCConfigRequestWithURL(s, wantBody, wantURL); err != nil {
		t.Fatal(err)
	}
}

func TestTenantUpdateOIDCProviderConfig(t *testing.T) {
	s := echoServer([]byte(oidcConfigResponse), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	options := (&OIDCProviderConfigToUpdate{}).
		DisplayName(oidcProviderConfig.DisplayName).
		Enabled(oidcProviderConfig.Enabled).
		ClientID(oidcProviderConfig.ClientID).
		Issuer(oidcProviderConfig.Issuer)
	oidc, err := client.UpdateOIDCProviderConfig(context.Background(), "oidc.provider", options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(oidc, oidcProviderConfig) {
		t.Errorf("UpdateOIDCProviderConfig() = %#v; want = %#v", oidc, oidcProviderConfig)
	}

	wantBody := map[string]interface{}{
		"displayName": oidcProviderConfig.DisplayName,
		"enabled":     oidcProviderConfig.Enabled,
		"clientId":    oidcProviderConfig.ClientID,
		"issuer":      oidcProviderConfig.Issuer,
	}
	wantMask := []string{
		"clientId",
		"displayName",
		"enabled",
		"issuer",
	}
	wantURL := "/projects/mock-project-id/tenants/tenantID/oauthIdpConfigs/oidc.provider"
	if err := checkUpdateOIDCConfigRequestWithURL(s, wantBody, wantMask, wantURL); err != nil {
		t.Fatal(err)
	}
}

func TestTenantDeleteOIDCProviderConfig(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	if err := client.DeleteOIDCProviderConfig(context.Background(), "oidc.provider"); err != nil {
		t.Fatal(err)
	}

	req := s.Req[0]
	if req.Method != http.MethodDelete {
		t.Errorf("DeleteOIDCProviderConfig() Method = %q; want = %q", req.Method, http.MethodDelete)
	}

	wantURL := "/projects/mock-project-id/tenants/tenantID/oauthIdpConfigs/oidc.provider"
	if req.URL.Path != wantURL {
		t.Errorf("DeleteOIDCProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestTenantOIDCProviderConfigs(t *testing.T) {
	template := `{
                "oauthIdpConfigs": [
                    %s,
                    %s,
                    %s
                ],
                "nextPageToken": ""
        }`
	response := fmt.Sprintf(template, oidcConfigResponse, oidcConfigResponse, oidcConfigResponse)
	s := echoServer([]byte(response), t)
	defer s.Close()

	want := []*OIDCProviderConfig{
		oidcProviderConfig,
		oidcProviderConfig,
		oidcProviderConfig,
	}
	wantPath := "/projects/mock-project-id/tenants/tenantID/oauthIdpConfigs"

	testIterator := func(iter *OIDCProviderConfigIterator, token string, req string) {
		count := 0
		for i := 0; i < len(want); i++ {
			config, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(config, want[i]) {
				t.Errorf("OIDCProviderConfigs(%q) = %#v; want = %#v", token, config, want[i])
			}
			count++
		}
		if count != len(want) {
			t.Errorf("OIDCProviderConfigs(%q) = %d; want = %d", token, count, len(want))
		}
		if _, err := iter.Next(); err != iterator.Done {
			t.Errorf("OIDCProviderConfigs(%q) = %v; want = %v", token, err, iterator.Done)
		}

		url := s.Req[len(s.Req)-1].URL
		if url.Path != wantPath {
			t.Errorf("OIDCProviderConfigs(%q) = %q; want = %q", token, url.Path, wantPath)
		}

		// Check the query string of the last HTTP request made.
		gotReq := url.Query().Encode()
		if gotReq != req {
			t.Errorf("OIDCProviderConfigs(%q) = %q; want = %v", token, gotReq, req)
		}
	}

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	testIterator(
		client.OIDCProviderConfigs(context.Background(), ""),
		"",
		"pageSize=100")
	testIterator(
		client.OIDCProviderConfigs(context.Background(), "pageToken"),
		"pageToken",
		"pageSize=100&pageToken=pageToken")
}

func TestTenantSAMLProviderConfig(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	saml, err := client.SAMLProviderConfig(context.Background(), "saml.provider")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("SAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	req := s.Req[0]
	if req.Method != http.MethodGet {
		t.Errorf("SAMLProviderConfig() Method = %q; want = %q", req.Method, http.MethodGet)
	}

	wantURL := "/projects/mock-project-id/tenants/tenantID/inboundSamlConfigs/saml.provider"
	if req.URL.Path != wantURL {
		t.Errorf("SAMLProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestTenantCreateSAMLProviderConfig(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	options := (&SAMLProviderConfigToCreate{}).
		ID(samlProviderConfig.ID).
		DisplayName(samlProviderConfig.DisplayName).
		Enabled(samlProviderConfig.Enabled).
		IDPEntityID(samlProviderConfig.IDPEntityID).
		SSOURL(samlProviderConfig.SSOURL).
		RequestSigningEnabled(samlProviderConfig.RequestSigningEnabled).
		X509Certificates(samlProviderConfig.X509Certificates).
		RPEntityID(samlProviderConfig.RPEntityID).
		CallbackURL(samlProviderConfig.CallbackURL)
	saml, err := client.CreateSAMLProviderConfig(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("CreateSAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	wantBody := map[string]interface{}{
		"displayName": samlProviderConfig.DisplayName,
		"enabled":     samlProviderConfig.Enabled,
		"idpConfig": map[string]interface{}{
			"idpEntityId":     samlProviderConfig.IDPEntityID,
			"ssoUrl":          samlProviderConfig.SSOURL,
			"signRequest":     samlProviderConfig.RequestSigningEnabled,
			"idpCertificates": idpCertsMap,
		},
		"spConfig": map[string]interface{}{
			"spEntityId":  samlProviderConfig.RPEntityID,
			"callbackUri": samlProviderConfig.CallbackURL,
		},
	}
	wantURL := "/projects/mock-project-id/tenants/tenantID/inboundSamlConfigs"
	if err := checkCreateSAMLConfigRequestWithURL(s, wantBody, wantURL); err != nil {
		t.Fatal(err)
	}
}

func TestTenantUpdateSAMLProviderConfig(t *testing.T) {
	s := echoServer([]byte(samlConfigResponse), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	options := (&SAMLProviderConfigToUpdate{}).
		DisplayName(samlProviderConfig.DisplayName).
		Enabled(samlProviderConfig.Enabled).
		IDPEntityID(samlProviderConfig.IDPEntityID).
		SSOURL(samlProviderConfig.SSOURL).
		RequestSigningEnabled(samlProviderConfig.RequestSigningEnabled).
		X509Certificates(samlProviderConfig.X509Certificates).
		RPEntityID(samlProviderConfig.RPEntityID).
		CallbackURL(samlProviderConfig.CallbackURL)
	saml, err := client.UpdateSAMLProviderConfig(context.Background(), "saml.provider", options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(saml, samlProviderConfig) {
		t.Errorf("UpdateSAMLProviderConfig() = %#v; want = %#v", saml, samlProviderConfig)
	}

	wantBody := map[string]interface{}{
		"displayName": samlProviderConfig.DisplayName,
		"enabled":     samlProviderConfig.Enabled,
		"idpConfig": map[string]interface{}{
			"idpEntityId":     samlProviderConfig.IDPEntityID,
			"ssoUrl":          samlProviderConfig.SSOURL,
			"signRequest":     samlProviderConfig.RequestSigningEnabled,
			"idpCertificates": idpCertsMap,
		},
		"spConfig": map[string]interface{}{
			"spEntityId":  samlProviderConfig.RPEntityID,
			"callbackUri": samlProviderConfig.CallbackURL,
		},
	}
	wantMask := []string{
		"displayName",
		"enabled",
		"idpConfig.idpCertificates",
		"idpConfig.idpEntityId",
		"idpConfig.signRequest",
		"idpConfig.ssoUrl",
		"spConfig.callbackUri",
		"spConfig.spEntityId",
	}
	wantURL := "/projects/mock-project-id/tenants/tenantID/inboundSamlConfigs/saml.provider"
	if err := checkUpdateSAMLConfigRequestWithURL(s, wantBody, wantMask, wantURL); err != nil {
		t.Fatal(err)
	}
}

func TestTenantDeleteSAMLProviderConfig(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	if err := client.DeleteSAMLProviderConfig(context.Background(), "saml.provider"); err != nil {
		t.Fatal(err)
	}

	req := s.Req[0]
	if req.Method != http.MethodDelete {
		t.Errorf("DeleteSAMLProviderConfig() Method = %q; want = %q", req.Method, http.MethodDelete)
	}

	wantURL := "/projects/mock-project-id/tenants/tenantID/inboundSamlConfigs/saml.provider"
	if req.URL.Path != wantURL {
		t.Errorf("DeleteSAMLProviderConfig() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestTenantSAMLProviderConfigs(t *testing.T) {
	template := `{
                "inboundSamlConfigs": [
                    %s,
                    %s,
                    %s
                ],
                "nextPageToken": ""
        }`
	response := fmt.Sprintf(template, samlConfigResponse, samlConfigResponse, samlConfigResponse)
	s := echoServer([]byte(response), t)
	defer s.Close()

	want := []*SAMLProviderConfig{
		samlProviderConfig,
		samlProviderConfig,
		samlProviderConfig,
	}
	wantPath := "/projects/mock-project-id/tenants/tenantID/inboundSamlConfigs"

	testIterator := func(iter *SAMLProviderConfigIterator, token string, req string) {
		count := 0
		for i := 0; i < len(want); i++ {
			config, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(config, want[i]) {
				t.Errorf("SAMLProviderConfigs(%q) = %#v; want = %#v", token, config, want[i])
			}
			count++
		}
		if count != len(want) {
			t.Errorf("SAMLProviderConfigs(%q) = %d; want = %d", token, count, len(want))
		}
		if _, err := iter.Next(); err != iterator.Done {
			t.Errorf("SAMLProviderConfigs(%q) = %v; want = %v", token, err, iterator.Done)
		}

		url := s.Req[len(s.Req)-1].URL
		if url.Path != wantPath {
			t.Errorf("SAMLProviderConfigs(%q) = %q; want = %q", token, url.Path, wantPath)
		}

		// Check the query string of the last HTTP request made.
		gotReq := url.Query().Encode()
		if gotReq != req {
			t.Errorf("SAMLProviderConfigs(%q) = %q; want = %v", token, gotReq, req)
		}
	}

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	testIterator(
		client.SAMLProviderConfigs(context.Background(), ""),
		"",
		"pageSize=100")
	testIterator(
		client.SAMLProviderConfigs(context.Background(), "pageToken"),
		"pageToken",
		"pageSize=100&pageToken=pageToken")
}

func TestTenantVerifyIDToken(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	s.Client.TenantManager.base.idTokenVerifier = testIDTokenVerifier

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
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

	if ft.Firebase.SignInProvider != "custom" {
		t.Errorf("SignInProvider = %q; want = %q", ft.Firebase.SignInProvider, "custom")
	}
	if ft.Firebase.Tenant != "tenantID" {
		t.Errorf("Tenant = %q; want = %q", ft.Firebase.Tenant, "tenantID")
	}
}

func TestTenantVerifyIDTokenAndCheckRevoked(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	s.Client.TenantManager.base.idTokenVerifier = testIDTokenVerifier

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	idToken := getIDToken(mockIDTokenPayload{
		"firebase": map[string]interface{}{
			"tenant":           "tenantID",
			"sign_in_provider": "custom",
		},
	})
	ft, err := client.VerifyIDTokenAndCheckRevoked(context.Background(), idToken)
	if err != nil {
		t.Fatal(err)
	}

	if ft.Firebase.SignInProvider != "custom" {
		t.Errorf("SignInProvider = %q; want = %q", ft.Firebase.SignInProvider, "custom")
	}
	if ft.Firebase.Tenant != "tenantID" {
		t.Errorf("Tenant = %q; want = %q", ft.Firebase.Tenant, "tenantID")
	}

	wantURI := "/projects/mock-project-id/tenants/tenantID/accounts:lookup"
	if s.Req[0].RequestURI != wantURI {
		t.Errorf("VerifySessionCookieAndCheckRevoked() URL = %q; want = %q", s.Req[0].RequestURI, wantURI)
	}
}

func TestInvalidTenantVerifyIDToken(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()
	s.Client.TenantManager.base.idTokenVerifier = testIDTokenVerifier

	client, err := s.Client.TenantManager.AuthForTenant("tenantID")
	if err != nil {
		t.Fatalf("AuthForTenant() = %v", err)
	}

	idToken := getIDToken(mockIDTokenPayload{
		"firebase": map[string]interface{}{
			"tenant":           "invalidTenantID",
			"sign_in_provider": "custom",
		},
	})
	ft, err := client.VerifyIDToken(context.Background(), idToken)
	if ft != nil || err == nil || !IsTenantIDMismatch(err) {
		t.Errorf("VerifyIDToken() = (%v, %v); want = (nil, %q)", ft, err, tenantIDMismatch)
	}
}

const tenantResponse = `{
    "name":"projects/mock-project-id/tenants/tenantID",
    "displayName": "Test Tenant",
    "allowPasswordSignup": true,
    "enableEmailLinkSignin": true
}`

const tenantResponse2 = `{
    "name":"projects/mock-project-id/tenants/tenantID2",
    "displayName": "Test Tenant 2",
    "allowPasswordSignup": true,
    "enableEmailLinkSignin": true
}`

const tenantNotFoundResponse = `{
	"error": {
		"message": "TENANT_NOT_FOUND"
	}
}`

var testTenant = &Tenant{
	ID:                    "tenantID",
	DisplayName:           "Test Tenant",
	AllowPasswordSignUp:   true,
	EnableEmailLinkSignIn: true,
}

var testTenant2 = &Tenant{
	ID:                    "tenantID2",
	DisplayName:           "Test Tenant 2",
	AllowPasswordSignUp:   true,
	EnableEmailLinkSignIn: true,
}

func TestTenant(t *testing.T) {
	s := echoServer([]byte(tenantResponse), t)
	defer s.Close()

	client := s.Client
	tenant, err := client.TenantManager.Tenant(context.Background(), "tenantID")
	if err != nil {
		t.Fatalf("Tenant() = %v", err)
	}

	if !reflect.DeepEqual(tenant, testTenant) {
		t.Errorf("Tenant() = %#v; want = %#v", tenant, testTenant)
	}

	req := s.Req[0]
	if req.Method != http.MethodGet {
		t.Errorf("Tenant() Method = %q; want = %q", req.Method, http.MethodGet)
	}

	wantURL := "/projects/mock-project-id/tenants/tenantID"
	if req.URL.Path != wantURL {
		t.Errorf("Tenant() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestTenantEmptyID(t *testing.T) {
	tm := &TenantManager{}
	wantErr := "tenantID must not be empty"

	tenant, err := tm.Tenant(context.Background(), "")
	if tenant != nil || err == nil || err.Error() != wantErr {
		t.Errorf("Tenant('') = (%v, %v); want = (nil, %q)", tenant, err, wantErr)
	}
}

func TestTenantError(t *testing.T) {
	s := echoServer([]byte(tenantNotFoundResponse), t)
	defer s.Close()
	s.Status = http.StatusNotFound

	client := s.Client
	tenant, err := client.TenantManager.Tenant(context.Background(), "tenantID")
	if tenant != nil || err == nil || !IsTenantNotFound(err) {
		t.Errorf("Tenant() = (%v, %v); want = (nil, TenantNotFound)", tenant, err)
	}
}

func TestTenantNoProjectID(t *testing.T) {
	tm := &TenantManager{}
	want := "project id not available"
	if _, err := tm.Tenant(context.Background(), "tenantID"); err == nil || err.Error() != want {
		t.Errorf("Tenant() = %v; want = %q", err, want)
	}
}

func TestCreateTenant(t *testing.T) {
	s := echoServer([]byte(tenantResponse), t)
	defer s.Close()

	client := s.Client
	options := (&TenantToCreate{}).
		DisplayName(testTenant.DisplayName).
		AllowPasswordSignUp(testTenant.AllowPasswordSignUp).
		EnableEmailLinkSignIn(testTenant.EnableEmailLinkSignIn)
	tenant, err := client.TenantManager.CreateTenant(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(tenant, testTenant) {
		t.Errorf("CreateTenant() = %#v; want = %#v", tenant, testTenant)
	}

	wantBody := map[string]interface{}{
		"displayName":           testTenant.DisplayName,
		"allowPasswordSignup":   testTenant.AllowPasswordSignUp,
		"enableEmailLinkSignin": testTenant.EnableEmailLinkSignIn,
	}
	if err := checkCreateTenantRequest(s, wantBody); err != nil {
		t.Fatal(err)
	}
}

func TestCreateTenantMinimal(t *testing.T) {
	s := echoServer([]byte(tenantResponse), t)
	defer s.Close()

	client := s.Client
	tenant, err := client.TenantManager.CreateTenant(context.Background(), &TenantToCreate{})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(tenant, testTenant) {
		t.Errorf("CreateTenant() = %#v; want = %#v", tenant, testTenant)
	}

	wantBody := map[string]interface{}{}
	if err := checkCreateTenantRequest(s, wantBody); err != nil {
		t.Fatal(err)
	}
}

func TestCreateTenantZeroValues(t *testing.T) {
	s := echoServer([]byte(tenantResponse), t)
	defer s.Close()
	client := s.Client

	options := (&TenantToCreate{}).
		DisplayName("").
		AllowPasswordSignUp(false).
		EnableEmailLinkSignIn(false)
	tenant, err := client.TenantManager.CreateTenant(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(tenant, testTenant) {
		t.Errorf("CreateTenant() = %#v; want = %#v", tenant, testTenant)
	}

	wantBody := map[string]interface{}{
		"displayName":           "",
		"allowPasswordSignup":   false,
		"enableEmailLinkSignin": false,
	}
	if err := checkCreateTenantRequest(s, wantBody); err != nil {
		t.Fatal(err)
	}
}

func TestCreateTenantError(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	s.Status = http.StatusInternalServerError
	defer s.Close()

	client := s.Client
	client.TenantManager.httpClient.RetryConfig = nil
	tenant, err := client.TenantManager.CreateTenant(context.Background(), &TenantToCreate{})
	if tenant != nil || !IsUnknown(err) {
		t.Errorf("CreateTenant() = (%v, %v); want = (nil, %q)", tenant, err, "unknown-error")
	}
}

func TestCreateTenantNilOptions(t *testing.T) {
	tm := &TenantManager{}
	want := "tenant must not be nil"
	if _, err := tm.CreateTenant(context.Background(), nil); err == nil || err.Error() != want {
		t.Errorf("CreateTenant(nil) = %v, want = %q", err, want)
	}
}

func TestUpdateTenant(t *testing.T) {
	s := echoServer([]byte(tenantResponse), t)
	defer s.Close()

	client := s.Client
	options := (&TenantToUpdate{}).
		DisplayName(testTenant.DisplayName).
		AllowPasswordSignUp(testTenant.AllowPasswordSignUp).
		EnableEmailLinkSignIn(testTenant.EnableEmailLinkSignIn)
	tenant, err := client.TenantManager.UpdateTenant(context.Background(), "tenantID", options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(tenant, testTenant) {
		t.Errorf("UpdateTenant() = %#v; want = %#v", tenant, testTenant)
	}

	wantBody := map[string]interface{}{
		"displayName":           testTenant.DisplayName,
		"allowPasswordSignup":   testTenant.AllowPasswordSignUp,
		"enableEmailLinkSignin": testTenant.EnableEmailLinkSignIn,
	}
	wantMask := []string{"allowPasswordSignup", "displayName", "enableEmailLinkSignin"}
	if err := checkUpdateTenantRequest(s, wantBody, wantMask); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateTenantMinimal(t *testing.T) {
	s := echoServer([]byte(tenantResponse), t)
	defer s.Close()

	client := s.Client
	options := (&TenantToUpdate{}).DisplayName(testTenant.DisplayName)
	tenant, err := client.TenantManager.UpdateTenant(context.Background(), "tenantID", options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(tenant, testTenant) {
		t.Errorf("UpdateTenant() = %#v; want = %#v", tenant, testTenant)
	}

	wantBody := map[string]interface{}{
		"displayName": testTenant.DisplayName,
	}
	wantMask := []string{"displayName"}
	if err := checkUpdateTenantRequest(s, wantBody, wantMask); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateTenantZeroValues(t *testing.T) {
	s := echoServer([]byte(tenantResponse), t)
	defer s.Close()
	client := s.Client

	options := (&TenantToUpdate{}).
		DisplayName("").
		AllowPasswordSignUp(false).
		EnableEmailLinkSignIn(false)
	tenant, err := client.TenantManager.UpdateTenant(context.Background(), "tenantID", options)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(tenant, testTenant) {
		t.Errorf("UpdateTenant() = %#v; want = %#v", tenant, testTenant)
	}

	wantBody := map[string]interface{}{
		"displayName":           "",
		"allowPasswordSignup":   false,
		"enableEmailLinkSignin": false,
	}
	wantMask := []string{"allowPasswordSignup", "displayName", "enableEmailLinkSignin"}
	if err := checkUpdateTenantRequest(s, wantBody, wantMask); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateTenantError(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	s.Status = http.StatusInternalServerError
	defer s.Close()

	client := s.Client
	client.TenantManager.httpClient.RetryConfig = nil
	options := (&TenantToUpdate{}).DisplayName("")
	tenant, err := client.TenantManager.UpdateTenant(context.Background(), "tenantID", options)
	if tenant != nil || !IsUnknown(err) {
		t.Errorf("UpdateTenant() = (%v, %v); want = (nil, %q)", tenant, err, "unknown-error")
	}
}

func TestUpdateTenantEmptyID(t *testing.T) {
	tm := &TenantManager{}
	want := "tenantID must not be empty"
	options := (&TenantToUpdate{}).DisplayName("")
	if _, err := tm.UpdateTenant(context.Background(), "", options); err == nil || err.Error() != want {
		t.Errorf("UpdateTenant(nil) = %v, want = %q", err, want)
	}
}

func TestUpdateTenantNilOptions(t *testing.T) {
	tm := &TenantManager{}
	want := "tenant must not be nil"
	if _, err := tm.UpdateTenant(context.Background(), "tenantID", nil); err == nil || err.Error() != want {
		t.Errorf("UpdateTenant(nil) = %v, want = %q", err, want)
	}
}

func TestUpdateTenantEmptyOptions(t *testing.T) {
	tm := &TenantManager{}
	want := "no parameters specified in the update request"
	if _, err := tm.UpdateTenant(context.Background(), "tenantID", &TenantToUpdate{}); err == nil || err.Error() != want {
		t.Errorf("UpdateTenant({}) = %v, want = %q", err, want)
	}
}

func TestDeleteTenant(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	client := s.Client
	if err := client.TenantManager.DeleteTenant(context.Background(), "tenantID"); err != nil {
		t.Fatalf("DeleteTenant() = %v", err)
	}

	req := s.Req[0]
	if req.Method != http.MethodDelete {
		t.Errorf("DeleteTenant() Method = %q; want = %q", req.Method, http.MethodDelete)
	}

	wantURL := "/projects/mock-project-id/tenants/tenantID"
	if req.URL.Path != wantURL {
		t.Errorf("DeleteTenant() URL = %q; want = %q", req.URL.Path, wantURL)
	}
}

func TestDeleteTenantEmptyID(t *testing.T) {
	tm := &TenantManager{}
	wantErr := "tenantID must not be empty"

	err := tm.DeleteTenant(context.Background(), "")
	if err == nil || err.Error() != wantErr {
		t.Errorf("DeleteTenant('') = %v; want = (nil, %q)", err, wantErr)
	}
}

func TestDeleteTenantError(t *testing.T) {
	s := echoServer([]byte(tenantNotFoundResponse), t)
	defer s.Close()
	s.Status = http.StatusNotFound

	client := s.Client
	err := client.TenantManager.DeleteTenant(context.Background(), "tenantID")
	if err == nil || !IsTenantNotFound(err) {
		t.Errorf("DeleteTenant() = %v; want = TenantNotFound", err)
	}
}

func TestTenants(t *testing.T) {
	template := `{
                "tenants": [
                    %s,
                    %s,
                    %s
                ],
                "nextPageToken": ""
        }`
	response := fmt.Sprintf(template, tenantResponse, tenantResponse2, tenantResponse)
	s := echoServer([]byte(response), t)
	defer s.Close()

	want := []*Tenant{
		testTenant,
		testTenant2,
		testTenant,
	}
	wantPath := "/projects/mock-project-id/tenants"

	testIterator := func(iter *TenantIterator, token string, req string) {
		count := 0
		for i := 0; i < len(want); i++ {
			tenant, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tenant, want[i]) {
				t.Errorf("Tenants(%q) = %#v; want = %#v", token, tenant, want[i])
			}
			count++
		}
		if count != len(want) {
			t.Errorf("Tenants(%q) = %d; want = %d", token, count, len(want))
		}
		if _, err := iter.Next(); err != iterator.Done {
			t.Errorf("Tenants(%q) = %v; want = %v", token, err, iterator.Done)
		}

		url := s.Req[len(s.Req)-1].URL
		if url.Path != wantPath {
			t.Errorf("Tenants(%q) = %q; want = %q", token, url.Path, wantPath)
		}

		// Check the query string of the last HTTP request made.
		gotReq := url.Query().Encode()
		if gotReq != req {
			t.Errorf("Tenants(%q) = %q; want = %v", token, gotReq, req)
		}
	}

	client := s.Client
	testIterator(
		client.TenantManager.Tenants(context.Background(), ""),
		"",
		"pageSize=100")
	testIterator(
		client.TenantManager.Tenants(context.Background(), "pageToken"),
		"pageToken",
		"pageSize=100&pageToken=pageToken")
}

func TestTenantsError(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()
	s.Status = http.StatusInternalServerError

	client := s.Client
	client.TenantManager.httpClient.RetryConfig = nil
	it := client.TenantManager.Tenants(context.Background(), "")
	config, err := it.Next()
	if config != nil || err == nil || !IsUnknown(err) {
		t.Errorf("Tenants() = (%v, %v); want = (nil, %q)", config, err, "unknown-error")
	}
}

func checkCreateTenantRequest(s *mockAuthServer, wantBody interface{}) error {
	req := s.Req[0]
	if req.Method != http.MethodPost {
		return fmt.Errorf("CreateTenant() Method = %q; want = %q", req.Method, http.MethodPost)
	}

	wantURL := "/projects/mock-project-id/tenants"
	if req.URL.Path != wantURL {
		return fmt.Errorf("CreateTenant() URL = %q; want = %q", req.URL.Path, wantURL)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(s.Rbody, &body); err != nil {
		return err
	}

	if !reflect.DeepEqual(body, wantBody) {
		return fmt.Errorf("CreateTenant() Body = %#v; want = %#v", body, wantBody)
	}

	return nil
}

func checkUpdateTenantRequest(s *mockAuthServer, wantBody interface{}, wantMask []string) error {
	req := s.Req[0]
	if req.Method != http.MethodPatch {
		return fmt.Errorf("UpdateTenant() Method = %q; want = %q", req.Method, http.MethodPatch)
	}

	wantURL := "/projects/mock-project-id/tenants/tenantID"
	if req.URL.Path != wantURL {
		return fmt.Errorf("UpdateTenant() URL = %q; want = %q", req.URL.Path, wantURL)
	}

	queryParam := req.URL.Query().Get("updateMask")
	mask := strings.Split(queryParam, ",")
	sort.Strings(mask)
	if !reflect.DeepEqual(mask, wantMask) {
		return fmt.Errorf("UpdateTenant() Query = %#v; want = %#v", mask, wantMask)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(s.Rbody, &body); err != nil {
		return err
	}

	if !reflect.DeepEqual(body, wantBody) {
		return fmt.Errorf("UpdateTenant() Body = %#v; want = %#v", body, wantBody)
	}

	return nil
}
