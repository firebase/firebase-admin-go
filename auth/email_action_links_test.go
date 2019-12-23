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
	"net/http"
	"reflect"
	"testing"
)

const (
	testActionLink       = "https://test.link"
	testActionLinkFormat = `{"oobLink": %q}`
	testEmail            = "user@domain.com"
)

var testActionLinkResponse = []byte(fmt.Sprintf(testActionLinkFormat, testActionLink))
var testActionCodeSettings = &ActionCodeSettings{
	URL:                   "https://example.dynamic.link",
	HandleCodeInApp:       true,
	DynamicLinkDomain:     "custom.page.link",
	IOSBundleID:           "com.example.ios",
	AndroidPackageName:    "com.example.android",
	AndroidInstallApp:     true,
	AndroidMinimumVersion: "6",
}
var testActionCodeSettingsMap = map[string]interface{}{
	"continueUrl":           "https://example.dynamic.link",
	"canHandleCodeInApp":    true,
	"dynamicLinkDomain":     "custom.page.link",
	"iOSBundleId":           "com.example.ios",
	"androidPackageName":    "com.example.android",
	"androidInstallApp":     true,
	"androidMinimumVersion": "6",
}
var invalidActionCodeSettings = []struct {
	name     string
	settings *ActionCodeSettings
	want     string
}{
	{
		"no-url",
		&ActionCodeSettings{},
		"URL must not be empty",
	},
	{
		"malformed-url",
		&ActionCodeSettings{
			URL: "not a url",
		},
		`malformed url string: "not a url"`,
	},
	{
		"no-android-package-1",
		&ActionCodeSettings{
			URL:               "https://example.dynamic.link",
			AndroidInstallApp: true,
		},
		"Android package name is required when specifying other Android settings",
	},
	{
		"no-android-package-2",
		&ActionCodeSettings{
			URL:                   "https://example.dynamic.link",
			AndroidMinimumVersion: "6",
		},
		"Android package name is required when specifying other Android settings",
	},
}

func TestEmailVerificationLink(t *testing.T) {
	s := echoServer(testActionLinkResponse, t)
	defer s.Close()

	link, err := s.Client.EmailVerificationLink(context.Background(), testEmail)
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
	if err := checkActionLinkRequest(want, s); err != nil {
		t.Fatalf("EmailVerificationLink() %v", err)
	}
}

func TestEmailVerificationLinkWithSettings(t *testing.T) {
	s := echoServer(testActionLinkResponse, t)
	defer s.Close()

	link, err := s.Client.EmailVerificationLinkWithSettings(context.Background(), testEmail, testActionCodeSettings)
	if err != nil {
		t.Fatal(err)
	}
	if link != testActionLink {
		t.Errorf("EmailVerificationLinkWithSettings() = %q; want = %q", link, testActionLink)
	}

	want := map[string]interface{}{
		"requestType":   "VERIFY_EMAIL",
		"email":         testEmail,
		"returnOobLink": true,
	}
	for k, v := range testActionCodeSettingsMap {
		want[k] = v
	}
	if err := checkActionLinkRequest(want, s); err != nil {
		t.Fatalf("EmailVerificationLinkWithSettings() %v", err)
	}
}

func TestPasswordResetLink(t *testing.T) {
	s := echoServer(testActionLinkResponse, t)
	defer s.Close()

	link, err := s.Client.PasswordResetLink(context.Background(), testEmail)
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
	if err := checkActionLinkRequest(want, s); err != nil {
		t.Fatalf("PasswordResetLink() %v", err)
	}
}

func TestPasswordResetLinkWithSettings(t *testing.T) {
	s := echoServer(testActionLinkResponse, t)
	defer s.Close()

	link, err := s.Client.PasswordResetLinkWithSettings(context.Background(), testEmail, testActionCodeSettings)
	if err != nil {
		t.Fatal(err)
	}
	if link != testActionLink {
		t.Errorf("PasswordResetLinkWithSettings() = %q; want = %q", link, testActionLink)
	}

	want := map[string]interface{}{
		"requestType":   "PASSWORD_RESET",
		"email":         testEmail,
		"returnOobLink": true,
	}
	for k, v := range testActionCodeSettingsMap {
		want[k] = v
	}
	if err := checkActionLinkRequest(want, s); err != nil {
		t.Fatalf("PasswordResetLinkWithSettings() %v", err)
	}
}

