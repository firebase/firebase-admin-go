package db

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"

	"encoding/json"

	"reflect"

	"firebase.google.com/go/internal"
	"google.golang.org/api/option"
)

const testURL = "https://test-db.firebaseio.com"

var testOpts = []option.ClientOption{
	option.WithTokenSource(&mockTokenSource{"mock-token"}),
}

var client *Client

func TestMain(m *testing.M) {
	var err error
	conf := &internal.DatabaseConfig{Opts: testOpts, BaseURL: testURL}
	client, err = NewClient(context.Background(), conf)
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestNewClient(t *testing.T) {
	c, err := NewClient(context.Background(), &internal.DatabaseConfig{
		Opts:    testOpts,
		BaseURL: testURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != testURL {
		t.Errorf("BaseURL = %q; want: %q", c.baseURL, testURL)
	} else if c.hc == nil {
		t.Errorf("http.Client = nil; want non-nil")
	}
}

func TestNewClientError(t *testing.T) {
	cases := []string{
		"",
		"foo",
		"http://db.firebaseio.com",
		"https://firebase.google.com",
	}
	for _, tc := range cases {
		c, err := NewClient(context.Background(), &internal.DatabaseConfig{
			Opts:    testOpts,
			BaseURL: tc,
		})
		if c != nil || err == nil {
			t.Errorf("NewClient() = (%v, %v); want = (nil, error)", c, err)
		}
	}
}

func TestNewRef(t *testing.T) {
	cases := []struct {
		Path     string
		WantPath string
		WantKey  string
	}{
		{"", "/", ""},
		{"/", "/", ""},
		{"foo", "/foo", "foo"},
		{"/foo", "/foo", "foo"},
		{"foo/bar", "/foo/bar", "bar"},
		{"/foo/bar", "/foo/bar", "bar"},
		{"/foo/bar/", "/foo/bar", "bar"},
	}
	for _, tc := range cases {
		r, err := client.NewRef(tc.Path)
		if err != nil {
			t.Fatal(err)
		}
		if r.client == nil {
			t.Errorf("Client = nil; want = %v", client)
		} else if r.Path != tc.WantPath {
			t.Errorf("Path = %q; want = %q", r.Path, tc.WantPath)
		} else if r.Key != tc.WantKey {
			t.Errorf("Key = %q; want = %q", r.Key, tc.WantKey)
		}
	}
}

func TestParent(t *testing.T) {
	cases := []struct {
		Path      string
		HasParent bool
		Want      string
	}{
		{"", false, ""},
		{"/", false, ""},
		{"foo", true, ""},
		{"/foo", true, ""},
		{"foo/bar", true, "foo"},
		{"/foo/bar", true, "foo"},
		{"/foo/bar/", true, "foo"},
	}
	for _, tc := range cases {
		r, err := client.NewRef(tc.Path)
		if err != nil {
			t.Fatal(err)
		}

		r = r.Parent()
		if tc.HasParent {
			if r == nil {
				t.Fatalf("Parent = nil; want = %q", tc.Want)
			} else if r.client == nil {
				t.Errorf("Client = nil; want = %v", client)
			} else if r.Key != tc.Want {
				t.Errorf("Key = %q; want = %q", r.Key, tc.Want)
			}
		} else if r != nil {
			t.Fatalf("Parent = %v; want = nil", r)
		}
	}
}

func TestGet(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	ref, err := client.NewRef("peter")
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := ref.Get(&got); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}

	checkRequestDefaults(t, mock.Req, 1)
	checkRequest(t, mock.Req[0], "GET", "/peter.json")
}

func TestGetWithStruct(t *testing.T) {
	want := person{Name: "Peter Parker", Age: 17}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	ref, err := client.NewRef("peter")
	if err != nil {
		t.Fatal(err)
	}

	var got person
	if err := ref.Get(&got); err != nil {
		t.Fatal(err)
	} else if want != got {
		t.Errorf("Get() = %v; want = %v", got, want)
	}

	checkRequestDefaults(t, mock.Req, 1)
	checkRequest(t, mock.Req[0], "GET", "/peter.json")
}

func TestSet(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	ref, err := client.NewRef("peter")
	if err != nil {
		t.Fatal(err)
	}

	if err := ref.Set(&want); err != nil {
		t.Fatal(err)
	}

	checkRequestDefaults(t, mock.Req, 1)
	checkRequest(t, mock.Req[0], "PUT", "/peter.json")
}

func TestSetWithStruct(t *testing.T) {
	want := &person{"Peter Parker", 17}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	ref, err := client.NewRef("peter")
	if err != nil {
		t.Fatal(err)
	}

	if err := ref.Set(&want); err != nil {
		t.Fatal(err)
	}

	checkRequestDefaults(t, mock.Req, 1)
	checkRequest(t, mock.Req[0], "PUT", "/peter.json")
}

func checkRequestDefaults(t *testing.T, req []*http.Request, num int) {
	if len(req) != num {
		t.Errorf("Request Count = %d; want = %d", len(req), num)
	}
	for _, r := range req {
		if h := r.Header.Get("Authorization"); h != "Bearer mock-token" {
			t.Errorf("Authorization = %q; want = %q", h, "Bearer mock-token")
		}
		if h := r.Header.Get("User-Agent"); h != userAgent {
			t.Errorf("User-Agent = %q; want = %q", h, userAgent)
		}
	}
}

func checkRequest(t *testing.T, r *http.Request, method, url string) {
	if r.Method != method {
		t.Errorf("Method = %q; want = %q", r.Method, method)
	}
	if r.RequestURI != url {
		t.Errorf("URL = %q; want = %q", r.RequestURI, url)
	}
}

type mockServer struct {
	Resp interface{}
	Req  []*http.Request
	srv  *httptest.Server
}

func (s *mockServer) Start(c *Client) *httptest.Server {
	if s.srv == nil {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.Req = append(s.Req, r)
			b, _ := json.Marshal(s.Resp)
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		})
		s.srv = httptest.NewServer(handler)
		c.baseURL = s.srv.URL
	}
	return s.srv
}

type mockTokenSource struct {
	AccessToken string
}

func (ts *mockTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: ts.AccessToken}, nil
}

type person struct {
	Name string `json:"name"`
	Age  int32  `json:"age"`
}
