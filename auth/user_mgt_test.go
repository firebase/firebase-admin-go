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
	"go/build"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	s := echoServer("get_user.json", t)
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
	s := echoServer("get_user.json", t)
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
	s := echoServer("get_user.json", t)
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

func TestListUsers(t *testing.T) {
	s := echoServer("list_users.json", t)
	defer s.Close()

	want := []*ExportedUserRecord{
		&ExportedUserRecord{UserRecord: testUser, PasswordHash: "passwordhash", PasswordSalt: "salt==="},
		&ExportedUserRecord{UserRecord: testUser, PasswordHash: "passwordhash", PasswordSalt: "salt==="},
		&ExportedUserRecord{UserRecord: testUser, PasswordHash: "passwordhash", PasswordSalt: "salt==="},
	}
	count := 0

	iter := s.Client.Users(context.Background(), "")
	for i := 0; i < len(want); i++ {
		user, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(user.UserRecord, want[i].UserRecord) {
			t.Errorf("Users() iterator [%d] = %#v; want = %#v", i, user, want[i])
		}
		if user.PasswordHash != want[i].PasswordHash {
			t.Errorf("Users() PasswordHash = %q; want = %q", user.PasswordHash, want[i].PasswordHash)
		}
		if user.PasswordSalt != want[i].PasswordSalt {
			t.Errorf("Users() PasswordSalt = %q; want = %q", user.PasswordSalt, want[i].PasswordSalt)
		}
		count++
	}
	if count != len(want) {
		t.Errorf("Users() = %d; want = %d", count, len(want))
	}
	if _, err := iter.Next(); err != iterator.Done {
		t.Errorf("Users() = %v, want %v", err, iterator.Done)
	}
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
			"uid must be a string at most 128 characters long",
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
		"email": "",
		"localId": "expectedUserID"
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	cases := []*UserToCreate{
		nil,
		{},
		(&UserToCreate{}).Password("123456"),
		(&UserToCreate{}).UID("1"),
		(&UserToCreate{}).UID(strings.Repeat("a", 128)),
		(&UserToCreate{}).PhoneNumber("+1"),
		(&UserToCreate{}).DisplayName("a"),
		(&UserToCreate{}).Email("a@a"),
		(&UserToCreate{}).PhoneNumber("+1"),
	}
	for _, tc := range cases {
		_, err := s.Client.CreateUser(context.Background(), tc)
		// There are two calls to the server, the first one, on creation retunrs the above []byte
		// that's how we know the params passed validation
		// the second call to GetUser, tries to get the user with the returned ID above, it fails
		// with the following expected error
		if err.Error() != "cannot find user from params: map[localId:[expectedUserID]]" {
			t.Error(err)
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
			"user must not be nil or empty for update",
		}, {
			&UserToUpdate{},
			"user must not be nil or empty for update",
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
		"localId": "expectedUserID",
		"email": "tefwfd1234eml5f@test.com",
		"displayName": "display_name",
		"passwordHash": "UkVEQUNURUQ=",
		"providerUserInfo": [
		 {
		  "providerId": "password",
		  "federatedId": "tefwfd1234eml5f@test.com",
		  "displayName": "display_name"
		 }
		],
		"emailVerified": false
	}`
	s := echoServer([]byte(resp), t)
	defer s.Close()

	cases := []*UserToUpdate{
		(&UserToUpdate{}).Password("123456"),
		(&UserToUpdate{}).PhoneNumber("+1"),
		(&UserToUpdate{}).DisplayName("a"),
		(&UserToUpdate{}).Email("a@a"),
		(&UserToUpdate{}).PhoneNumber("+1"),
		(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": strings.Repeat("a", 992)}),
	}

	for _, tc := range cases {
		_, err := s.Client.UpdateUser(context.Background(), "expectedUserID", tc)
		// There are two calls to the server, the first one, on creation retunrs the above []byte
		// that's how we know the params passed validation
		// the second call to GetUser, tries to get the user with the returned ID above, it fails
		// with the following expected error
		if err.Error() != "cannot find user from params: map[localId:[expectedUserID]]" {
			t.Error(err)
		}
	}
}

func TestInvalidSetCustomClaims(t *testing.T) {
	cases := []struct {
		cc   map[string]interface{}
		want string
	}{{
		map[string]interface{}{"a": strings.Repeat("a", 993)},
		"serialized custom claims must not exceed 1000 characters",
	}}

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
			t.Errorf("SetCustomUserClaims() = nil; want = error: %s", tc.want)
		}
		if err.Error() != tc.want {
			t.Errorf("SetCustomUserClaims() = %q; want = %q", err.Error(), tc.want)
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
	if err := s.Client.DeleteUser(context.Background(), ""); err != nil {
		t.Error(err)
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
		PasswordSalt: "salt===",

		ValidSince:         1494364393,
		Disabled:           false,
		CreationTimestamp:  1234567890,
		LastLogInTimestamp: 1233211232,
		CustomClaims:       `{"admin": true, "package": "gold"}`,
	}
	want := &ExportedUserRecord{
		UserRecord:   testUser,
		PasswordHash: "passwordhash",
		PasswordSalt: "salt===",
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

func TestCreateRequest(t *testing.T) {
	tests := []struct {
		utc  *UserToCreate
		want string
	}{
		{
			nil,
			`{}`,
		}, {
			(&UserToCreate{}),
			`{}`,
		}, {
			(&UserToCreate{}).Disabled(true),
			`{"disableUser":true}`,
		}, {
			(&UserToCreate{}).DisplayName("a"),
			`{"displayName":"a"}`,
		}, {
			(&UserToCreate{}).Email("a@a.a"),
			`{"email":"a@a.a"}`,
		}, {
			(&UserToCreate{}).EmailVerified(true),
			`{"emailVerified":true}`,
		}, {
			(&UserToCreate{}).Password("654321"),
			`{"password":"654321"}`,
		}, {
			(&UserToCreate{}).PhoneNumber("+1"),
			`{"phoneNumber":"+1"}`,
		}, {
			(&UserToCreate{}).UID("1"),
			`{"localId":"1"}`,
		}, {
			(&UserToCreate{}).PhotoURL("http://some.url"),
			`{"photoUrl":"http://some.url"}`,
		}, {
			(&UserToCreate{}).UID(strings.Repeat("a", 128)),
			`{"localId":"` + strings.Repeat("aaaa", 32) + `"}`,
		},
	}

	for _, test := range tests {
		s := echoServer(nil, t) // the returned json is of no importance, we just need the request body.
		defer s.Close()
		s.Client.CreateUser(context.Background(), test.utc)
		if string(s.Rbody) != test.want {
			t.Errorf("CreateUser() request body = %q; want: %q", s.Rbody, test.want)
		}
	}
}

func TestUpdateRequest(t *testing.T) {
	tests := []struct {
		utup *UserToUpdate
		want string
	}{
		{
			(&UserToUpdate{}).Disabled(true),
			`{"disableUser":true,"localId":"uid"}`,
		}, {
			(&UserToUpdate{}).DisplayName("a"),
			`{"displayName":"a","localId":"uid"}`,
		}, {
			(&UserToUpdate{}).DisplayName(""),
			`{"deleteAttribute":["DISPLAY_NAME"],"localId":"uid"}`,
		}, {
			(&UserToUpdate{}).Email("a@a.a"),
			`{"email":"a@a.a","localId":"uid"}`,
		}, {
			(&UserToUpdate{}).EmailVerified(true),
			`{"emailVerified":true,"localId":"uid"}`,
		}, {
			(&UserToUpdate{}).Password("654321"),
			`{"localId":"uid","password":"654321"}`,
		}, {
			(&UserToUpdate{}).PhoneNumber("+1"),
			`{"localId":"uid","phoneNumber":"+1"}`,
		}, {
			(&UserToUpdate{}).PhoneNumber(""),
			`{"deleteProvider":["phone"],"localId":"uid"}`,
		}, {
			(&UserToUpdate{}).PhotoURL("http://some.url"),
			`{"localId":"uid","photoUrl":"http://some.url"}`,
		}, {
			(&UserToUpdate{}).PhotoURL(""),
			`{"deleteAttribute":["PHOTO_URL"],"localId":"uid"}`,
		}, {
			(&UserToUpdate{}).PhotoURL("").PhoneNumber("").DisplayName(""),
			`{"deleteAttribute":["DISPLAY_NAME","PHOTO_URL"],"deleteProvider":["phone"],"localId":"uid"}`,
		}, {
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": "b", "b": true, "c": 1}),
			`{"customAttributes":"{\"a\":\"b\",\"b\":true,\"c\":1}","localId":"uid"}`,
		}, {
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{}),
			`{"customAttributes":"{}","localId":"uid"}`,
		}, {
			(&UserToUpdate{}).CustomClaims(map[string]interface{}(nil)),
			`{"customAttributes":"{}","localId":"uid"}`,
		},
	}

	for _, test := range tests {
		s := echoServer(nil, t) // the returned json is of no importance, we just need the request body.
		defer s.Close()

		s.Client.UpdateUser(context.Background(), "uid", test.utup)
		var got, want map[string]interface{}
		err := json.Unmarshal(s.Rbody, &got)
		if err != nil {
			t.Fatal(err)
		}
		err = json.Unmarshal([]byte(test.want), &want)
		if err != nil {
			t.Fatal(err)
		}
		// Test params regqrdless of order
		if !reflect.DeepEqual(got, want) {
			t.Errorf("UpdateUser() request body = %q; want: %q", s.Rbody, test.want)
		}
		// json should have sorted keys.
		if string(s.Rbody) != test.want {
			t.Errorf("UpdateUser() request body = %q; want: %q", s.Rbody, test.want)
		}
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
		t.Errorf("got error = %v; want: `%v`", err, want)
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
//   * string: the contents of the file named by the string in []byte form
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
	case string:
		fp := filepath.Join([]string{build.Default.GOPATH, "src", "firebase.google.com", "go", "testdata", v}...)
		b, err = ioutil.ReadFile(fp)
		if err != nil {
			t.Fatal(err)
		}
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
	authClient, err := NewClient(context.Background(), &internal.AuthConfig{})
	if err != nil {
		t.Fatal()
	}
	authClient.url = s.Srv.URL + "/"
	s.Client = authClient
	return &s
}

func (s *mockAuthServer) Close() {
	s.Srv.Close()
}
