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
	"firebase.google.com/go/ptr"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type mockAuthServer struct {
	Resp   []byte
	Header map[string]string
	Status int
	Req    *http.Request
	srv    *httptest.Server
	client *Client
}

// echoServer takes either a []byte or a string filename, or an object
// it returns a server whose client will echo either
//   the []byte it got
//   the contents of the file named by the string in []byte form
//   the marshalled object, in []byte form
// it also returns a closing functions that has to be defer closed
func echoServer(resp interface{}, t *testing.T) (*mockAuthServer, func()) {
	var b []byte
	var err error
	switch v := resp.(type) {
	case []byte:
		b = v
	case string:
		b, err = ioutil.ReadFile(internal.Resource(v))
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

		s.Req = r
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
	authClient, err := NewClient(context.Background(),
		&internal.AuthConfig{
			Opts: []option.ClientOption{option.WithHTTPClient(s.srv.Client())},
		})
	if err != nil {
		t.Error()
	}
	authClient.url = s.srv.URL + "/"
	s.client = authClient
	return &s, s.srv.Close
}

func (s *mockAuthServer) Client() *Client {
	return s.client
}
func TestSetClaimsField(t *testing.T) {
	tests := []struct {
		p          UserParams
		attributes string
	}{
		{
			UserParams{
				CustomClaims: &map[string]interface{}{
					"asdf":  "ff",
					"asdff": true,
				},
			},
			"{\"asdf\":\"ff\",\"asdff\":true}",
		}, {
			UserParams{
				CustomClaims: &map[string]interface{}{},
			},
			"{}",
		}, {
			UserParams{
				CustomClaims: &map[string]interface{}{
					"integer": 12,
					"number":  12.3,
				},
			},
			"{\"integer\":12,\"number\":12.3}",
		},
	}
	for _, test := range tests {
		up := userParams{UserParams: &test.p}

		up.setClaimsField()
		if *up.CustomAttributes != test.attributes {
			t.Errorf("wrong custom attribute string got %s expecting %s", *up.CustomAttributes, test.attributes)

		}
	}
}

func TestDeleteParams(t *testing.T) {
	tests := []struct {
		params UserParams
		up     userParams
	}{
		{
			UserParams{
				DisplayName: ptr.String(""),
			},
			userParams{
				UserParams:          &UserParams{},
				DeleteAttributeList: []string{"DISPLAY_NAME"},
			},
		},
		{
			UserParams{
				PhotoURL: ptr.String(""),
			},
			userParams{
				UserParams:          &UserParams{},
				DeleteAttributeList: []string{"PHOTO_URL"},
			},
		}, {
			UserParams{
				DisplayName: ptr.String(""),
				PhotoURL:    ptr.String(""),
			},
			userParams{
				UserParams:          &UserParams{},
				DeleteAttributeList: []string{"DISPLAY_NAME", "PHOTO_URL"},
			},
		},
		{
			UserParams{
				PhoneNumber: ptr.String(""),
			},
			userParams{
				UserParams:         &UserParams{},
				DeleteProviderList: []string{"phone"},
			},
		},
	}
	for i, test := range tests {
		up := userParams{UserParams: &test.params}
		up.setDeleteFields()
		if !reflect.DeepEqual(up, test.up) {
			t.Errorf("test %d: got %#v expecting %#v", i, up, test.up)
		}
	}
}

func TestGetUser(t *testing.T) {
	s, closer := echoServer("get_user.json", t)
	defer closer()

	user, err := s.Client().GetUser(context.Background(), "ignored_id")
	if err != nil {
		t.Error(err)
	}
	tests := []struct {
		got  interface{}
		want interface{}
	}{
		{user.UID, "testuser"},
		{user.UserMetadata, &UserMetadata{CreationTimestamp: 1234567890, LastLogInTimestamp: 1233211232}},
		{user.Email, "testuser@example.com"},
		{user.EmailVerified, true},
		{user.PhotoURL, "http://www.example.com/testuser/photo.png"},
		{user.Disabled, false},
		{user.ProviderUserInfo, []*UserInfo{
			{
				ProviderID:  "password",
				DisplayName: "Test User",
				PhotoURL:    "http://www.example.com/testuser/photo.png",
				Email:       "testuser@example.com",
			}, {
				ProviderID:  "phone",
				PhoneNumber: "+1234567890",
			},
		}},
		{user.DisplayName, "Test User"},
		{user.PasswordHash, "passwordhash"},
		{user.PasswordSalt, "salt==="},
		{user.CustomClaims, &map[string]interface{}{"admin": true, "package": "gold"}},
	}
	for _, test := range tests {
		if !reflect.DeepEqual(test.want, test.got) {
			t.Errorf("got %#v wanted %#v", test.got, test.want)
		}
	}
}

var listUsers []*ExportedUserRecord

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
				CustomClaims: &map[string]interface{}{"asssssdf": true, "asssssdfdf": "ffd"},
			},
			PasswordHash: "V4X0yt9qGyp6cfw6BNwRHdS4SDwgTKtUSZcW2LEBFRuadpYJePqOsHyNtEszBaO3veC_6eA24PF06gH61Ghq8w==",
			PasswordSalt: "BxzGq0di67rcTw==",
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
				CustomClaims: &map[string]interface{}{"admin": true, "package": "gold"},
			},
			PasswordHash: "passwordHash",
			PasswordSalt: "passwordSalt",
		},
	}
}

