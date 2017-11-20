package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"firebase.google.com/go/internal"
	"firebase.google.com/go/p"
	"golang.org/x/net/context"
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

func echoServer(resp interface{}) *mockAuthServer {

	//	if reflect.ValueOf(resp).Type() == reflect.ValueOf([]byte("")) {
	//b = []byte()
	//	}

	var b []byte

	b, _ = json.Marshal(resp)
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
	_ = err
	authClient.url = s.srv.URL + "/"
	s.client = authClient
	return &s
}
func (s *mockAuthServer) Client() *Client {
	return s.client
}

func TestCreateParams(t *testing.T) {
	t1 := UserCreateParams{
		DisplayName: p.String(""),
		Disabled:    p.Bool(false),
		CustomClaims: &CustomClaimsMap{"asdf": "ff",
			"asdff": "ffdf"},
	}
	m, e := json.Marshal(t1)
	t2 := UserCreateParams{
		DisplayName:  t1.DisplayName,
		CustomClaims: t1.CustomClaims,
	}
	fmt.Println(m, e)
	m, e = json.Marshal(t2)
	fmt.Println(m, e)

}
func TestExportPayload(t *testing.T) {

}

/*
users := []map[string]interface{}{
{
        "localId" : "testuser0",
        "email" : "testuser@example.com",
        "phoneNumber" : "+1234567890",
        "emailVerified" : true,
        "displayName" : "Test User",
        "providerUserInfo" : [ {
            "providerId" : "password",
            "displayName" : "Test User",
            "photoUrl" : "http://www.example.com/testuser/photo.png",
            "federatedId" : "testuser@example.com",
            "email" : "testuser@example.com",
            "rawId" : "testuser@example.com"
        }, {
            "providerId" : "phone",
            "phoneNumber" : "+1234567890",
            "rawId" : "+1234567890"
        } ],
        "photoUrl" : "http://www.example.com/testuser/photo.png",
        "passwordHash" : "passwordHash",
        "salt": "passwordSalt",
        "passwordUpdatedAt" : 1.494364393E+12,
        "validSince" : "1494364393",
        "disabled" : false,
        "createdAt" : "1234567890",
        "customAttributes" : "{\"admin\": true, \"package\": \"gold\"}"
    }, {
        "localId" : "testuser1",
        "email" : "testuser@example.com",
        "phoneNumber" : "+1234567890",
        "emailVerified" : true,
        "displayName" : "Test User",
        "providerUserInfo" : [ {
            "providerId" : "password",
            "displayName" : "Test User",
            "photoUrl" : "http://www.example.com/testuser/photo.png",
            "federatedId" : "testuser@example.com",
            "email" : "testuser@example.com",
            "rawId" : "testuser@example.com"
        }, {
            "providerId" : "phone",
            "phoneNumber" : "+1234567890",
            "rawId" : "+1234567890"
        } ],
        "photoUrl" : "http://www.example.com/testuser/photo.png",
        "passwordHash" : "passwordHash",
        "salt": "passwordSalt",
        "passwordUpdatedAt" : 1.494364393E+12,
        "validSince" : "1494364393",
        "disabled" : false,
        "createdAt" : "1234567890",
        "customAttributes" : "{\"admin\": true, \"package\": \"gold\"}"
    } ]
*/
func TestGetUser(t *testing.T) {
	/*
		b, err := ioutil.ReadFile(internal.Resource("get_user_data.json"))
		if err != nil {
			log.Fatalln(err)
		}*/
	//	s2 := echoServer(true, b)
	//	defer s2.srv.Close()
	s := echoServer(map[string]interface{}{
		"kind": "identitytoolkit#GetAccountInfoResponse",
		"users": []map[string]interface{}{
			{
				"localId":       "ZY1rJK0...",
				"email":         "user@example.com",
				"emailVerified": false,
				"displayName":   "John Doe",
				"providerUserInfo": []map[string]interface{}{
					{
						"providerId":  "password",
						"displayName": "John Doe",
						"photoUrl":    "http://localhost:8080/img1234567890/photo.png",
						"email":       "user@example.com",
					},
				},
				"photoUrl":     "https://lh5.googleusercontent.com/.../photo.jpg",
				"passwordHash": "...",
				"disabled":     false,
				"lastLoginAt":  "1484628946000",
				"createdAt":    "1484124142000",
			},
		},
	})

	defer s.srv.Close()
	for _, serv := range []*mockAuthServer{s} {
		user, err := serv.Client().GetUser(context.Background(), "ignored_id")
		if err != nil {
			t.Error(err)

		}
		tests := []struct {
			got  interface{}
			want interface{}
		}{
			{user.UID, "ZY1rJK0..."},
			{user.UserMetadata, &UserMetadata{CreationTimestamp: 1484124142000, LastLogInTimestamp: 1484628946000}},
			{user.Email, "user@example.com"},
			{user.EmailVerified, false},
			{user.PhotoURL, "https://lh5.googleusercontent.com/.../photo.jpg"},
			{user.Disabled, false},
			{user.ProviderUserInfo, []*UserInfo{
				{
					ProviderID:  "password",
					DisplayName: "John Doe",
					PhotoURL:    "http://localhost:8080/img1234567890/photo.png",
					Email:       "user@example.com",
				},
			}},
			{user.DisplayName, "John Doe"},
			{user.PasswordHash, "..."},
		}
		for _, test := range tests {
			if !reflect.DeepEqual(test.want, test.got) {
				t.Errorf("got %#v wanted %#v", test.got, test.want)
			}
		}
	}

}

// -- --
