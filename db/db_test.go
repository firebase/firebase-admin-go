package db

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"

	"encoding/json"

	"reflect"

	"io/ioutil"

	"net/url"

	"firebase.google.com/go/internal"
	"google.golang.org/api/option"
)

const testURL = "https://test-db.firebaseio.com"

var testUserAgent string
var testAuthOverrides string
var testOpts = []option.ClientOption{
	option.WithTokenSource(&mockTokenSource{"mock-token"}),
}

var client *Client
var aoClient *Client
var ref *Ref

func TestMain(m *testing.M) {
	var err error
	client, err = NewClient(context.Background(), &internal.DatabaseConfig{
		Opts:    testOpts,
		BaseURL: testURL,
		Version: "1.2.3",
		AO:      map[string]interface{}{},
	})
	if err != nil {
		log.Fatalln(err)
	}

	ao := map[string]interface{}{"uid": "user1"}
	aoClient, err = NewClient(context.Background(), &internal.DatabaseConfig{
		Opts:    testOpts,
		BaseURL: testURL,
		Version: "1.2.3",
		AO:      ao,
	})
	if err != nil {
		log.Fatalln(err)
	}

	b, err := json.Marshal(ao)
	if err != nil {
		log.Fatalln(err)
	}
	testAuthOverrides = string(b)

	ref, err = client.NewRef("peter")
	if err != nil {
		log.Fatalln(err)
	}

	testUserAgent = fmt.Sprintf(userAgent, "1.2.3", runtime.Version())
	os.Exit(m.Run())
}

