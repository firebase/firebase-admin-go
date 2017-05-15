package auth

import (
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

type mockHTTPResponse struct {
	Response http.Response
	Err      error
}

func (m *mockHTTPResponse) RoundTrip(*http.Request) (*http.Response, error) {
	return &m.Response, m.Err
}

type mockReadCloser struct {
	data    string
	index   int64
	counter int
}

func (r *mockReadCloser) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.index >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.index:])
	r.index += int64(n)
	return
}

func (r *mockReadCloser) Close() error {
	r.counter++
	return nil
}

type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

func TestHTTPKeySource(t *testing.T) {
	data, err := ioutil.ReadFile("../credentials/testdata/public_certs.json")
	if err != nil {
		t.Fatal(err)
	}

	mc := &mockClock{now: time.Unix(0, 0)}
	rc := &mockReadCloser{
		data:    string(data),
		counter: 0,
	}
	ks := &httpKeySource{
		HTTPClient: &http.Client{
			Transport: &mockHTTPResponse{
				Response: http.Response{
					Status:     "200 OK",
					StatusCode: 200,
					Header: http.Header{
						"Cache-Control": {"public, max-age=100"},
					},
					Body: rc,
				},
				Err: nil,
			},
		},
		Clock: mc,
	}

	exp := time.Unix(100, 0)
	for i := 0; i <= 100; i++ {
		keys, err := ks.Keys()
		if err != nil {
			t.Fatal(err)
		}
		if len(keys) != 3 {
			t.Errorf("Keys: %d; want: 3", len(keys))
		} else if rc.counter != 1 {
			t.Errorf("Counter: %d; want: 1", rc.counter)
		} else if ks.ExpiryTime != exp {
			t.Errorf("Expiry: %v; want: %v", ks.ExpiryTime, exp)
		}
		mc.now = mc.now.Add(time.Second)
	}

	mc.now = time.Unix(101, 0)
	keys, err := ks.Keys()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Errorf("Keys: %d; want: 3", len(keys))
	} else if rc.counter != 2 {
		t.Errorf("Counter: %d; want: 2", rc.counter)
	}
}
