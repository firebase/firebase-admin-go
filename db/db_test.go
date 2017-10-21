package db

import (
	"net/http"
	"net/http/httptest"
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
	c := newTestClient(t)
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
		r, err := c.NewRef(tc.Path)
		if err != nil {
			t.Fatal(err)
		}
		if r.client == nil {
			t.Errorf("Client = nil; want = %v", c)
		} else if r.Path != tc.WantPath {
			t.Errorf("Path = %q; want = %q", r.Path, tc.WantPath)
		} else if r.Key != tc.WantKey {
			t.Errorf("Key = %q; want = %q", r.Key, tc.WantKey)
		}
	}
}

func TestParent(t *testing.T) {
	c := newTestClient(t)
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
		r, err := c.NewRef(tc.Path)
		if err != nil {
			t.Fatal(err)
		}

		r = r.Parent()
		if tc.HasParent {
			if r == nil {
				t.Fatalf("Parent = nil; want = %q", tc.Want)
			} else if r.client == nil {
				t.Errorf("Client = nil; want = %v", c)
			} else if r.Key != tc.Want {
				t.Errorf("Key = %q; want = %q", r.Key, tc.Want)
			}
		} else if r != nil {
			t.Fatalf("Parent = %v; want = nil", r)
		}
	}
}

func TestGet(t *testing.T) {
	want := map[string]interface{}{
		"name": "Peter Parker",
		"age":  float64(17),
	}
	c := newTestClient(t)
	mock, err := newMockServer(want)
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Srv.Close()
	c.baseURL = mock.Srv.URL

	ref, err := c.NewRef("peter")
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := ref.Get(&got); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkRequests(t, mock.Req, 1)
}

func TestGetWithStruct(t *testing.T) {
	want := person{Name: "Peter Parker", Age: 17}
	c := newTestClient(t)
	mock, err := newMockServer(want)
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Srv.Close()
	c.baseURL = mock.Srv.URL

	ref, err := c.NewRef("peter")
	if err != nil {
		t.Fatal(err)
	}

	var got person
	if err := ref.Get(&got); err != nil {
		t.Fatal(err)
	} else if want != got {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkRequests(t, mock.Req, 1)
}

func newTestClient(t *testing.T) *Client {
	c, err := NewClient(context.Background(), &internal.DatabaseConfig{
		Opts:    testOpts,
		BaseURL: testURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func checkRequests(t *testing.T, req []*http.Request, num int) {
	if len(req) != num {
		t.Errorf("Request Count = %d; want = %d", len(req), num)
	}
	for _, r := range req {
		if h := r.Header.Get("Authorization"); h != "Bearer mock-token" {
			t.Errorf("Authorization = %q; want = %q", h, "Bearer mock-token")
		} else if h := r.Header.Get("User-Agent"); h != userAgent {
			t.Errorf("User-Agent = %q; want = %q", h, userAgent)
		}
	}
}

func newMockServer(v interface{}) (*mockServer, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	mock := &mockServer{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.Req = append(mock.Req, r)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	mock.Srv = httptest.NewServer(handler)
	return mock, nil
}

type mockServer struct {
	Req []*http.Request
	Srv *httptest.Server
}

type mockTokenSource struct {
	AccessToken string
}

type person struct {
	Name string `json:"name"`
	Age  int32  `json:"age"`
}

func (ts *mockTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: ts.AccessToken}, nil
}