func TestNewClient(t *testing.T) {
	c, err := NewClient(context.Background(), &internal.DatabaseConfig{
		Opts:    testOpts,
		BaseURL: testURL,
		AO:      make(map[string]interface{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != testURL {
		t.Errorf("BaseURL = %q; want: %q", c.baseURL, testURL)
	}
	if c.hc == nil {
		t.Errorf("http.Client = nil; want non-nil")
	}
	if c.ao != "" {
		t.Errorf("AuthOverrides = %q; want %q", c.ao, "")
	}
}

func TestNewClientAuthOverrides(t *testing.T) {
	cases := []map[string]interface{}{
		nil,
		map[string]interface{}{"uid": "user1"},
	}
	for _, tc := range cases {
		c, err := NewClient(context.Background(), &internal.DatabaseConfig{
			Opts:    testOpts,
			BaseURL: testURL,
			AO:      tc,
		})
		if err != nil {
			t.Fatal(err)
		}
		if c.baseURL != testURL {
			t.Errorf("BaseURL = %q; want: %q", c.baseURL, testURL)
		}
		if c.hc == nil {
			t.Errorf("http.Client = nil; want non-nil")
		}
		b, err := json.Marshal(tc)
		if err != nil {
			t.Fatal(err)
		}
		if c.ao != string(b) {
			t.Errorf("AuthOverrides = %q; want %q", c.ao, string(b))
		}
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
		}
		if r.Path != tc.WantPath {
			t.Errorf("Path = %q; want = %q", r.Path, tc.WantPath)
		}
		if r.Key != tc.WantKey {
			t.Errorf("Key = %q; want = %q", r.Key, tc.WantKey)
		}
	}
}

func TestInvalidNewRef(t *testing.T) {
	cases := []string{"foo#", "foo.", "foo$", "foo[", "foo]"}
	for _, tc := range cases {
		r, err := client.NewRef(tc)
		if r != nil || err == nil {
			t.Errorf("NewRef(%q) = (%v, %v); want = (nil, err)", tc, r, err)
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
			}
			if r.client == nil {
				t.Errorf("Client = nil; want = %v", client)
			}
			if r.Key != tc.Want {
				t.Errorf("Key = %q; want = %q", r.Key, tc.Want)
			}
		} else if r != nil {
			t.Fatalf("Parent = %v; want = nil", r)
		}
	}
}

func TestChild(t *testing.T) {
	r, err := client.NewRef("/test")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		Path   string
		Want   string
		Parent string
	}{
		{"foo", "/test/foo", "/test"},
		{"foo/bar", "/test/foo/bar", "/test/foo"},
		{"foo/bar/", "/test/foo/bar", "/test/foo"},
	}
	for _, tc := range cases {
		c, err := r.Child(tc.Path)
		if err != nil {
			t.Fatal(err)
		}
		if c.Path != tc.Want {
			t.Errorf("Child(%q) = %q; want = %q", tc.Path, c.Path, tc.Want)
		}
		if c.Parent().Path != tc.Parent {
			t.Errorf("Child().Parent() = %q; want = %q", c.Parent().Path, tc.Parent)
		}
	}
}

func TestInvalidChild(t *testing.T) {
	r, err := client.NewRef("/test")
	if err != nil {
		t.Fatal(err)
	}

	cases := []string{"", "/", "/foo", "foo#bar"}
	for _, tc := range cases {
		c, err := r.Child(tc)
		if c != nil || err == nil {
			t.Errorf("Child(%q) = (%v, %v); want = (nil, err)", tc, c, err)
		}
	}
}

func checkOnlyRequest(t *testing.T, got []*testReq, want *testReq) {
	checkAllRequests(t, got, []*testReq{want})
}

func checkAllRequests(t *testing.T, got []*testReq, want []*testReq) {
	if len(got) != len(want) {
		t.Errorf("Request Count = %d; want = %d", len(got), len(want))
	}
	for i, r := range got {
		checkRequest(t, r, want[i])
	}
}

func checkRequest(t *testing.T, got, want *testReq) {
	if h := got.Header.Get("Authorization"); h != "Bearer mock-token" {
		t.Errorf("Authorization = %q; want = %q", h, "Bearer mock-token")
	}
	if h := got.Header.Get("User-Agent"); h != testUserAgent {
		t.Errorf("User-Agent = %q; want = %q", h, testUserAgent)
	}

	if got.Method != want.Method {
		t.Errorf("Method = %q; want = %q", got.Method, want.Method)
	}

	if got.Path != want.Path {
		t.Errorf("Path = %q; want = %q", got.Path, want.Path)
	}
	if len(want.Query) != len(got.Query) {
		t.Errorf("QueryParam = %v; want = %v", got.Query, want.Query)
	}
	for k, v := range want.Query {
		if got.Query[k] != v {
			t.Errorf("QueryParam(%v) = %v; want = %v", k, got.Query[k], v)
		}
	}
	for k, v := range want.Header {
		if got.Header.Get(k) != v[0] {
			t.Errorf("Header(%q) = %q; want = %q", k, got.Header.Get(k), v[0])
		}
	}
	if want.Body != nil {
		if h := got.Header.Get("Content-Type"); h != "application/json" {
			t.Errorf("User-Agent = %q; want = %q", h, "application/json")
		}
		var wi, gi interface{}
		if err := json.Unmarshal(want.Body, &wi); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(got.Body, &gi); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gi, wi) {
			t.Errorf("Body = %v; want = %v", gi, wi)
		}
	} else if len(got.Body) != 0 {
		t.Errorf("Body = %v; want empty", got.Body)
	}
}

type testReq struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
	Query  map[string]string
}

func newTestReq(r *http.Request) (*testReq, error) {
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(r.RequestURI)
	if err != nil {
		return nil, err
	}

	query := make(map[string]string)
	for k, v := range u.Query() {
		query[k] = v[0]
	}
	return &testReq{
		Method: r.Method,
		Path:   u.Path,
		Header: r.Header,
		Body:   b,
		Query:  query,
	}, nil
}

type mockServer struct {
	Resp   interface{}
	Header map[string]string
	Status int
	Reqs   []*testReq
	srv    *httptest.Server
}

func (s *mockServer) Start(c *Client) *httptest.Server {
	if s.srv != nil {
		return s.srv
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr, _ := newTestReq(r)
		s.Reqs = append(s.Reqs, tr)

		for k, v := range s.Header {
			w.Header().Set(k, v)
		}

		print := r.URL.Query().Get("print")
		if s.Status != 0 {
			w.WriteHeader(s.Status)
		} else if print == "silent" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		b, _ := json.Marshal(s.Resp)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	s.srv = httptest.NewServer(handler)
	c.baseURL = s.srv.URL
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

func serialize(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
