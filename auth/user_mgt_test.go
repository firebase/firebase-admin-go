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
	"google.golang.org/api/option"
)

type mockAuthServer struct {
	Resp   []byte
	Header map[string]string
	Status int
	Req    []*http.Request
	rbody  []byte
	srv    *httptest.Server
	Client *Client
}

var listUsers []*ExportedUserRecord

func TestGetUser(t *testing.T) {
	s, closer := echoServer("get_user.json", t)
	defer closer()

	user, err := s.Client.GetUser(context.Background(), "ignored_id")
	if err != nil {
		t.Error(err)
	}
	want := &UserRecord{
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
	if !reflect.DeepEqual(user, want) {
		t.Errorf("got %#v, wanted %#v", user, want)
		testCompareUserRecords(user, want, t)
	}
}

func TestGetUsers(t *testing.T) {
	setListUsers()
	s, closer := echoServer("list_users.json", t)
	defer closer()
	iter := s.Client.Users(context.Background(), "")

	for i := 0; i < len(listUsers); i++ {
		user, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Error(err)
		}
		if i >= len(listUsers) {
			t.Errorf("got %d users, wanted %d", i+1, len(listUsers))
		}
		if !reflect.DeepEqual(user, listUsers[i]) {
			t.Errorf("got %#v, wanted %#v", user, listUsers[i])
			testCompareUserRecords(user.UserRecord, listUsers[i].UserRecord, t)
		}
	}
	_, err := iter.Next()
	if err != iterator.Done {
		t.Errorf("got more than %d users, wanted %d", len(listUsers), len(listUsers))
	}

}

func TestBadCreateUser(t *testing.T) {
	badUserParams := []struct {
		params         *UserToCreate
		expectingError string
	}{
		{
			(&UserToCreate{}).Password("short"),
			`invalid Password string. Password must be a string at least 6 characters long`,
		}, {
			(&UserToCreate{}).PhoneNumber("1234"),
			`invalid phone number: "1234". Phone number must be a valid, E.164 compliant identifier`,
		}, {
			(&UserToCreate{}).PhoneNumber("+_!@#$"),
			`invalid phone number: "+_!@#$". Phone number must be a valid, E.164 compliant identifier`,
		}, {
			(&UserToCreate{}).UID(""),
			`invalid uid: "". The uid must be a non-empty string with no more than 128 characters`,
		}, {
			(&UserToCreate{}).UID(strings.Repeat("a", 129)),
			`invalid uid: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa". The uid must be a non-empty string with no more than 128 characters`,
		}, {
			(&UserToCreate{}).DisplayName(""),
			`DisplayName must not be empty`,
		}, {
			(&UserToCreate{}).PhotoURL(""),
			`invalid photo URL: "". PhotoURL must be a non-empty string`,
		}, {
			(&UserToCreate{}).Email(""),
			`invalid Email: "" Email must be a non-empty string`,
		}, {
			(&UserToCreate{}).Email("a"),
			`malformed email address string: "a"`,
		}, {
			(&UserToCreate{}).Email("a@"),
			`malformed email address string: "a@"`,
		}, {
			(&UserToCreate{}).Email("@a"),
			`malformed email address string: "@a"`,
		}, {
			(&UserToCreate{}).Email("a@a@a"),
			`malformed email address string: "a@a@a"`,
		},
	}
	for i, test := range badUserParams {
		_, err := client.CreateUser(context.Background(), test.params)
		if err == nil {
			t.Errorf("%d) got no error, wanted error %s", i, test.expectingError)
		}
		if err.Error() != test.expectingError {
			t.Errorf(`got error: "%s" wanted error: "%s"`, err.Error(), test.expectingError)
		}
	}

}

