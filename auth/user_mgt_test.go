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
	Rbody  []byte
	Srv    *httptest.Server
	Client *Client
}

var listUsers []*ExportedUserRecord

func TestGetUser(t *testing.T) {
	s := echoServer("get_user.json", t)
	defer s.Close()

	user, err := s.Client.GetUser(context.Background(), "ignored_id")
	if err != nil {
		t.Fatal(err)
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
		t.Errorf("GetUser(UID) = %#v, want: %#v", user, want)
		testCompareUserRecords("GetUser(UID)", user, want, t)
	}
}

func TestListUsers(t *testing.T) {
	setListUsers()
	s := echoServer("list_users.json", t)
	defer s.Close()
	iter := s.Client.Users(context.Background(), "")

	for i := 0; i < len(listUsers); i++ {
		user, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if i >= len(listUsers) {
			t.Errorf("Users() got %d users, want at least %d users", i+1, len(listUsers))
		}
		if !reflect.DeepEqual(user, listUsers[i]) {
			t.Errorf("Users() iterator [%d], got: %#v, want: %#v", i, user, listUsers[i])
			testCompareUserRecords("Users() iterator - Next()", user.UserRecord, listUsers[i].UserRecord, t)
		}
	}
	if _, err := iter.Next(); err != iterator.Done {
		t.Errorf("Users() itereator got more than %d users, want %d", len(listUsers), len(listUsers))
	}
}

func TestGetUserBy(t *testing.T) {
	s := echoServer(nil, t)
	defer s.Close()

	tests := []struct {
		name   string
		getfun func(context.Context, string) (*UserRecord, error)
		param  string
		want   string
	}{
		{"GetUser()",
			s.Client.GetUser,
			"uid",
			`{"localId":["uid"]}`,
		},
		{"GetUserByEmail()",
			s.Client.GetUserByEmail,
			"email@email.com",
			`{"email":["email@email.com"]}`,
		},
		{"GetUserByPhoneNumber",
			s.Client.GetUserByPhoneNumber,
			"+12341234123",
			`{"phoneNumber":["+12341234123"]}`,
		},
	}

	for _, test := range tests {
		test.getfun(context.Background(), test.param)
		if string(s.Rbody) != test.want {
			t.Errorf("%s [request body] =  %q; want: %q", test.name, s.Rbody, test.want)
		}
	}
}

