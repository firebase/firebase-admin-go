package internal

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

var cases = []struct {
	req     *Request
	method  string
	body    interface{}
	headers map[string]string
	query   map[string]string
}{
	{
		req: &Request{
			Method: "GET",
		},
		method: "GET",
	},
	{
		req: &Request{
			Method: "GET",
			Opts: []HTTPOption{
				WithHeader("Test-Header", "value1"),
				WithQueryParam("testParam", "value2"),
			},
		},
		method:  "GET",
		headers: map[string]string{"Test-Header": "value1"},
		query:   map[string]string{"testParam": "value2"},
	},
	{
		req: &Request{
			Method: "POST",
			Body:   map[string]string{"foo": "bar"},
			Opts: []HTTPOption{
				WithHeader("Test-Header", "value1"),
				WithQueryParam("testParam1", "value2"),
				WithQueryParam("testParam2", "value3"),
			},
		},
		method:  "POST",
		body:    map[string]string{"foo": "bar"},
		headers: map[string]string{"Test-Header": "value1"},
		query:   map[string]string{"testParam1": "value2", "testParam2": "value3"},
	},
	{
		req: &Request{
			Method: "POST",
			Body:   "body",
			Opts: []HTTPOption{
				WithHeader("Test-Header", "value1"),
				WithQueryParams(map[string]string{"testParam1": "value2", "testParam2": "value3"}),
			},
		},
		method:  "POST",
		body:    "body",
		headers: map[string]string{"Test-Header": "value1"},
		query:   map[string]string{"testParam1": "value2", "testParam2": "value3"},
	},
	{
		req: &Request{
			Method: "PUT",
			Body:   Null,
			Opts: []HTTPOption{
				WithHeader("Test-Header", "value1"),
				WithQueryParams(map[string]string{"testParam1": "value2", "testParam2": "value3"}),
			},
		},
		method:  "PUT",
		body:    Null,
		headers: map[string]string{"Test-Header": "value1"},
		query:   map[string]string{"testParam1": "value2", "testParam2": "value3"},
	},
}

func TestSend(t *testing.T) {
	want := map[string]interface{}{
		"key1": "value1",
		"key2": float64(100),
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	idx := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := cases[idx]
		if r.Method != want.method {
			t.Errorf("[%d] Method = %q; want = %q", idx, r.Method, want.method)
		}
		for k, v := range want.headers {
			h := r.Header.Get(k)
			if h != v {
				t.Errorf("[%d] Header(%q) = %q; want = %q", idx, k, h, v)
			}
		}
		if want.query == nil {
			if r.URL.Query().Encode() != "" {
				t.Errorf("[%d] Query = %v; want = empty", idx, r.URL.Query().Encode())
			}
		}
		for k, v := range want.query {
			q := r.URL.Query().Get(k)
			if q != v {
				t.Errorf("[%d] Query(%q) = %q; want = %q", idx, k, q, v)
			}
		}
		if want.body != nil {
			h := r.Header.Get("Content-Type")
			if h != "application/json" {
				t.Errorf("[%d] Content-Type = %q; want = %q", idx, h, "application/json")
			}

			var wb []byte
			if want.body == Null {
				wb = []byte("null")
			} else {
				wb, err = json.Marshal(want.body)
				if err != nil {
					t.Fatal(err)
				}
			}
			gb, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(wb, gb) {
				t.Errorf("[%d] Body = %q; want = %q", idx, string(gb), string(wb))
			}
		}

		idx++
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	for _, tc := range cases {
		tc.req.URL = server.URL
		resp, err := tc.req.Send(context.Background(), http.DefaultClient)
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.CheckStatus(http.StatusOK, nil); err != nil {
			t.Errorf("CheckStatus() = %v; want nil", err)
		}
		if err := resp.CheckStatus(http.StatusCreated, nil); err == nil {
			t.Errorf("CheckStatus() = nil; want error")
		}

		var got map[string]interface{}
		if err := resp.Unmarshal(http.StatusOK, nil, &got); err != nil {
			t.Errorf("Unmarshal() = %v; want nil", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Body = %v; want = %v", got, want)
		}
	}
}

func TestErrorParser(t *testing.T) {
	data := map[string]interface{}{
		"error": "test error",
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	req := &Request{
		Method: "GET",
		URL:    server.URL,
	}
	resp, err := req.Send(context.Background(), http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}

	ep := func(r *Response) string {
		var b struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(r.Body, &b); err != nil {
			return ""
		}
		return b.Error
	}

	want := "http error status: 500; reason: test error"
	if err := resp.CheckStatus(http.StatusOK, ep); err.Error() != want {
		t.Errorf("CheckStatus() = %q; want = %q", err.Error(), want)
	}
	var got map[string]interface{}
	if err := resp.Unmarshal(http.StatusOK, ep, &got); err.Error() != want {
		t.Errorf("CheckStatus() = %q; want = %q", err.Error(), want)
	}
	if got != nil {
		t.Errorf("Body = %v; want = nil", got)
	}
}

func TestInvalidURL(t *testing.T) {
	req := &Request{
		Method: "GET",
		URL:    "http://localhost:250/mock.url",
	}
	_, err := req.Send(context.Background(), http.DefaultClient)
	if err == nil {
		t.Errorf("Send() = nil; want error")
	}
}

func TestUnmarshalError(t *testing.T) {
	data := map[string]interface{}{
		"foo": "bar",
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	req := &Request{
		Method: "GET",
		URL:    server.URL,
	}
	resp, err := req.Send(context.Background(), http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}

	var got func()
	if err := resp.Unmarshal(http.StatusOK, nil, &got); err == nil {
		t.Errorf("Unmarshal() = nil; want error")
	}
}