type basicCompare struct {
	got  interface{}
	want interface{}
}

func TestGetUsers(t *testing.T) {
	setListUsers()
	s, closer := echoServer("list_users.json", t)
	defer closer()
	iter := s.Client().Users(context.Background(), "")
	i := 0
	for {
		user, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Error(err)

		}
		tests := []basicCompare{
			{user, listUsers[i]},
			{user.CustomClaims, listUsers[i].CustomClaims},
			{user.UserInfo, listUsers[i].UserInfo},
			{user.UserRecord, listUsers[i].UserRecord},
			{user.ProviderUserInfo, listUsers[i].ProviderUserInfo},
			{user.UserMetadata, listUsers[i].UserMetadata},
			{user.PasswordHash, listUsers[i].PasswordHash},
			{user.PasswordSalt, listUsers[i].PasswordSalt},
		}
		for k, pui := range user.ProviderUserInfo {
			tests = append(tests, basicCompare{pui, listUsers[i].ProviderUserInfo[k]})
		}
		for j, test := range tests {
			if !reflect.DeepEqual(test.got, test.want) {
				t.Errorf("item %d %d \ngot:    %T %#v \nwanted: %T %#v", i, j,
					test.got, test.got, test.want, test.want)
			}
		}
		i++
	}

}

type badParamsTest struct {
	params         *UserParams
	expectingError string
}

func TestBadCreateUser(t *testing.T) {
	badUserParams := []badParamsTest{
		{
			&UserParams{Password: ptr.String("short")},
			"error in params: Password must be at least 6 chars long",
		}, {
			&UserParams{PhoneNumber: ptr.String("1234")},
			"error in params: PhoneNumber must begin with a +",
		}, {
			&UserParams{PhoneNumber: ptr.String("+_!@#$")},
			"error in params: PhoneNumber must contain an alphanumeric character",
		}, {
			&UserParams{UID: ptr.String("")},
			"error in params: UID must be at least 1 chars long",
		}, {
			&UserParams{UID: ptr.String(strings.Repeat("a", 129))},
			"error in params: UID must be at most 128 chars long",
		}, {
			&UserParams{DisplayName: ptr.String("")},
			"error in params: DisplayName must be at least 1 chars long",
		}, {
			&UserParams{PhotoURL: ptr.String("")},
			"error in params: PhotoURL must be at least 1 chars long",
		}, {
			&UserParams{Email: ptr.String("")},
			"error in params: Email must be at least 1 chars long",
		}, {
			&UserParams{Email: ptr.String("a")},
			"error in params: Email must contain exactly one '@' sign",
		}, {
			&UserParams{Email: ptr.String("a@")},
			"error in params: Email must have non empty account and domain",
		}, {
			&UserParams{Email: ptr.String("@a")},
			"error in params: Email must have non empty account and domain",
		}, {
			&UserParams{Email: ptr.String("a@a@a")},
			"error in params: Email must contain exactly one '@' sign",
		},
	}
	for i, test := range badUserParams {

		_, err := client.CreateUser(context.Background(), test.params)
		if err == nil {
			t.Errorf("%d) expecting error %s", i, test.expectingError)
		}
		if err.Error() != test.expectingError {
			t.Errorf("got error: \"%s\" expecting error: \"%s\"", err.Error(), test.expectingError)
		}
	}

}