func TestCreateUserValidatorsFail(t *testing.T) {
	badUserParams := []struct {
		params *UserToCreate
		want   string
	}{
		{
			(&UserToCreate{}).Password("short"),
			`password must be a string at least 6 characters long`,
		}, {
			(&UserToCreate{}).PhoneNumber(""),
			"phoneNumber must be a non-empty string",
		}, {
			(&UserToCreate{}).PhoneNumber("1234"),
			`invalid phoneNumber "1234". Must be a valid, E.164 compliant identifier`,
		}, {
			(&UserToCreate{}).PhoneNumber("+_!@#$"),
			`invalid phoneNumber "+_!@#$". Must be a valid, E.164 compliant identifier`,
		}, {
			(&UserToCreate{}).UID(""),
			`localId must be a non-empty string`,
		}, {
			(&UserToCreate{}).UID(strings.Repeat("a", 129)),
			"localId must be a string at most 128 characters long",
		}, {
			(&UserToCreate{}).DisplayName(""),
			`displayName must be a non-empty string`,
		}, {
			(&UserToCreate{}).PhotoURL(""),
			"photoUrl must be a non-empty string",
		}, {
			(&UserToCreate{}).Email(""),
			`email must be a non-empty string`,
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
	for i, test := range badUserParams {
		_, err := client.CreateUser(context.Background(), test.params)
		if err == nil {
			t.Errorf("[%d] error = nil; want: error %q", i, test.want)
		}
		if err.Error() != test.want {
			t.Errorf("[%d] error = %q; want error: %q", i, err.Error(), test.want)
		}
	}
}

func TestCreateUserValidatorsPass(t *testing.T) {
	s := echoServer([]byte(`{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"email": "",
		"localId": "expectedUserID"
	   }`), t)
	defer s.Close()
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

func TestUpdateParamsValidatorsFail(t *testing.T) {
	badParams := []struct {
		params *UserToUpdate
		want   string
	}{
		{
			nil,
			"params must not be empty for update",
		}, {
			&UserToUpdate{},
			"params must not be empty for update",
		}, {
			(&UserToUpdate{}).PhoneNumber("1"),
			`invalid phoneNumber "1". Must be a valid, E.164 compliant identifier`,
		}, {
			(&UserToUpdate{}).CustomClaims(map[string]interface{}{"a": strings.Repeat("a", 993)}),
			fmt.Sprintf("stringified JSON of CustomClaims must be a string at most %d characters long", maxLenPayloadCC),
		},
	}

	for _, res := range reservedClaims {
		badParams = append(
			badParams,
			struct {
				params *UserToUpdate
				want   string
			}{
				(&UserToUpdate{}).CustomClaims(map[string]interface{}{res: true}),
				fmt.Sprintf(`CustomClaims(%q: ...): claim %q is reserved, and must not be set`, res, res)})
	}

	for i, test := range badParams {
		_, err := client.UpdateUser(context.Background(), "uid", test.params)
		if err == nil {
			t.Errorf("[%d] UpdateUser() error = <nil>; want error: %s", i, test.want)
		}
		if err.Error() != test.want {
			t.Errorf(`[%d] UpdateUser() error = %q; want error: %q`, i, err.Error(), test.want)
		}
	}
}

func TestUpdateUserValidatorsPass(t *testing.T) {
	s := echoServer([]byte(`{
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
	defer s.Close()

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
		want string
	}{{
		map[string]interface{}{"a": strings.Repeat("a", 993)},
		fmt.Sprintf("stringified JSON of CustomClaims must be a string at most %d characters long", maxLenPayloadCC),
	}}

	for _, res := range reservedClaims {
		badUserParams = append(badUserParams,
			struct {
				cc   map[string]interface{}
				want string
			}{
				map[string]interface{}{res: true},
				fmt.Sprintf("CustomClaims(%q: ...): claim %q is reserved, and must not be set", res, res),
			})
	}

	for _, test := range badUserParams {
		err := client.SetCustomUserClaims(context.Background(), "uid", test.cc)
		if err == nil {
			t.Errorf("SetCustomUserClaims() error = <nil>; want error: %s", test.want)
		}
		if err.Error() != test.want {
			t.Errorf("SetCustomUserClaims() error = %q; want error: %q", err.Error(), test.want)
		}
	}
}

func TestDelete(t *testing.T) {
	s := echoServer([]byte(`{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"email": "",
		"localId": "expectedUserID"
	   }`), t)
	defer s.Close()
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
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exported, want) {
		// zero in
		t.Errorf("makeExportedUser() = %#v; want: %#v", exported, want)
		testCompareUserRecords("makeExportedUser()", exported.UserRecord, want.UserRecord, t)
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

func TestBadServer(t *testing.T) {
	s := badServer(t)
	defer s.Close()
	want := "http error status: 500; reason: {}"
	_, err := s.Client.GetUser(context.Background(), "some uid")
	if err == nil || err.Error() != want {
		t.Errorf("got error = %v; want: `%v`", err, want)
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

// for drilling down comparison.
func testCompareUserRecords(testName string, u1, u2 *UserRecord, t *testing.T) {
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
	for _, test := range tests {
		if !reflect.DeepEqual(test.got, test.want) {
			t.Errorf("%s = (%T) %#v;\nwant: (%T) %#v", testName,
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
	conf := &internal.AuthConfig{
		Opts: []option.ClientOption{
			option.WithHTTPClient(s.Srv.Client()),
		},
	}
	authClient, err := NewClient(context.Background(), conf)
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

func badServer(t *testing.T) *mockAuthServer {
	s := mockAuthServer{}
	s.Srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Req = append(s.Req, r)
		w.WriteHeader(500)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	conf := &internal.AuthConfig{
		Opts: []option.ClientOption{
			option.WithHTTPClient(s.Srv.Client()),
		},
	}
	authClient, err := NewClient(context.Background(), conf)
	if err != nil {
		t.Fatal()
	}
	authClient.url = s.Srv.URL + "/"
	s.Client = authClient
	return &s
}