func TestCreateUser(t *testing.T) {
	s, closer := echoServer([]byte(`{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"email": "",
		"localId": "expectedUserID"
	   }`), t)
	defer closer()
	goodParams := []*UserToCreate{
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
	for _, par := range goodParams {
		_, err := s.Client.CreateUser(context.Background(), par)
		// There are two calls to the server, the first one, on creation retunrs the above []byte
		// that's how we know the params passed validation
		// the second call to GetUser, tries to get the user with the returned ID above, it fails
		// with the following expected error
		if err.Error() != "cannot find user map[localId:[expectedUserID]]" {
			t.Error(err)
		}
	}
}

func TestBadUpdateParams(t *testing.T) {
	badParams := []struct {
		params         *UserToUpdate
		expectingError string
	}{
		{
			nil,
			"params must not be empty for update",
		}, {
			&UserToUpdate{},
			"params must not be empty for update",
		}, {
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": strings.Repeat("a", 993)}),
			fmt.Sprintf(`Custom Claims payload must not exceed %d characters`, maxLenPayloadCC),
		},
	}

	for _, res := range reservedClaims {
		badParams = append(badParams,
			struct {
				params         *UserToUpdate
				expectingError string
			}{
				(&UserToUpdate{}).CustomClaims(map[string]interface{}{res: true}),
				fmt.Sprintf(`claim "%s" is reserved, and must not be set`, res)})
	}

	for i, test := range badParams {
		_, err := client.UpdateUser(context.Background(), "outofstruct", test.params)
		if err == nil {
			t.Errorf("%d) got no error wanted error %s", i, test.expectingError)
		}
		if err.Error() != test.expectingError {
			t.Errorf(`%d) got error "%s" wanted error "%s"`, i, err.Error(), test.expectingError)
		}
	}
}

func TestUpdateUser(t *testing.T) {
	s, closer := echoServer([]byte(`{
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
	   }`), t)
	defer closer()

	goodParams := []*UserToUpdate{
		(&UserToUpdate{}).Password("123456"),
		(&UserToUpdate{}).PhoneNumber("+1"),
		(&UserToUpdate{}).DisplayName("a"),
		(&UserToUpdate{}).Email("a@a"),
		(&UserToUpdate{}).PhoneNumber("+1"),
		(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": strings.Repeat("a", 992)}),
	}

	for _, par := range goodParams {
		_, err := s.Client.UpdateUser(context.Background(), "expectedUserID", par)
		// There are two calls to the server, the first one, on creation retunrs the above []byte
		// that's how we know the params passed validation
		// the second call to GetUser, tries to get the user with the returned ID above, it fails
		// with the following expected error
		if err.Error() != "cannot find user map[localId:[expectedUserID]]" {
			t.Error(err)
		}
	}
}

func TestBadSetCustomClaims(t *testing.T) {
	badUserParams := []struct {
		cc   map[string]interface{}
		estr string
	}{{
		map[string]interface{}{"a": strings.Repeat("a", 993)},
		fmt.Sprintf("Custom Claims payload must not exceed %d characters", maxLenPayloadCC),
	}}

	for _, res := range reservedClaims {
		badUserParams = append(badUserParams,
			struct {
				cc   map[string]interface{}
				estr string
			}{
				cc:   map[string]interface{}{res: true},
				estr: fmt.Sprintf(`claim "%s" is reserved, and must not be set`, res),
			})
	}

	for i, test := range badUserParams {
		err := client.SetCustomUserClaims(context.Background(), "uid", test.cc)
		if err == nil {
			t.Errorf("%d) expecting error %s", i, test.estr)
		}
		if err.Error() != test.estr {
			t.Errorf(`got error: "%s" expecting error: "%s"`, err.Error(), test.estr)
		}
	}
}

func TestDelete(t *testing.T) {
	s, closer := echoServer([]byte(`{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"email": "",
		"localId": "expectedUserID"
	   }`), t)
	defer closer()
	if err := s.Client.DeleteUser(context.Background(), ""); err != nil {
		t.Error(err)
		return
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
		UserRecord: &UserRecord{
			UserInfo: &UserInfo{
				UID:         "testuser",
				Email:       "testuser@example.com",
				PhoneNumber: "+1234567890",
				PhotoURL:    "http://www.example.com/testuser/photo.png",
				DisplayName: "Test User",
			},
			CustomClaims:  map[string]interface{}{"admin": true, "package": "gold"},
			Disabled:      false,
			EmailVerified: true,
			UserMetadata: &UserMetadata{
				CreationTimestamp:  1234567890,
				LastLogInTimestamp: 1233211232,
			},
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
		},
		PasswordHash: "passwordhash",
		PasswordSalt: "salt===",
	}
	exported, err := makeExportedUser(rur)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(exported, want) {
		// zero in
		t.Errorf("got %#v, wanted %#v", exported, want)
		testCompareUserRecords(exported.UserRecord, want.UserRecord, t)
	}
}

func TestCreateRequest(t *testing.T) {
	tests := []struct {
		up        *UserToCreate
		expecting string
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

	for i, test := range tests {
		s, closer := echoServer(nil, t) // the returned json is of no importance, we just need the request body.
		defer closer()
		s.Client.CreateUser(context.Background(), test.up)

		if string(s.rbody) != test.expecting {
			t.Errorf("%d)request body = `%s` want: `%s`", i, s.rbody, test.expecting)

		}
	}

}

func TestUpdateRequest(t *testing.T) {
	tests := []struct {
		up        *UserToUpdate
		expecting string
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
			`{"deleteAttribute":["PHOTO_URL","DISPLAY_NAME"],"deleteProvider":["phone"],"localId":"uid"}`,
		}, {
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": "b", "b": true, "c": 1}),
			`{"customAttributes":"{\"a\":\"b\",\"b\":true,\"c\":1}","localId":"uid"}`,
		},
	}

	for _, test := range tests {
		s, closer := echoServer(nil, t) // the returned json is of no importance, we just need the request body.
		defer closer()

		s.Client.UpdateUser(context.Background(), "uid", test.up)
		var got, want map[string]interface{}
		err := json.Unmarshal(s.rbody, &got)
		if err != nil {
			t.Error(err)
		}
		err = json.Unmarshal([]byte(test.expecting), &want)
		if err != nil {
			t.Error(err)
		}
		// Test params regqrdless of order
		if !reflect.DeepEqual(got, want) {
			t.Errorf("request body = `%s` want: `%s`", s.rbody, test.expecting)
		}
		// json should have sorted keys.
		if string(s.rbody) != test.expecting {
			t.Errorf("request body = `%s` want: `%s`", s.rbody, test.expecting)

		}
	}

}

//---------------------------------------

// for pretty printing
func toString(e *ExportedUserRecord) string {
	return fmt.Sprintf("ExportedUserRecord: %#v\n"+
		"    UserRecord: %#v\n"+
		"        UserInfo: %#v\n"+
		"        MetaData: %#v\n"+
		"        CustomClaims: %#v\n"+
		"        ProviderData: %#v %s",
		e,
		e.UserRecord,
		e.UserInfo,
		e.UserMetadata,
		e.CustomClaims,
		e.ProviderUserInfo,
		provString(e))
}

// for pretty printing
func provString(e *ExportedUserRecord) string {
	providerStr := ""
	if e.ProviderUserInfo != nil {
		for _, info := range e.ProviderUserInfo {
			providerStr += fmt.Sprintf("\n            %#v", info)
		}
	}
	return providerStr
}

// used as a referece for the wanted results given by the list_users.json data file
func setListUsers() {
	listUsers = []*ExportedUserRecord{
		{
			UserRecord: &UserRecord{
				UserInfo: &UserInfo{
					UID: "VHHROt3NAjPoc1hanwMRcTdCESz2",
				},
				UserMetadata: &UserMetadata{
					LastLogInTimestamp: 0,
					CreationTimestamp:  1511284665000,
				},
				Disabled: false,
			},
		},
		{
			UserRecord: &UserRecord{
				UserInfo: &UserInfo{
					UID:         "tefwfd1234",
					DisplayName: "display_name",
					Email:       "tefwfd1234eml5f@test.com",
				},
				UserMetadata: &UserMetadata{
					LastLogInTimestamp: 0,
					CreationTimestamp:  1511284665000,
				},
				Disabled:      false,
				EmailVerified: false,
				ProviderUserInfo: []*UserInfo{
					{
						ProviderID:  "password",
						DisplayName: "display_name",
						Email:       "tefwfd1234eml5f@test.com",
					},
				},
				CustomClaims: map[string]interface{}{"asssssdf": true, "asssssdfdf": "ffd"},
			},
			PasswordHash: "pwhash==",
			PasswordSalt: "pwsalt==",
		},
		{
			UserRecord: &UserRecord{
				UserInfo: &UserInfo{
					DisplayName: "Test User",
					Email:       "testuser@example.com",
					UID:         "testuser0",
					PhoneNumber: "+1234567890",
					PhotoURL:    "http://www.example.com/testuser/photo.png",
				},
				UserMetadata: &UserMetadata{
					LastLogInTimestamp: 0,
					CreationTimestamp:  1234567890,
				},
				Disabled:      false,
				EmailVerified: true,
				ProviderUserInfo: []*UserInfo{
					{
						ProviderID:  "password",
						DisplayName: "Test User",
						Email:       "testuser@example.com",
						PhotoURL:    "http://www.example.com/testuser/photo.png",
					},
					{
						ProviderID:  "phone",
						PhoneNumber: "+1234567890",
					},
				},
				CustomClaims: map[string]interface{}{"admin": true, "package": "gold"},
			},
			PasswordHash: "passwordHash",
			PasswordSalt: "passwordSalt",
		},
	}
}

func testCompareUserRecords(u1, u2 *UserRecord, t *testing.T) {
	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"user", u1, u2},
		{"user.UID", u1.UID, u2.UID},
		{"user.UserInfo", u1.UserInfo, u2.UserInfo},
		{"user.ProviderUserInfo", u1.ProviderUserInfo, u2.ProviderUserInfo},
		{"user.UserMetadata", u1.UserMetadata, u2.UserMetadata},
	}
	for k, pui := range u1.ProviderUserInfo {
		tests = append(tests, struct {
			name string
			got  interface{}
			want interface{}
		}{fmt.Sprintf("Provider %d", k), pui, u2.ProviderUserInfo[k]})
	}
	for j, test := range tests {
		if !reflect.DeepEqual(test.got, test.want) {
			t.Errorf("test %d \n %s =  (%T) %#v \nwanted: (%T) %#v", j, test.name,
				test.got, test.got, test.want, test.want)
		}
	}

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
func echoServer(resp interface{}, t *testing.T) (*mockAuthServer, func()) {
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
			t.Error(err)
		}
		s.Req = append(s.Req, r)
		s.rbody = reqBody
		for k, v := range s.Header {
			w.Header().Set(k, v)
		}
		if s.Status != 0 {
			w.WriteHeader(s.Status)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(s.Resp)

	})
	s.srv = httptest.NewServer(handler)
	conf := &internal.AuthConfig{
		Creds: testCreds,
		Opts: []option.ClientOption{
			option.WithHTTPClient(s.srv.Client()),
		},
	}
	fmt.Println("AUTH CLIENT")
	authClient, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Error()
	}
	authClient.url = s.srv.URL + "/"
	s.Client = authClient
	return &s, s.srv.Close
}
