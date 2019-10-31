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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/internal"
	"google.golang.org/api/iterator"
)

var testUser = &UserRecord{
	UserInfo: &UserInfo{
		UID:         "testuser",
		Email:       "testuser@example.com",
		PhoneNumber: "+1234567890",
		DisplayName: "Test User",
		PhotoURL:    "http://www.example.com/testuser/photo.png",
		ProviderID:  defaultProviderID,
	},
	Disabled: false,

	EmailVerified: true,
	ProviderUserInfo: []*UserInfo{
		{
			ProviderID:  "password",
			DisplayName: "Test User",
			PhotoURL:    "http://www.example.com/testuser/photo.png",
			Email:       "testuser@example.com",
			UID:         "testuid",
		}, {
			ProviderID:  "phone",
			PhoneNumber: "+1234567890",
			UID:         "testuid",
		},
	},
	TokensValidAfterMillis: 1494364393000,
	UserMetadata: &UserMetadata{
		CreationTimestamp:  1234567890000,
		LastLogInTimestamp: 1233211232000,
	},
	CustomClaims: map[string]interface{}{"admin": true, "package": "gold"},
}

func TestGetUser(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	user, err := s.Client.GetUser(context.Background(), "ignored_id")
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

	wantPath := "/projects/mock-project-id/accounts:lookup"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("GetUser() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestGetUserByEmail(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	user, err := s.Client.GetUserByEmail(context.Background(), "test@email.com")
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

	wantPath := "/projects/mock-project-id/accounts:lookup"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("GetUserByEmail() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestGetUserByPhoneNumber(t *testing.T) {
	s := echoServer(testGetUserResponse, t)
	defer s.Close()

	user, err := s.Client.GetUserByPhoneNumber(context.Background(), "+1234567890")
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

	wantPath := "/projects/mock-project-id/accounts:lookup"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("GetUserByPhoneNumber() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestInvalidGetUser(t *testing.T) {
	client := &Client{
		userManagementClient: &userManagementClient{},
	}
	user, err := client.GetUser(context.Background(), "")
	if user != nil || err == nil {
		t.Errorf("GetUser('') = (%v, %v); want = (nil, error)", user, err)
	}
	user, err = client.GetUserByEmail(context.Background(), "")
	if user != nil || err == nil {
		t.Errorf("GetUserByEmail('') = (%v, %v); want = (nil, error)", user, err)
	}
	user, err = client.GetUserByPhoneNumber(context.Background(), "")
	if user != nil || err == nil {
		t.Errorf("GetUserPhoneNumber('') = (%v, %v); want = (nil, error)", user, err)
	}
}

func TestGetNonExistingUser(t *testing.T) {
	resp := `{
		"kind" : "identitytoolkit#GetAccountInfoResponse",
		"users" : []
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	we := `cannot find user from uid: "id-nonexisting"`
	user, err := s.Client.GetUser(context.Background(), "id-nonexisting")
	if user != nil || err == nil || err.Error() != we || !IsUserNotFound(err) {
		t.Errorf("GetUser(non-existing) = (%v, %q); want = (nil, %q)", user, err, we)
	}

	we = `cannot find user from email: "foo@bar.nonexisting"`
	user, err = s.Client.GetUserByEmail(context.Background(), "foo@bar.nonexisting")
	if user != nil || err == nil || err.Error() != we || !IsUserNotFound(err) {
		t.Errorf("GetUserByEmail(non-existing) = (%v, %q); want = (nil, %q)", user, err, we)
	}

	we = `cannot find user from phone number: "+12345678901"`
	user, err = s.Client.GetUserByPhoneNumber(context.Background(), "+12345678901")
	if user != nil || err == nil || err.Error() != we || !IsUserNotFound(err) {
		t.Errorf("GetUserPhoneNumber(non-existing) = (%v, %q); want = (nil, %q)", user, err, we)
	}
}

func TestListUsers(t *testing.T) {
	testListUsersResponse, err := ioutil.ReadFile("../testdata/list_users.json")
	if err != nil {
		t.Fatal(err)
	}
	s := echoServer(testListUsersResponse, t)
	defer s.Close()

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

		wantPath := "/projects/mock-project-id/accounts:batchGet"
		if hr.URL.Path != wantPath {
			t.Errorf("Users(%q) URL = %q; want = %q", token, hr.URL.Path, wantPath)
		}
	}
	testIterator(
		s.Client.Users(context.Background(), ""),
		"",
		"maxResults=1000")
	testIterator(
		s.Client.Users(context.Background(), "pageToken"),
		"pageToken",
		"maxResults=1000&nextPageToken=pageToken")
}

func TestInvalidCreateUser(t *testing.T) {
	cases := []struct {
		params *UserToCreate
		want   string
	}{
		{
			(&UserToCreate{}).Password("short"),
			"password must be a string at least 6 characters long",
		}, {
			(&UserToCreate{}).PhoneNumber(""),
			"phone number must be a non-empty string",
		}, {
			(&UserToCreate{}).PhoneNumber("1234"),
			"phone number must be a valid, E.164 compliant identifier",
		}, {
			(&UserToCreate{}).PhoneNumber("+_!@#$"),
			"phone number must be a valid, E.164 compliant identifier",
		}, {
			(&UserToCreate{}).UID(""),
			"uid must be a non-empty string",
		}, {
			(&UserToCreate{}).UID(strings.Repeat("a", 129)),
			"uid string must not be longer than 128 characters",
		}, {
			(&UserToCreate{}).DisplayName(""),
			"display name must be a non-empty string",
		}, {
			(&UserToCreate{}).PhotoURL(""),
			"photo url must be a non-empty string",
		}, {
			(&UserToCreate{}).Email(""),
			"email must be a non-empty string",
		}, {
			(&UserToCreate{}).Email("a"),
			`malformed email string: "a"`,
		}, {
			(&UserToCreate{}).Email("a@"),
			`malformed email string: "a@"`,
		}, {
			(&UserToCreate{}).Email("@a"),
			`malformed email string: "@a"`,
		}, {
			(&UserToCreate{}).Email("a@a@a"),
			`malformed email string: "a@a@a"`,
		},
	}
	client := &Client{}
	for i, tc := range cases {
		user, err := client.CreateUser(context.Background(), tc.params)
		if user != nil || err == nil {
			t.Errorf("[%d] CreateUser() = (%v, %v); want = (nil, error)", i, user, err)
		}
		if err.Error() != tc.want {
			t.Errorf("[%d] CreateUser() = %v; want = %v", i, err.Error(), tc.want)
		}
	}
}

var createUserCases = []struct {
	params *UserToCreate
	req    map[string]interface{}
}{
	{
		nil,
		map[string]interface{}{},
	},
	{
		&UserToCreate{},
		map[string]interface{}{},
	},
	{
		(&UserToCreate{}).Password("123456"),
		map[string]interface{}{"password": "123456"},
	},
	{
		(&UserToCreate{}).UID("1"),
		map[string]interface{}{"localId": "1"},
	},
	{
		(&UserToCreate{}).UID(strings.Repeat("a", 128)),
		map[string]interface{}{"localId": strings.Repeat("a", 128)},
	},
	{
		(&UserToCreate{}).PhoneNumber("+1"),
		map[string]interface{}{"phoneNumber": "+1"},
	},
	{
		(&UserToCreate{}).DisplayName("a"),
		map[string]interface{}{"displayName": "a"},
	},
	{
		(&UserToCreate{}).Email("a@a"),
		map[string]interface{}{"email": "a@a"},
	},
	{
		(&UserToCreate{}).Disabled(true),
		map[string]interface{}{"disabled": true},
	},
	{
		(&UserToCreate{}).Disabled(false),
		map[string]interface{}{"disabled": false},
	},
	{
		(&UserToCreate{}).EmailVerified(true),
		map[string]interface{}{"emailVerified": true},
	},
	{
		(&UserToCreate{}).EmailVerified(false),
		map[string]interface{}{"emailVerified": false},
	},
	{
		(&UserToCreate{}).PhotoURL("http://some.url"),
		map[string]interface{}{"photoUrl": "http://some.url"},
	},
}

func TestCreateUser(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	wantPath := "/projects/mock-project-id/accounts"
	for _, tc := range createUserCases {
		uid, err := s.Client.createUser(context.Background(), tc.params)
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

func TestInvalidUpdateUser(t *testing.T) {
	cases := []struct {
		params *UserToUpdate
		want   string
	}{
		{
			nil,
			"update parameters must not be nil or empty",
		}, {
			&UserToUpdate{},
			"update parameters must not be nil or empty",
		}, {
			(&UserToUpdate{}).Email(""),
			"email must be a non-empty string",
		}, {
			(&UserToUpdate{}).Email("invalid"),
			`malformed email string: "invalid"`,
		}, {
			(&UserToUpdate{}).PhoneNumber("1"),
			"phone number must be a valid, E.164 compliant identifier",
		}, {
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": strings.Repeat("a", 993)}),
			"serialized custom claims must not exceed 1000 characters",
		}, {
			(&UserToUpdate{}).Password("short"),
			"password must be a string at least 6 characters long",
		},
	}

	for _, claim := range reservedClaims {
		s := struct {
			params *UserToUpdate
			want   string
		}{
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{claim: true}),
			fmt.Sprintf("claim %q is reserved and must not be set", claim),
		}
		cases = append(cases, s)
	}

	client := &Client{}
	for i, tc := range cases {
		user, err := client.UpdateUser(context.Background(), "uid", tc.params)
		if user != nil || err == nil {
			t.Errorf("[%d] UpdateUser() = (%v, %v); want = (nil, error)", i, user, err)
		}
		if err.Error() != tc.want {
			t.Errorf("[%d] UpdateUser() = %v; want = %v", i, err.Error(), tc.want)
		}
	}
}

func TestUpdateUserEmptyUID(t *testing.T) {
	params := (&UserToUpdate{}).DisplayName("test")
	client := &Client{}
	user, err := client.UpdateUser(context.Background(), "", params)
	if user != nil || err == nil {
		t.Errorf("UpdateUser('') = (%v, %v); want = (nil, error)", user, err)
	}
}

var updateUserCases = []struct {
	params *UserToUpdate
	req    map[string]interface{}
}{
	{
		(&UserToUpdate{}).Password("123456"),
		map[string]interface{}{"password": "123456"},
	},
	{
		(&UserToUpdate{}).PhoneNumber("+1"),
		map[string]interface{}{"phoneNumber": "+1"},
	},
	{
		(&UserToUpdate{}).DisplayName("a"),
		map[string]interface{}{"displayName": "a"},
	},
	{
		(&UserToUpdate{}).Email("a@a"),
		map[string]interface{}{"email": "a@a"},
	},
	{
		(&UserToUpdate{}).Disabled(true),
		map[string]interface{}{"disableUser": true},
	},
	{
		(&UserToUpdate{}).Disabled(false),
		map[string]interface{}{"disableUser": false},
	},
	{
		(&UserToUpdate{}).EmailVerified(true),
		map[string]interface{}{"emailVerified": true},
	},
	{
		(&UserToUpdate{}).EmailVerified(false),
		map[string]interface{}{"emailVerified": false},
	},
	{
		(&UserToUpdate{}).PhotoURL("http://some.url"),
		map[string]interface{}{"photoUrl": "http://some.url"},
	},
	{
		(&UserToUpdate{}).DisplayName(""),
		map[string]interface{}{"deleteAttribute": []string{"DISPLAY_NAME"}},
	},
	{
		(&UserToUpdate{}).PhoneNumber(""),
		map[string]interface{}{"deleteProvider": []string{"phone"}},
	},
	{
		(&UserToUpdate{}).PhotoURL(""),
		map[string]interface{}{"deleteAttribute": []string{"PHOTO_URL"}},
	},
	{
		(&UserToUpdate{}).PhotoURL("").PhoneNumber("").DisplayName(""),
		map[string]interface{}{
			"deleteAttribute": []string{"DISPLAY_NAME", "PHOTO_URL"},
			"deleteProvider":  []string{"phone"},
		},
	},
	{
		(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": strings.Repeat("a", 992)}),
		map[string]interface{}{"customAttributes": fmt.Sprintf(`{"a":%q}`, strings.Repeat("a", 992))},
	},
	{
		(&UserToUpdate{}).CustomClaims(map[string]interface{}{}),
		map[string]interface{}{"customAttributes": "{}"},
	},
	{
		(&UserToUpdate{}).CustomClaims(nil),
		map[string]interface{}{"customAttributes": "{}"},
	},
}

func TestUpdateUser(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SetAccountInfoResponse",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	wantPath := "/projects/mock-project-id/accounts:update"
	for _, tc := range updateUserCases {
		err := s.Client.updateUser(context.Background(), "uid", tc.params)
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

func TestRevokeRefreshTokens(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SetAccountInfoResponse",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()
	before := time.Now().Unix()
	if err := s.Client.RevokeRefreshTokens(context.Background(), "some_uid"); err != nil {
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

	wantPath := "/projects/mock-project-id/accounts:update"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("RevokeRefreshTokens() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestRevokeRefreshTokensInvalidUID(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SetAccountInfoResponse",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	we := "uid must be a non-empty string"
	if err := s.Client.RevokeRefreshTokens(context.Background(), ""); err == nil || err.Error() != we {
		t.Errorf("RevokeRefreshTokens(); err = %s; want err = %s", err.Error(), we)
	}
}

func TestInvalidSetCustomClaims(t *testing.T) {
	cases := []struct {
		cc   map[string]interface{}
		want string
	}{
		{
			map[string]interface{}{"a": strings.Repeat("a", 993)},
			"serialized custom claims must not exceed 1000 characters",
		},
		{
			map[string]interface{}{"a": func() {}},
			"custom claims marshaling error: json: unsupported type: func()",
		},
	}

	for _, res := range reservedClaims {
		s := struct {
			cc   map[string]interface{}
			want string
		}{
			map[string]interface{}{res: true},
			fmt.Sprintf("claim %q is reserved and must not be set", res),
		}
		cases = append(cases, s)
	}

	client := &Client{}
	for _, tc := range cases {
		err := client.SetCustomUserClaims(context.Background(), "uid", tc.cc)
		if err == nil {
			t.Errorf("SetCustomUserClaims() = nil; want error: %s", tc.want)
		}
		if err.Error() != tc.want {
			t.Errorf("SetCustomUserClaims() = %q; want = %q", err.Error(), tc.want)
		}
	}
}

var setCustomUserClaimsCases = []map[string]interface{}{
	nil,
	{},
	{"admin": true},
	{"admin": true, "package": "gold"},
}

func TestSetCustomUserClaims(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SetAccountInfoResponse",
		"localId": "uid"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	wantPath := "/projects/mock-project-id/accounts:update"
	for _, tc := range setCustomUserClaimsCases {
		err := s.Client.SetCustomUserClaims(context.Background(), "uid", tc)
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

func TestUserToImport(t *testing.T) {
	cases := []struct {
		user *UserToImport
		want map[string]interface{}
	}{
		{
			user: (&UserToImport{}).UID("test"),
			want: map[string]interface{}{
				"localId": "test",
			},
		},
		{
			user: (&UserToImport{}).UID("test").DisplayName("name"),
			want: map[string]interface{}{
				"localId":     "test",
				"displayName": "name",
			},
		},
		{
			user: (&UserToImport{}).UID("test").Email("test@example.com"),
			want: map[string]interface{}{
				"localId": "test",
				"email":   "test@example.com",
			},
		},
		{
			user: (&UserToImport{}).UID("test").PhotoURL("https://test.com/user.png"),
			want: map[string]interface{}{
				"localId":  "test",
				"photoUrl": "https://test.com/user.png",
			},
		},
		{
			user: (&UserToImport{}).UID("test").PhoneNumber("+1234567890"),
			want: map[string]interface{}{
				"localId":     "test",
				"phoneNumber": "+1234567890",
			},
		},
		{
			user: (&UserToImport{}).UID("test").Metadata(&UserMetadata{
				CreationTimestamp:  int64(100),
				LastLogInTimestamp: int64(150),
			}),
			want: map[string]interface{}{
				"localId":     "test",
				"createdAt":   int64(100),
				"lastLoginAt": int64(150),
			},
		},
		{
			user: (&UserToImport{}).UID("test").PasswordHash([]byte("password")),
			want: map[string]interface{}{
				"localId":      "test",
				"passwordHash": base64.RawURLEncoding.EncodeToString([]byte("password")),
			},
		},
		{
			user: (&UserToImport{}).UID("test").PasswordSalt([]byte("nacl")),
			want: map[string]interface{}{
				"localId": "test",
				"salt":    base64.RawURLEncoding.EncodeToString([]byte("nacl")),
			},
		},
		{
			user: (&UserToImport{}).UID("test").CustomClaims(map[string]interface{}{"admin": true}),
			want: map[string]interface{}{
				"localId":          "test",
				"customAttributes": `{"admin":true}`,
			},
		},
		{
			user: (&UserToImport{}).UID("test").CustomClaims(map[string]interface{}{}),
			want: map[string]interface{}{
				"localId": "test",
			},
		},
		{
			user: (&UserToImport{}).UID("test").ProviderData([]*UserProvider{
				{
					ProviderID: "google.com",
					UID:        "test",
				},
			}),
			want: map[string]interface{}{
				"localId": "test",
				"providerUserInfo": []*UserProvider{
					{
						ProviderID: "google.com",
						UID:        "test",
					},
				},
			},
		},
		{
			user: (&UserToImport{}).UID("test").EmailVerified(true),
			want: map[string]interface{}{
				"localId":       "test",
				"emailVerified": true,
			},
		},
		{
			user: (&UserToImport{}).UID("test").EmailVerified(false),
			want: map[string]interface{}{
				"localId":       "test",
				"emailVerified": false,
			},
		},
		{
			user: (&UserToImport{}).UID("test").Disabled(true),
			want: map[string]interface{}{
				"localId":  "test",
				"disabled": true,
			},
		},
		{
			user: (&UserToImport{}).UID("test").Disabled(false),
			want: map[string]interface{}{
				"localId":  "test",
				"disabled": false,
			},
		},
	}

	for idx, tc := range cases {
		got, err := tc.user.validatedUserInfo()
		if err != nil {
			t.Errorf("[%d] invalid user: %v", idx, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("[%d] UserToImport = %#v; want = %#v", idx, got, tc.want)
		}
	}
}

func TestUserToImportError(t *testing.T) {
	cases := []struct {
		user *UserToImport
		want string
	}{
		{
			&UserToImport{},
			"no parameters are set on the user to import",
		},
		{
			(&UserToImport{}).UID(""),
			"uid must be a non-empty string",
		},
		{
			(&UserToImport{}).UID(strings.Repeat("a", 129)),
			"uid string must not be longer than 128 characters",
		},
		{
			(&UserToImport{}).UID("test").Email("not-an-email"),
			`malformed email string: "not-an-email"`,
		},
		{
			(&UserToImport{}).UID("test").PhoneNumber("not-a-phone"),
			"phone number must be a valid, E.164 compliant identifier",
		},
		{
			(&UserToImport{}).UID("test").CustomClaims(map[string]interface{}{"key": strings.Repeat("a", 1000)}),
			"serialized custom claims must not exceed 1000 characters",
		},
		{
			(&UserToImport{}).UID("test").ProviderData([]*UserProvider{
				{
					UID: "test",
				},
			}),
			"user provider must specify a provider ID",
		},
		{
			(&UserToImport{}).UID("test").ProviderData([]*UserProvider{
				{
					ProviderID: "google.com",
				},
			}),
			"user provdier must specify a uid",
		},
	}

	s := echoServer([]byte("{}"), t)
	defer s.Close()
	for idx, tc := range cases {
		_, err := s.Client.ImportUsers(context.Background(), []*UserToImport{tc.user})
		if err == nil || err.Error() != tc.want {
			t.Errorf("[%d] UserToImport = %v; want = %q", idx, err, tc.want)
		}
	}
}

func TestInvalidImportUsers(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	result, err := s.Client.ImportUsers(context.Background(), nil)
	if result != nil || err == nil {
		t.Errorf("ImportUsers(nil) = (%v, %v); want = (nil, error)", result, err)
	}

	result, err = s.Client.ImportUsers(context.Background(), []*UserToImport{})
	if result != nil || err == nil {
		t.Errorf("ImportUsers([]) = (%v, %v); want = (nil, error)", result, err)
	}

	var users []*UserToImport
	for i := 0; i < 1001; i++ {
		users = append(users, (&UserToImport{}).UID(fmt.Sprintf("user%d", i)))
	}
	result, err = s.Client.ImportUsers(context.Background(), users)
	if result != nil || err == nil {
		t.Errorf("ImportUsers(len > 1000) = (%v, %v); want = (nil, error)", result, err)
	}
}

func TestImportUsers(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	users := []*UserToImport{
		(&UserToImport{}).UID("user1"),
		(&UserToImport{}).UID("user2"),
	}
	result, err := s.Client.ImportUsers(context.Background(), users)
	if err != nil {
		t.Fatal(err)
	}

	if result.SuccessCount != 2 || result.FailureCount != 0 {
		t.Errorf("ImportUsers() = %#v; want = {SuccessCount: 2, FailureCount: 0}", result)
	}

	wantPath := "/projects/mock-project-id/accounts:batchCreate"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("ImportUsers() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestImportUsersError(t *testing.T) {
	resp := `{
		"error": [
      {"index": 0, "message": "Some error occurred in user1"},
      {"index": 2, "message": "Another error occurred in user3"}
    ]
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()
	users := []*UserToImport{
		(&UserToImport{}).UID("user1"),
		(&UserToImport{}).UID("user2"),
		(&UserToImport{}).UID("user3"),
	}
	result, err := s.Client.ImportUsers(context.Background(), users)
	if err != nil {
		t.Fatal(err)
	}
	if result.SuccessCount != 1 || result.FailureCount != 2 || len(result.Errors) != 2 {
		t.Fatalf("ImportUsers() = %#v; want = {SuccessCount: 1, FailureCount: 2}", result)
	}
	want := []ErrorInfo{
		{Index: 0, Reason: "Some error occurred in user1"},
		{Index: 2, Reason: "Another error occurred in user3"},
	}
	for idx, we := range want {
		if *result.Errors[idx] != we {
			t.Errorf("[%d] Error = %#v; want = %#v", idx, result.Errors[idx], we)
		}
	}
}

type mockHash struct {
	key, saltSep       string
	rounds, memoryCost int64
}

func (h mockHash) Config() (internal.HashConfig, error) {
	return internal.HashConfig{
		"hashAlgorithm": "MOCKHASH",
		"signerKey":     h.key,
		"saltSeparator": h.saltSep,
		"rounds":        h.rounds,
		"memoryCost":    h.memoryCost,
	}, nil
}

func TestImportUsersWithHash(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()

	users := []*UserToImport{
		(&UserToImport{}).UID("user1").PasswordHash([]byte("password")),
		(&UserToImport{}).UID("user2"),
	}
	result, err := s.Client.ImportUsers(context.Background(), users, WithHash(mockHash{
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

	wantPath := "/projects/mock-project-id/accounts:batchCreate"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("ImportUsers() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestImportUsersMissingRequiredHash(t *testing.T) {
	s := echoServer([]byte("{}"), t)
	defer s.Close()
	users := []*UserToImport{
		(&UserToImport{}).UID("user1").PasswordHash([]byte("password")),
		(&UserToImport{}).UID("user2"),
	}
	result, err := s.Client.ImportUsers(context.Background(), users)
	if result != nil || err == nil {
		t.Fatalf("ImportUsers() = (%v, %v); want = (nil, error)", result, err)
	}
}

func TestDeleteUser(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"email": "",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	if err := s.Client.DeleteUser(context.Background(), "uid"); err != nil {
		t.Errorf("DeleteUser() = %v; want = nil", err)
	}

	wantPath := "/projects/mock-project-id/accounts:delete"
	if s.Req[0].RequestURI != wantPath {
		t.Errorf("DeleteUser() URL = %q; want = %q", s.Req[0].RequestURI, wantPath)
	}
}

func TestInvalidDeleteUser(t *testing.T) {
	client := &Client{}
	if err := client.DeleteUser(context.Background(), ""); err == nil {
		t.Errorf("DeleteUser('') = nil; want error")
	}
}

func TestMakeExportedUser(t *testing.T) {
	queryResponse := &userQueryResponse{
		UID:                "testuser",
		Email:              "testuser@example.com",
		PhoneNumber:        "+1234567890",
		EmailVerified:      true,
		DisplayName:        "Test User",
		PasswordSalt:       "salt",
		PhotoURL:           "http://www.example.com/testuser/photo.png",
		PasswordHash:       "passwordhash",
		ValidSinceSeconds:  1494364393,
		Disabled:           false,
		CreationTimestamp:  1234567890000,
		LastLogInTimestamp: 1233211232000,
		CustomAttributes:   `{"admin": true, "package": "gold"}`,
		ProviderUserInfo: []*UserInfo{
			{
				ProviderID:  "password",
				DisplayName: "Test User",
				PhotoURL:    "http://www.example.com/testuser/photo.png",
				Email:       "testuser@example.com",
				UID:         "testuid",
			}, {
				ProviderID:  "phone",
				PhoneNumber: "+1234567890",
				UID:         "testuid",
			}},
	}

	want := &ExportedUserRecord{
		UserRecord:   testUser,
		PasswordHash: "passwordhash",
		PasswordSalt: "salt",
	}
	exported, err := queryResponse.makeExportedUserRecord()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exported.UserRecord, want.UserRecord) {
		// zero in
		t.Errorf("makeExportedUser() = %#v; want: %#v \n(%#v)\n(%#v)", exported.UserRecord, want.UserRecord,
			exported.UserMetadata, want.UserMetadata)
	}
	if exported.PasswordHash != want.PasswordHash {
		t.Errorf("PasswordHash = %q; want = %q", exported.PasswordHash, want.PasswordHash)
	}
	if exported.PasswordSalt != want.PasswordSalt {
		t.Errorf("PasswordSalt = %q; want = %q", exported.PasswordSalt, want.PasswordSalt)
	}
}

func TestExportedUserRecordShouldClearRedacted(t *testing.T) {
	queryResponse := &userQueryResponse{
		UID:          "uid1",
		PasswordHash: base64.StdEncoding.EncodeToString([]byte("REDACTED")),
	}

	exported, err := queryResponse.makeExportedUserRecord()
	if err != nil {
		t.Fatal(err)
	}
	if exported.PasswordHash != "" {
		t.Errorf("PasswordHash = %q; want = ''", exported.PasswordHash)
	}
}

var createSessionCookieCases = []struct {
	expiresIn time.Duration
	want      float64
}{
	{
		expiresIn: 10 * time.Minute,
		want:      600.0,
	},
	{
		expiresIn: 300500 * time.Millisecond,
		want:      300.0,
	},
}

func TestSessionCookie(t *testing.T) {
	resp := `{
		"sessionCookie": "expectedCookie"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	for _, tc := range createSessionCookieCases {
		cookie, err := s.Client.SessionCookie(context.Background(), "idToken", tc.expiresIn)
		if cookie != "expectedCookie" || err != nil {
			t.Errorf("SessionCookie() = (%q, %v); want = (%q, nil)", cookie, err, "expectedCookie")
		}

		wantURL := "/projects/mock-project-id:createSessionCookie"
		if s.Req[0].URL.Path != wantURL {
			t.Errorf("SesionCookie() URL = %q; want = %q", s.Req[0].URL.Path, wantURL)
		}

		var got map[string]interface{}
		if err := json.Unmarshal(s.Rbody, &got); err != nil {
			t.Fatal(err)
		}
		want := map[string]interface{}{
			"idToken":       "idToken",
			"validDuration": tc.want,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("SessionCookie(%f) request =%#v; want = %#v", tc.want, got, want)
		}
	}
}

func TestSessionCookieError(t *testing.T) {
	resp := `{
		"error": {
			"message": "PERMISSION_DENIED"
		}
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()
	s.Status = http.StatusForbidden

	cookie, err := s.Client.SessionCookie(context.Background(), "idToken", 10*time.Minute)
	if cookie != "" || err == nil {
		t.Fatalf("SessionCookie() = (%q, %v); want = (%q, error)", cookie, err, "")
	}

	want := fmt.Sprintf("http error status: 403; body: %s", resp)
	if err.Error() != want || !IsInsufficientPermission(err) {
		t.Errorf("SessionCookie() error = %v; want = %q", err, want)
	}
}

func TestSessionCookieWithoutProjectID(t *testing.T) {
	client := &Client{
		userManagementClient: &userManagementClient{},
	}
	_, err := client.SessionCookie(context.Background(), "idToken", 10*time.Minute)
	want := "project id not available"
	if err == nil || err.Error() != want {
		t.Errorf("SessionCookie() = %v; want = %q", err, want)
	}
}

func TestSessionCookieWithoutIDToken(t *testing.T) {
	client := &Client{}
	_, err := client.SessionCookie(context.Background(), "", 10*time.Minute)
	if err == nil {
		t.Errorf("CreateSessionCookie('') = nil; want error")
	}
}

func TestSessionCookieShortExpiresIn(t *testing.T) {
	client := &Client{}
	lessThanFiveMins := 5*time.Minute - time.Second
	_, err := client.SessionCookie(context.Background(), "idToken", lessThanFiveMins)
	if err == nil {
		t.Errorf("SessionCookie(< 5 mins) = nil; want error")
	}
}

func TestSessionCookieLongExpiresIn(t *testing.T) {
	client := &Client{}
	moreThanTwoWeeks := 14*24*time.Hour + time.Second
	_, err := client.SessionCookie(context.Background(), "idToken", moreThanTwoWeeks)
	if err == nil {
		t.Errorf("SessionCookie(> 14 days) = nil; want error")
	}
}

func TestHTTPError(t *testing.T) {
	s := echoServer([]byte(`{"error":"test"}`), t)
	defer s.Close()
	s.Client.userManagementClient.httpClient.RetryConfig = nil
	s.Status = http.StatusInternalServerError

	u, err := s.Client.GetUser(context.Background(), "some uid")
	if u != nil || err == nil {
		t.Fatalf("GetUser() = (%v, %v); want = (nil, error)", u, err)
	}

	want := `http error status: 500; body: {"error":"test"}`
	if err.Error() != want || !IsUnknown(err) {
		t.Errorf("GetUser() = %v; want = %q", err, want)
	}
}

func TestHTTPErrorWithCode(t *testing.T) {
	errorCodes := map[string]func(error) bool{
		"CONFIGURATION_NOT_FOUND": IsConfigurationNotFound,
		"DUPLICATE_EMAIL":         IsEmailAlreadyExists,
		"DUPLICATE_LOCAL_ID":      IsUIDAlreadyExists,
		"EMAIL_EXISTS":            IsEmailAlreadyExists,
		"INSUFFICIENT_PERMISSION": IsInsufficientPermission,
		"PHONE_NUMBER_EXISTS":     IsPhoneNumberAlreadyExists,
		"PROJECT_NOT_FOUND":       IsProjectNotFound,
	}
	s := echoServer(nil, t)
	defer s.Close()
	s.Client.userManagementClient.httpClient.RetryConfig = nil
	s.Status = http.StatusInternalServerError

	for code, check := range errorCodes {
		s.Resp = []byte(fmt.Sprintf(`{"error":{"message":"%s"}}`, code))
		u, err := s.Client.GetUser(context.Background(), "some uid")
		if u != nil || err == nil {
			t.Fatalf("GetUser() = (%v, %v); want = (nil, error)", u, err)
		}

		want := fmt.Sprintf(`http error status: 500; body: {"error":{"message":"%s"}}`, code)
		if err.Error() != want || !check(err) {
			t.Errorf("GetUser() = %v; want = %q", err, want)
		}
	}
}

type mockAuthServer struct {
	Resp   []byte
	Header map[string]string
	Status int
	Req    []*http.Request
	Rbody  []byte
	Srv    *httptest.Server
	Client *Client
}

// echoServer takes either a []byte or a string filename, or an object.
//
// echoServer returns a server whose client will reply with depending on the input type:
//   * []byte: the []byte it got
//   * object: the marshalled object, in []byte form
//   * nil: "{}" empty json, in case we aren't interested in the returned value, just the marshalled request
// The marshalled request is available through s.rbody, s being the retuned server.
// It also returns a closing functions that has to be defer closed.
func echoServer(resp interface{}, t *testing.T) *mockAuthServer {
	var b []byte
	var err error
	switch v := resp.(type) {
	case nil:
		b = []byte("")
	case []byte:
		b = v
	default:
		if b, err = json.Marshal(resp); err != nil {
			t.Fatal("marshaling error")
		}
	}
	s := mockAuthServer{Resp: b}

	const testToken = "test.token"
	const testVersion = "test.version"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		s.Rbody = bytes.TrimSpace(reqBody)
		s.Req = append(s.Req, r)

		gh := r.Header.Get("Authorization")
		wh := "Bearer " + testToken
		if gh != wh {
			t.Errorf("Authorization header = %q; want = %q", gh, wh)
		}

		gh = r.Header.Get("X-Client-Version")
		wh = "Go/Admin/" + testVersion
		if gh != wh {
			t.Errorf("X-Client-Version header = %q; want: %q", gh, wh)
		}

		for k, v := range s.Header {
			w.Header().Set(k, v)
		}
		if s.Status != 0 {
			w.WriteHeader(s.Status)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(s.Resp)
	})
	s.Srv = httptest.NewServer(handler)
	conf := &internal.AuthConfig{
		Opts:      optsWithTokenSource,
		ProjectID: "mock-project-id",
		Version:   testVersion,
	}

	authClient, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatal(err)
	}

	authClient.userManagementClient.baseURL = s.Srv.URL
	authClient.providerConfigClient.endpoint = s.Srv.URL
	s.Client = authClient
	return &s
}

func (s *mockAuthServer) Close() {
	s.Srv.Close()
}