func TestGoodCreateParams(t *testing.T) {
	s, closer := echoServer([]byte(`{
		"kind": "identitytoolkit#SignupNewUserResponse",
		"email": "",
		"localId": "expectedUserID"
	   }`), t)
	defer closer()
	goodParams := []*UserParams{
		nil,
		{},
		{Password: ptr.String("123456")},
		{UID: ptr.String("1")},
		{UID: ptr.String(strings.Repeat("a", 128))},
		{PhoneNumber: ptr.String("+1")},
		{DisplayName: ptr.String("a")},
		{Email: ptr.String("a@a")},
		{PhoneNumber: ptr.String("+1")},
		{CustomClaims: &map[string]interface{}{"a": strings.Repeat("a", 992)}},
	}
	for _, par := range goodParams {
		_, err := s.Client().CreateUser(context.Background(), par)
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

	badParams := []badParamsTest{
		{
			&UserParams{UID: ptr.String("inparamstruct")},
			"uid mismatch",
		}, {
			&UserParams{CustomClaims: &map[string]interface{}{"a": strings.Repeat("a", 993)}},
			"error in params: stringified JSON claims must be at most 1000 chars long",
		},
	}

	for _, res := range reservedClaims {
		badParams = append(badParams,
			badParamsTest{&UserParams{CustomClaims: &map[string]interface{}{res: true}},
				fmt.Sprintf("error in params: %s is a reserved claim", res)})
	}

	for i, test := range badParams {
		_, err := client.UpdateUser(context.Background(), "outofstruct", test.params)
		if err == nil {
			t.Errorf("%d) expecting error %s", i, test.expectingError)
		}
		if err.Error() != test.expectingError {
			t.Errorf("got error \"%s\" expecting error \"%s\"", err.Error(), test.expectingError)
		}
	}
}
func TestGoodUpdateParams(t *testing.T) {
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

	goodParams := []*UserParams{
		nil,
		{},
		{Password: ptr.String("123456")},
		{UID: ptr.String("expectedUserID")},
		{PhoneNumber: ptr.String("+1")},
		{DisplayName: ptr.String("a")},
		{Email: ptr.String("a@a")},
		{PhoneNumber: ptr.String("+1")},
		{CustomClaims: &map[string]interface{}{"a": strings.Repeat("a", 992)}},
	}

	for _, par := range goodParams {
		_, err := s.Client().UpdateUser(context.Background(), "expectedUserID", par)
		// There are two calls to the server, the first one, on creation retunrs the above []byte
		// that's how we know the params passed validation
		// the second call to GetUser, tries to get the user with the returned ID above, it fails
		// with the following expected error
		if err.Error() != "cannot find user map[localId:[expectedUserID]]" {
			t.Error(err)
		}
	}
}

type ccErr struct {
	cc   *map[string]interface{}
	estr string
}

func TestBadSetCustomClaims(t *testing.T) {
	badUserParams := []*ccErr{{
		&map[string]interface{}{"a": strings.Repeat("a", 993)},
		"error in params: stringified JSON claims must be at most 1000 chars long",
	}}

	for _, res := range reservedClaims {
		badUserParams = append(badUserParams,
			&ccErr{
				cc:   &map[string]interface{}{res: true},
				estr: fmt.Sprintf("error in params: %s is a reserved claim", res),
			})
	}

	for i, test := range badUserParams {
		err := client.SetCustomUserClaims(context.Background(), "uid", test.cc)
		if err == nil {
			t.Errorf("%d) expecting error %s", i, test.estr)
		}
		if err.Error() != test.estr {
			t.Errorf("got error: \"%s\" expecting error: \"%s\"", err.Error(), test.estr)
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
	if err := s.Client().DeleteUser(context.Background(), ""); err != nil {
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
		CustomClaims:       "{\"admin\": true, \"package\": \"gold\"}",
	}
	eur := &ExportedUserRecord{
		UserRecord: &UserRecord{
			UserInfo: &UserInfo{
				UID:         "testuser",
				Email:       "testuser@example.com",
				PhoneNumber: "+1234567890",
				PhotoURL:    "http://www.example.com/testuser/photo.png",
				DisplayName: "Test User",
			},
			CustomClaims:  &map[string]interface{}{"admin": true, "package": "gold"},
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
	if !reflect.DeepEqual(exported, eur) {
		// zero in
		tests := []struct {
			i1 interface{}
			i2 interface{}
		}{
			{exported.UserInfo, eur.UserInfo},
			{exported.CustomClaims, eur.CustomClaims},
			{exported.Disabled, eur.Disabled},
			{exported.EmailVerified, eur.EmailVerified},
			{exported.UserMetadata, eur.UserMetadata},
			{exported.ProviderUserInfo, eur.ProviderUserInfo},
			{exported.PasswordHash, eur.PasswordHash},
			{exported.PasswordSalt, eur.PasswordSalt},
			{exported.UserRecord, eur.UserRecord},
			{exported, eur},
		}
		for i, t1 := range tests {
			if !reflect.DeepEqual(t1.i1, t1.i2) {
				t.Errorf("comparison %d: got %#v \nexpecting %#v", i, t1.i1, t1.i2)
			}
		}
	}

}

func provString(e *ExportedUserRecord) string {
	providerStr := ""
	if e.ProviderUserInfo != nil {
		for _, info := range e.ProviderUserInfo {
			providerStr += fmt.Sprintf("\n            %#v", info)
		}
	}
	return providerStr
}

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

func TestValidateStringLTE(t *testing.T) {
	tests := []struct {
		testLength  int
		expectedErr *string
	}{
		{5, nil},
		{4, nil},
		{3, ptr.String("test must be at most 3 chars long")},
	}
	testString := "1234"
	for _, test := range tests {
		if errstr := validateStringLenLTE(&testString, "test", test.testLength); errstr != test.expectedErr {
			testErrorOnStrPointers(errstr, test.expectedErr, t)
		}
	}
}
func TestValidateStringGTE(t *testing.T) {
	tests := []struct {
		testLength  int
		expectedErr *string
	}{
		{3, nil},
		{4, nil},
		{5, ptr.String("test must be at least 5 chars long")},
	}
	testString := "1234"
	for _, test := range tests {
		if errstr := validateStringLenGTE(&testString, "test", test.testLength); errstr != test.expectedErr {
			testErrorOnStrPointers(errstr, test.expectedErr, t)
		}
	}
}

func TestValidatePhoneNumber(t *testing.T) {
	tests := []struct {
		phone       string
		expectedErr *string
	}{
		{"", ptr.String("PhoneNumber cannot be empty")},
		{"123", ptr.String("PhoneNumber must begin with a +")},
		{"+_!@#$", ptr.String("PhoneNumber must contain an alphanumeric character")},
		{"+1", nil},
	}
	for _, test := range tests {
		if errstr := validatePhoneNumber(&test.phone); errstr != test.expectedErr {
			testErrorOnStrPointers(errstr, test.expectedErr, t)
		}
	}
}

func testErrorOnStrPointers(got, expected *string, t *testing.T) {
	if got == nil {
		t.Errorf("got nil error string, expecting %s", *expected)
	} else if expected == nil {
		t.Errorf("got error string %s, expecting nil", *got)
	} else if *got != *expected {
		t.Errorf("got err: %v, expecting %v", got, expected)
	}
}