func TestEmailSignInLink(t *testing.T) {
	s := echoServer(testActionLinkResponse, t)
	defer s.Close()

	link, err := s.Client.EmailSignInLink(context.Background(), testEmail, testActionCodeSettings)
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
	if err := checkActionLinkRequest(want, s); err != nil {
		t.Fatalf("EmailSignInLink() %v", err)
	}
}

func TestEmailActionLinkNoEmail(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}

	if _, err := client.EmailVerificationLink(context.Background(), ""); err == nil {
		t.Errorf("EmailVerificationLink('') = nil; want error")
	}

	if _, err := client.PasswordResetLink(context.Background(), ""); err == nil {
		t.Errorf("PasswordResetLink('') = nil; want error")
	}

	if _, err := client.EmailSignInLink(context.Background(), "", testActionCodeSettings); err == nil {
		t.Errorf("EmailSignInLink('') = nil; want error")
	}
}

func TestEmailVerificationLinkInvalidSettings(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}
	for _, tc := range invalidActionCodeSettings {
		_, err := client.EmailVerificationLinkWithSettings(context.Background(), testEmail, tc.settings)
		if err == nil || err.Error() != tc.want {
			t.Errorf("EmailVerificationLinkWithSettings(%q) = %v; want = %q", tc.name, err, tc.want)
		}
	}
}

func TestPasswordResetLinkInvalidSettings(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}
	for _, tc := range invalidActionCodeSettings {
		_, err := client.PasswordResetLinkWithSettings(context.Background(), testEmail, tc.settings)
		if err == nil || err.Error() != tc.want {
			t.Errorf("PasswordResetLinkWithSettings(%q) = %v; want = %q", tc.name, err, tc.want)
		}
	}
}

func TestEmailSignInLinkInvalidSettings(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}
	for _, tc := range invalidActionCodeSettings {
		_, err := client.EmailSignInLink(context.Background(), testEmail, tc.settings)
		if err == nil || err.Error() != tc.want {
			t.Errorf("EmailSignInLink(%q) = %v; want = %q", tc.name, err, tc.want)
		}
	}
}

func TestEmailSignInLinkNoSettings(t *testing.T) {
	client := &Client{
		baseClient: &baseClient{},
	}
	_, err := client.EmailSignInLink(context.Background(), testEmail, nil)
	if err == nil {
		t.Errorf("EmailSignInLink(nil) = %v; want = error", err)
	}
}

func TestEmailVerificationLinkError(t *testing.T) {
	cases := map[string]func(error) bool{
		"UNAUTHORIZED_DOMAIN":         IsUnauthorizedContinueURI,
		"INVALID_DYNAMIC_LINK_DOMAIN": IsInvalidDynamicLinkDomain,
	}
	s := echoServer(testActionLinkResponse, t)
	defer s.Close()
	s.Client.baseClient.httpClient.RetryConfig = nil
	s.Status = http.StatusInternalServerError

	for code, check := range cases {
		resp := fmt.Sprintf(`{"error": {"message": %q}}`, code)
		s.Resp = []byte(resp)
		_, err := s.Client.EmailVerificationLink(context.Background(), testEmail)
		if err == nil || !check(err) {
			t.Errorf("EmailVerificationLink(%q) = %v; want = %q", code, err, serverError[code])
		}
	}
}

func checkActionLinkRequest(want map[string]interface{}, s *mockAuthServer) error {
	wantURL := "/projects/mock-project-id/accounts:sendOobCode"
	return checkActionLinkRequestWithURL(want, wantURL, s)
}

func checkActionLinkRequestWithURL(want map[string]interface{}, wantURL string, s *mockAuthServer) error {
	req := s.Req[0]
	if req.Method != http.MethodPost {
		return fmt.Errorf("Method = %q; want = %q", req.Method, http.MethodPatch)
	}

	if req.URL.Path != wantURL {
		return fmt.Errorf("URL = %q; want = %q", req.URL.Path, wantURL)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(s.Rbody, &got); err != nil {
		return err
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Body = %#v; want = %#v", got, want)
	}
	return nil
}
