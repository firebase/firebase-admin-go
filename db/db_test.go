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

	"io/ioutil"

	"firebase.google.com/go/internal"
	"google.golang.org/api/option"
)

const testURL = "https://test-db.firebaseio.com"

var testOpts = []option.ClientOption{
	option.WithTokenSource(&mockTokenSource{"mock-token"}),
}

var client *Client
var ref *Ref

func TestMain(m *testing.M) {
	var err error
	conf := &internal.DatabaseConfig{Opts: testOpts, BaseURL: testURL}
	client, err = NewClient(context.Background(), conf)
	if err != nil {
		log.Fatalln(err)
	}

	ref, err = client.NewRef("peter")
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

	var got map[string]interface{}
	if err := ref.Get(&got); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestGetWithStruct(t *testing.T) {
	want := person{Name: "Peter Parker", Age: 17}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got person
	if err := ref.Get(&got); err != nil {
		t.Fatal(err)
	} else if want != got {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestWerlformedHttpError(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"error": "test error"}, Status: 500}
	srv := mock.Start(client)
	defer srv.Close()

	var got person
	err := ref.Get(&got)
	want := "http error status: 500; reason: test error"
	if err == nil || err.Error() != want {
		t.Errorf("Get() = %v; want = %v", err, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestUnexpectedHttpError(t *testing.T) {
	mock := &mockServer{Resp: "unexpected error", Status: 500}
	srv := mock.Start(client)
	defer srv.Close()

	var got person
	err := ref.Get(&got)
	want := "http error status: 500; message: \"unexpected error\""
	if err == nil || err.Error() != want {
		t.Errorf("Get() = %v; want = %v", err, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestSet(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	if err := ref.Set(want); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PUT",
		Path:   "/peter.json?print=silent",
		Body:   serialize(want),
	})
}

func TestSetWithStruct(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	want := &person{"Peter Parker", 17}
	if err := ref.Set(&want); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PUT",
		Path:   "/peter.json?print=silent",
		Body:   serialize(want),
	})
}

func TestPush(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"name": "new_key"}}
	srv := mock.Start(client)
	defer srv.Close()

	child, err := ref.Push(nil)
	if err != nil {
		t.Fatal(err)
	}

	if child.Key != "new_key" {
		t.Errorf("Push() = %q; want = %q", child.Key, "new_key")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "POST",
		Path:   "/peter.json",
	})
}

func TestPushWithValue(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"name": "new_key"}}
	srv := mock.Start(client)
	defer srv.Close()

	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	child, err := ref.Push(want)
	if err != nil {
		t.Fatal(err)
	}

	if child.Key != "new_key" {
		t.Errorf("Push() = %q; want = %q", child.Key, "new_key")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "POST",
		Path:   "/peter.json",
		Body:   serialize(want),
	})
}

func TestUpdate(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	if err := ref.Update(want); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PATCH",
		Path:   "/peter.json?print=silent",
		Body:   serialize(want),
	})
}

func TestInvalidUpdate(t *testing.T) {
	if err := ref.Update(nil); err == nil {
		t.Errorf("Update(nil) = nil; want error")
	}

	m := make(map[string]interface{})
	if err := ref.Update(m); err == nil {
		t.Errorf("Update(map{}) = nil; want error")
	}
}

func TestRemove(t *testing.T) {
	mock := &mockServer{Resp: "null"}
	srv := mock.Start(client)
	defer srv.Close()

	if err := ref.Remove(); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "DELETE",
		Path:   "/peter.json",
	})
}

func checkOnlyRequest(t *testing.T, got []*testReq, want *testReq) {
	checkAllRequests(t, got, []*testReq{want})
}

func checkAllRequests(t *testing.T, got []*testReq, want []*testReq) {
	if len(got) != len(want) {
		t.Errorf("Request Count = %d; want = %d", len(got), len(want))
	}
	for i, r := range got {
		if h := r.Header.Get("Authorization"); h != "Bearer mock-token" {
			t.Errorf("Authorization = %q; want = %q", h, "Bearer mock-token")
		}
		if h := r.Header.Get("User-Agent"); h != userAgent {
			t.Errorf("User-Agent = %q; want = %q", h, userAgent)
		}

		w := want[i]
		if r.Method != w.Method {
			t.Errorf("Method = %q; want = %q", r.Method, w.Method)
		}
		if r.Path != w.Path {
			t.Errorf("URL = %q; want = %q", r.Path, w.Path)
		}
		if w.Body != nil {
			if !reflect.DeepEqual(r.Body, w.Body) {
				t.Errorf("Body = %v; want = %v", string(r.Body), string(w.Body))
			}
		} else if len(r.Body) != 0 {
			t.Errorf("Body = %v; want empty", r.Body)
		}
	}
}

type mockServer struct {
	Resp   interface{}
	Status int
	Reqs   []*testReq
	srv    *httptest.Server
}

type testReq struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

func serialize(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func newTestReq(r *http.Request) (*testReq, error) {
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return &testReq{
		Method: r.Method,
		Path:   r.RequestURI,
		Header: r.Header,
		Body:   b,
	}, nil
}

func (s *mockServer) Start(c *Client) *httptest.Server {
	if s.srv == nil {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tr, _ := newTestReq(r)
			s.Reqs = append(s.Reqs, tr)

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
