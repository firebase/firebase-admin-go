// Copyright 2017 Google Inc. All Rights Reserved.
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

package auth

import (
	"errors"
	"fmt"
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
	data       string
	index      int64
	closeCount int
}

func newTestHTTPClient(data []byte) (*http.Client, *mockReadCloser) {
	rc := &mockReadCloser{
		data:       string(data),
		closeCount: 0,
	}
	client := &http.Client{
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
	}
	return client, rc
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
	r.closeCount++
	r.index = 0
	return nil
}

func TestHTTPKeySource(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/public_certs.json")
	if err != nil {
		t.Fatal(err)
	}

	ks := newHTTPKeySource("http://mock.url", http.DefaultClient)
	if ks.HTTPClient == nil {
		t.Errorf("HTTPClient = nil; want non-nil")
	}
	hc, rc := newTestHTTPClient(data)
	ks.HTTPClient = hc
	if err := verifyHTTPKeySource(ks, rc); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPKeySourceWithClient(t *testing.T) {
	data, err := ioutil.ReadFile("../testdata/public_certs.json")
	if err != nil {
		t.Fatal(err)
	}

	hc, rc := newTestHTTPClient(data)
	ks := newHTTPKeySource("http://mock.url", hc)
	if ks.HTTPClient != hc {
		t.Errorf("HTTPClient = %v; want %v", ks.HTTPClient, hc)
	}
	if err := verifyHTTPKeySource(ks, rc); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPKeySourceEmptyResponse(t *testing.T) {
	hc, _ := newTestHTTPClient([]byte(""))
	ks := newHTTPKeySource("http://mock.url", hc)
	if keys, err := ks.Keys(); keys != nil || err == nil {
		t.Errorf("Keys() = (%v, %v); want = (nil, error)", keys, err)
	}
}

func TestHTTPKeySourceIncorrectResponse(t *testing.T) {
	hc, _ := newTestHTTPClient([]byte("{\"foo\": 1}"))
	ks := newHTTPKeySource("http://mock.url", hc)
	if keys, err := ks.Keys(); keys != nil || err == nil {
		t.Errorf("Keys() = (%v, %v); want = (nil, error)", keys, err)
	}
}

func TestHTTPKeySourceTransportError(t *testing.T) {
	hc := &http.Client{
		Transport: &mockHTTPResponse{
			Err: errors.New("transport error"),
		},
	}
	ks := newHTTPKeySource("http://mock.url", hc)
	if keys, err := ks.Keys(); keys != nil || err == nil {
		t.Errorf("Keys() = (%v, %v); want = (nil, error)", keys, err)
	}
}

func TestFindMaxAge(t *testing.T) {
	cases := []struct {
		cc   string
		want int64
	}{
		{"max-age=100", 100},
		{"public, max-age=100", 100},
		{"public,max-age=100", 100},
	}
	for _, tc := range cases {
		resp := &http.Response{
			Header: http.Header{"Cache-Control": {tc.cc}},
		}
		age, err := findMaxAge(resp)
		if err != nil {
			t.Errorf("findMaxAge(%q) = %v", tc.cc, err)
		} else if *age != (time.Duration(tc.want) * time.Second) {
			t.Errorf("findMaxAge(%q) = %v; want %v", tc.cc, *age, tc.want)
		}
	}
}

func TestFindMaxAgeError(t *testing.T) {
	cases := []string{
		"",
		"max-age 100",
		"max-age: 100",
		"max-age2=100",
		"max-age=foo",
	}
	for _, tc := range cases {
		resp := &http.Response{
			Header: http.Header{"Cache-Control": []string{tc}},
		}
		if age, err := findMaxAge(resp); age != nil || err == nil {
			t.Errorf("findMaxAge(%q) = (%v, %v); want = (nil, err)", tc, age, err)
		}
	}
}

func TestParsePublicKeys(t *testing.T) {
	b, err := ioutil.ReadFile("../testdata/public_certs.json")
	if err != nil {
		t.Fatal(err)
	}
	keys, err := parsePublicKeys(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Errorf("parsePublicKeys() = %d; want: %d", len(keys), 3)
	}
}

func TestParsePublicKeysError(t *testing.T) {
	cases := []string{
		"",
		"not-json",
	}
	for _, tc := range cases {
		if keys, err := parsePublicKeys([]byte(tc)); keys != nil || err == nil {
			t.Errorf("parsePublicKeys(%q) = (%v, %v); want: (nil, err)", tc, keys, err)
		}
	}
}

func TestDefaultServiceAcctSigner(t *testing.T) {
	signer := &serviceAcctSigner{}
	if email, err := signer.Email(); email != "" || err == nil {
		t.Errorf("Email() = (%v, %v); want = ('', error)", email, err)
	}
	if sig, err := signer.Sign([]byte("")); sig != nil || err == nil {
		t.Errorf("Sign() = (%v, %v); want = ('', error)", sig, err)
	}
}

func verifyHTTPKeySource(ks *httpKeySource, rc *mockReadCloser) error {
	mc := &mockClock{now: time.Unix(0, 0)}
	ks.Clock = mc

	exp := time.Unix(100, 0)
	for i := 0; i <= 100; i++ {
		keys, err := ks.Keys()
		if err != nil {
			return err
		}
		if len(keys) != 3 {
			return fmt.Errorf("Keys: %d; want: 3", len(keys))
		} else if rc.closeCount != 1 {
			return fmt.Errorf("HTTP calls: %d; want: 1", rc.closeCount)
		} else if ks.ExpiryTime != exp {
			return fmt.Errorf("Expiry: %v; want: %v", ks.ExpiryTime, exp)
		}
		mc.now = mc.now.Add(time.Second)
	}

	mc.now = time.Unix(101, 0)
	keys, err := ks.Keys()
	if err != nil {
		return err
	}
	if len(keys) != 3 {
		return fmt.Errorf("Keys: %d; want: 3", len(keys))
	} else if rc.closeCount != 2 {
		return fmt.Errorf("HTTP calls: %d; want: 2", rc.closeCount)
	}
	return nil
}
