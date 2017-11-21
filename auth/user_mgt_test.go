package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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

func echoServer(resp interface{}) (*mockAuthServer, error) {
	var b []byte

	switch v := resp.(type) {
	case []byte:
		b = v
	default:
		var err error
		b, err = json.Marshal(resp)

		if err != nil {
			fmt.Println("marshaling error")
			return nil, err
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
	_ = err
	authClient.url = s.srv.URL + "/"
	s.client = authClient
	return &s, nil
}

func (s *mockAuthServer) Client() *Client {
	return s.client
}

func TestCreateParams(t *testing.T) {
	t1 := UserParams{
		DisplayName: p.String(""),
		Disabled:    p.Bool(false),
		CustomClaims: &map[string]interface{}{
			"asdf":  "ff",
			"asdff": true},
	}
	m, e := json.Marshal(t1)
	t2 := UserParams{
		DisplayName:  t1.DisplayName,
		CustomClaims: t1.CustomClaims,
	}
	fmt.Println(m, e)
	m, e = json.Marshal(t2)
	fmt.Println(m, e)

}
func TestExportPayload(t *testing.T) {

}

func TestGetUser(t *testing.T) {

	b, err := ioutil.ReadFile(internal.Resource("get_user.json"))
	if err != nil {
		log.Fatalln(err)
	}
	s, err := echoServer(b)
	if err != nil {
		log.Fatalln(err)
	}
	defer s.srv.Close()

	user, err := s.Client().GetUser(context.Background(), "ignored_id")
	if err != nil {
		t.Error(err)
	}
	tests := []struct {
		got  interface{}
		want interface{}
	}{
		{user.UID, "testuser"},
		{user.UserMetadata, &UserMetadata{CreationTimestamp: 1234567890, LastLogInTimestamp: 0}},
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
		{user.CustomClaims, &map[string]interface{}{"admin": true, "package": "gold"}},
	}
	for _, test := range tests {
		if !reflect.DeepEqual(test.want, test.got) {
			t.Errorf("got %#v wanted %#v", test.got, test.want)
		}
	}
}

// -- --
