// Copyright 2018 Google Inc. All Rights Reserved.
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

package db

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"testing"

	"firebase.google.com/go/internal"
	"google.golang.org/api/option"
)

const (
	testURL           = "https://test-db.firebaseio.com"
	defaultMaxRetries = 1
)

var (
	aoClient          *Client
	client            *Client
	testAuthOverrides string
	testref           *Ref
	testUserAgent     string

	testOpts = []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "mock-token"}),
	}
)

func TestMain(m *testing.M) {
	var err error
	client, err = NewClient(context.Background(), &internal.DatabaseConfig{
		Opts:         testOpts,
		URL:          testURL,
		Version:      "1.2.3",
		AuthOverride: map[string]interface{}{},
	})
	if err != nil {
		log.Fatalln(err)
	}
	retryConfig := client.hc.RetryConfig
	retryConfig.MaxRetries = defaultMaxRetries
	retryConfig.ExpBackoffFactor = 0

	ao := map[string]interface{}{"uid": "user1"}
	aoClient, err = NewClient(context.Background(), &internal.DatabaseConfig{
		Opts:         testOpts,
		URL:          testURL,
		Version:      "1.2.3",
		AuthOverride: ao,
	})
	if err != nil {
		log.Fatalln(err)
	}

	b, err := json.Marshal(ao)
	if err != nil {
		log.Fatalln(err)
	}
	testAuthOverrides = string(b)

	testref = client.NewRef("peter")
	testUserAgent = fmt.Sprintf(userAgentFormat, "1.2.3", runtime.Version())
	os.Exit(m.Run())
}

func TestNewClient(t *testing.T) {
	c, err := NewClient(context.Background(), &internal.DatabaseConfig{
		Opts:         testOpts,
		URL:          testURL,
		AuthOverride: make(map[string]interface{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.url != testURL {
		t.Errorf("NewClient().url = %q; want = %q", c.url, testURL)
	}
	if c.hc == nil {
		t.Errorf("NewClient().hc = nil; want non-nil")
	}
	if c.authOverride != "" {
		t.Errorf("NewClient().ao = %q; want = %q", c.authOverride, "")
	}
}

func TestNewClientAuthOverrides(t *testing.T) {
	cases := []map[string]interface{}{
		nil,
		{"uid": "user1"},
	}
	for _, tc := range cases {
		c, err := NewClient(context.Background(), &internal.DatabaseConfig{
			Opts:         testOpts,
			URL:          testURL,
			AuthOverride: tc,
		})
		if err != nil {
			t.Fatal(err)
		}
		if c.url != testURL {
			t.Errorf("NewClient(%v).url = %q; want = %q", tc, c.url, testURL)
		}
		if c.hc == nil {
			t.Errorf("NewClient(%v).hc = nil; want non-nil", tc)
		}
		b, err := json.Marshal(tc)
		if err != nil {
			t.Fatal(err)
		}
		if c.authOverride != string(b) {
			t.Errorf("NewClient(%v).ao = %q; want = %q", tc, c.authOverride, string(b))
		}
	}
}

func TestInvalidURL(t *testing.T) {
	cases := []string{
		"",
		"foo",
		"http://db.firebaseio.com",
		"https://firebase.google.com",
	}
	for _, tc := range cases {
		c, err := NewClient(context.Background(), &internal.DatabaseConfig{
			Opts: testOpts,
			URL:  tc,
		})
		if c != nil || err == nil {
			t.Errorf("NewClient(%q) = (%v, %v); want = (nil, error)", tc, c, err)
		}
	}
}

func TestInvalidAuthOverride(t *testing.T) {
	c, err := NewClient(context.Background(), &internal.DatabaseConfig{
		Opts:         testOpts,
		URL:          testURL,
		AuthOverride: map[string]interface{}{"uid": func() {}},
	})
	if c != nil || err == nil {
		t.Errorf("NewClient() = (%v, %v); want = (nil, error)", c, err)
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
		r := client.NewRef(tc.Path)
		if r.client == nil {
			t.Errorf("NewRef(%q).client = nil; want = %v", tc.Path, r.client)
		}
		if r.Path != tc.WantPath {
			t.Errorf("NewRef(%q).Path = %q; want = %q", tc.Path, r.Path, tc.WantPath)
		}
		if r.Key != tc.WantKey {
			t.Errorf("NewRef(%q).Key = %q; want = %q", tc.Path, r.Key, tc.WantKey)
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
		r := client.NewRef(tc.Path).Parent()
		if tc.HasParent {
			if r == nil {
				t.Fatalf("Parent(%q) = nil; want = Ref(%q)", tc.Path, tc.Want)
			}
			if r.client == nil {
				t.Errorf("Parent(%q).client = nil; want = %v", tc.Path, client)
			}
			if r.Key != tc.Want {
				t.Errorf("Parent(%q).Key = %q; want = %q", tc.Path, r.Key, tc.Want)
			}
		} else if r != nil {
			t.Fatalf("Parent(%q) = %v; want = nil", tc.Path, r)
		}
	}
}

func TestChild(t *testing.T) {
	r := client.NewRef("/test")
	cases := []struct {
		Path   string
		Want   string
		Parent string
	}{
		{"", "/test", "/"},
		{"foo", "/test/foo", "/test"},
		{"/foo", "/test/foo", "/test"},
		{"foo/", "/test/foo", "/test"},
		{"/foo/", "/test/foo", "/test"},
		{"//foo//", "/test/foo", "/test"},
		{"foo/bar", "/test/foo/bar", "/test/foo"},
		{"/foo/bar", "/test/foo/bar", "/test/foo"},
		{"foo/bar/", "/test/foo/bar", "/test/foo"},
		{"/foo/bar/", "/test/foo/bar", "/test/foo"},
		{"//foo/bar", "/test/foo/bar", "/test/foo"},
		{"foo//bar/", "/test/foo/bar", "/test/foo"},
		{"foo/bar//", "/test/foo/bar", "/test/foo"},
	}
	for _, tc := range cases {
		c := r.Child(tc.Path)
		if c.Path != tc.Want {
			t.Errorf("Child(%q) = %q; want = %q", tc.Path, c.Path, tc.Want)
		}
		if c.Parent().Path != tc.Parent {
			t.Errorf("Child(%q).Parent() = %q; want = %q", tc.Path, c.Parent().Path, tc.Parent)
		}
	}
}

func checkOnlyRequest(t *testing.T, got []*testReq, want *testReq) {
	checkAllRequests(t, got, []*testReq{want})
}

func checkAllRequests(t *testing.T, got []*testReq, want []*testReq) {
	if len(got) != len(want) {
		t.Errorf("Request Count = %d; want = %d", len(got), len(want))
	} else {
		for i, r := range got {
			checkRequest(t, r, want[i])
		}
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
	c.url = s.srv.URL
	return s.srv
}

type person struct {
	Name string `json:"name"`
	Age  int32  `json:"age"`
}

func serialize(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
