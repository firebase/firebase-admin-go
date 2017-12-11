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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"firebase.google.com/go/internal"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
)

var testUser = &UserRecord{
	UserInfo: &UserInfo{
		UID:         "testuser",
		Email:       "testuser@example.com",
		PhoneNumber: "+1234567890",
		DisplayName: "Test User",
		PhotoURL:    "http://www.example.com/testuser/photo.png",
	},
	Disabled: false,

	EmailVerified: true,
	ProviderUserInfo: []*UserInfo{
		{
			ProviderID:  "password",
			DisplayName: "Test User",
			PhotoURL:    "http://www.example.com/testuser/photo.png",
			Email:       "testuser@example.com",
		}, {
			ProviderID:  "phone",
			PhoneNumber: "+1234567890",
		},
	},
	UserMetadata: &UserMetadata{
		CreationTimestamp:  1234567890,
		LastLogInTimestamp: 1233211232,
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
}

func TestInvalidGetUser(t *testing.T) {
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

	user, err := s.Client.GetUser(context.Background(), "ignored_id")
	if user != nil || err == nil {
		t.Errorf("GetUser(non-existing) = (%v, %v); want = (nil, error)", user, err)
	}
	user, err = s.Client.GetUserByEmail(context.Background(), "test@email.com")
	if user != nil || err == nil {
		t.Errorf("GetUserByEmail(non-existing) = (%v, %v); want = (nil, error)", user, err)
	}
	user, err = s.Client.GetUserByPhoneNumber(context.Background(), "+1234567890")
	if user != nil || err == nil {
		t.Errorf("GetUserPhoneNumber(non-existing) = (%v, %v); want = (nil, error)", user, err)
	}
}

func TestListUsers(t *testing.T) {
	s := echoServer(testListUsersResponse, t)
	defer s.Close()

	want := []*ExportedUserRecord{
		&ExportedUserRecord{UserRecord: testUser, PasswordHash: "passwordhash1", PasswordSalt: "salt1"},
		&ExportedUserRecord{UserRecord: testUser, PasswordHash: "passwordhash2", PasswordSalt: "salt2"},
		&ExportedUserRecord{UserRecord: testUser, PasswordHash: "passwordhash3", PasswordSalt: "salt3"},
	}

	testIterator := func(iter *UserIterator, token string, req map[string]interface{}) {
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

		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		if string(s.Rbody) != string(b) {
			t.Errorf("Users(%q) = %v, want = %v", token, string(s.Rbody), string(b))
		}
	}
	testIterator(
		s.Client.Users(context.Background(), ""),
		"", map[string]interface{}{"maxResults": 1000})
	testIterator(
		s.Client.Users(context.Background(), "pageToken"),
		"pageToken", map[string]interface{}{"maxResults": 1000, "nextPageToken": "pageToken"})
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
			"phone number must not be empty",
		}, {
			(&UserToCreate{}).PhoneNumber("1234"),
			"phone number must be a valid, E.164 compliant identifier",
		}, {
			(&UserToCreate{}).PhoneNumber("+_!@#$"),
			"phone number must be a valid, E.164 compliant identifier",
		}, {
			(&UserToCreate{}).UID(""),
			"uid must not be empty",
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
			"email must not be empty",
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

func TestCreateUser(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	cases := []struct {
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
			(&UserToCreate{}).EmailVerified(true),
			map[string]interface{}{"emailVerified": true},
		},
		{
			(&UserToCreate{}).PhotoURL("http://some.url"),
			map[string]interface{}{"photoUrl": "http://some.url"},
		},
	}
	for _, tc := range cases {
		uid, err := s.Client.createUser(context.Background(), tc.params)
		if uid != "expectedUserID" || err != nil {
			t.Errorf("createUser(%v) = (%q, %v); want = (%q, nil)", tc.params, uid, err, "expectedUserID")
		}
		want, err := json.Marshal(tc.req)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(s.Rbody, want) {
			t.Errorf("createUser() request = %v; want = %v", string(s.Rbody), string(want))
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
			(&UserToUpdate{}).PhoneNumber("1"),
			"phone number must be a valid, E.164 compliant identifier",
		}, {
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": strings.Repeat("a", 993)}),
			"serialized custom claims must not exceed 1000 characters",
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
	user, err := client.UpdateUser(context.Background(), "", params)
	if user != nil || err == nil {
		t.Errorf("UpdateUser('') = (%v, %v); want = (nil, error)", user, err)
	}
}

func TestUpdateUser(t *testing.T) {
	resp := `{
		"kind": "identitytoolkit#SetAccountInfoResponse",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	cases := []struct {
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
			(&UserToUpdate{}).EmailVerified(true),
			map[string]interface{}{"emailVerified": true},
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
	for _, tc := range cases {
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

func TestSetCustomClaims(t *testing.T) {
	cases := []map[string]interface{}{
		nil,
		map[string]interface{}{},
		map[string]interface{}{"admin": true},
		map[string]interface{}{"admin": true, "package": "gold"},
	}

	resp := `{
		"kind": "identitytoolkit#SetAccountInfoResponse",
		"localId": "uid"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()
	for _, tc := range cases {
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
}

func TestInvalidDeleteUser(t *testing.T) {
	if err := client.DeleteUser(context.Background(), ""); err == nil {
		t.Errorf("DeleteUser('') = nil; want error")
	}
}

func TestMakeExportedUser(t *testing.T) {
	rur := responseUserRecord{
		UID:           "testuser",
		Email:         "testuser@example.com",
		PhoneNumber:   "+1234567890",
		EmailVerified: true,
		DisplayName:   "Test User",
		ProviderUserInfo: []*UserInfo{
			{
				ProviderID:  "password",
				DisplayName: "Test User",
				PhotoURL:    "http://www.example.com/testuser/photo.png",
				Email:       "testuser@example.com",
			}, {
				ProviderID:  "phone",
				PhoneNumber: "+1234567890",
			}},
		PhotoURL:     "http://www.example.com/testuser/photo.png",
		PasswordHash: "passwordhash",
		PasswordSalt: "salt",

		ValidSince:         1494364393,
		Disabled:           false,
		CreationTimestamp:  1234567890,
		LastLogInTimestamp: 1233211232,
		CustomClaims:       `{"admin": true, "package": "gold"}`,
	}
	want := &ExportedUserRecord{
		UserRecord:   testUser,
		PasswordHash: "passwordhash",
		PasswordSalt: "salt",
	}
	exported, err := makeExportedUser(rur)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exported.UserRecord, want.UserRecord) {
		// zero in
		t.Errorf("makeExportedUser() = %#v; want: %#v", exported.UserRecord, want.UserRecord)
	}
	if exported.PasswordHash != want.PasswordHash {
		t.Errorf("PasswordHash = %q; want = %q", exported.PasswordHash, want.PasswordHash)
	}
	if exported.PasswordSalt != want.PasswordSalt {
		t.Errorf("PasswordSalt = %q; want = %q", exported.PasswordSalt, want.PasswordSalt)
	}
}

func TestHTTPError(t *testing.T) {
	s := mockAuthServer{}
	s.Srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Req = append(s.Req, r)
		w.WriteHeader(500)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":"test"}`))
	}))
	defer s.Close()

	client, err := NewClient(context.Background(), &internal.AuthConfig{})
	if err != nil {
		t.Fatal()
	}
	client.url = s.Srv.URL + "/"

	want := `http error status: 500; reason: {"error":"test"}`
	_, err = client.GetUser(context.Background(), "some uid")
	if err == nil || err.Error() != want {
		t.Errorf("GetUser() = %v; want = %q", err, want)
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
	testVersion := "test.version"
	switch v := resp.(type) {
	case nil:
		b = []byte("")
	case []byte:
		b = v
	default:
		b, err = json.Marshal(resp)
		if err != nil {
			t.Fatal("marshaling error")
		}
	}
	s := mockAuthServer{Resp: b}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		s.Req = append(s.Req, r)
		vh := r.Header.Get("X-Client-Version")
		wantvh := "Go/Admin/" + testVersion
		if vh != wantvh {
			t.Errorf("version header = %s; want: %s", vh, wantvh)
		}
		s.Rbody = reqBody
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
	authClient, err := NewClient(context.Background(), &internal.AuthConfig{Version: testVersion})
	if err != nil {
		t.Fatal(err)
	}
	authClient.url = s.Srv.URL + "/"
	s.Client = authClient
	return &s
}

func (s *mockAuthServer) Close() {
	s.Srv.Close()
}
