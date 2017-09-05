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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/api/option"
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

func newHTTPClient(data []byte) (*http.Client, *mockReadCloser) {
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

	ks, err := newHTTPKeySource(context.Background(), "http://mock.url")
	if err != nil {
		t.Fatal(err)
	}

	if ks.HTTPClient == nil {
		t.Errorf("HTTPClient = nil; want non-nil")
	}
	hc, rc := newHTTPClient(data)
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

	hc, rc := newHTTPClient(data)
	ks, err := newHTTPKeySource(context.Background(), "http://mock.url", option.WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}

	if ks.HTTPClient != hc {
		t.Errorf("HTTPClient = %v; want %v", ks.HTTPClient, hc)
	}
	if err := verifyHTTPKeySource(ks, rc); err != nil {
		t.Fatal(err)
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
