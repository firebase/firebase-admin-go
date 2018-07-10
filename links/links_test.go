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

package links

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"golang.org/x/net/context"

	"firebase.google.com/go/internal"
	"google.golang.org/api/option"
)

var (
	client                *Client
	testLinkStatsResponse []byte
	wantedStatResult      = &LinkStats{
		EventStats: []EventStats{
			{
				Platform:  Android,
				Count:     123,
				EventType: Click,
			},
			{
				Platform:  IOS,
				Count:     123,
				EventType: Click,
			},
			{
				Platform:  Desktop,
				Count:     456,
				EventType: Click,
			},
			{
				Platform:  Android,
				Count:     99,
				EventType: AppInstall,
			},
			{
				Platform:  Android,
				Count:     42,
				EventType: AppFirstOpen,
			},
			{
				Platform:  Android,
				Count:     142,
				EventType: AppReOpen,
			},
			{
				Platform:  IOS,
				Count:     124,
				EventType: Redirect,
			},
		},
	}
)

func TestMain(m *testing.M) {
	defaultTestConf := &internal.LinksConfig{
		Opts: []option.ClientOption{
			option.WithTokenSource(&internal.MockTokenSource{
				AccessToken: "test-token",
			}),
		},
	}

	var err error
	client, err = NewClient(context.Background(), defaultTestConf)
	if err != nil {
		log.Fatalln(err)
	}

	testLinkStatsResponse, err = ioutil.ReadFile("../testdata/get_link_stats.json")
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestGetLinks(t *testing.T) {
	var tr *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(testLinkStatsResponse))
	}))
	defer ts.Close()
	client.linksEndpoint = ts.URL

	ls, err := client.LinkStats(context.Background(), "https://mock", StatOptions{LastNDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	wantRequestURI := "/https%3A%2F%2Fmock/linkStats?durationDays=7"
	if tr.RequestURI != wantRequestURI {
		t.Errorf("RequestURI = %q; want = %q", tr.RequestURI, wantRequestURI)
	}
	if !reflect.DeepEqual(ls, wantedStatResult) {
		t.Errorf("LinkStats() = %#v; want = %#v", ls, wantedStatResult)
	}
}

func TestGetLinksStatsServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte("intentional error"))
	}))
	defer ts.Close()
	client.linksEndpoint = ts.URL

	_, err := client.LinkStats(context.Background(), "https://mock", StatOptions{LastNDays: 7})
	we := "http error status: 500; reason: intentional error"
	if err == nil || err.Error() != we {
		t.Fatalf("LinkStats() error = %q; want = %q", err, we)
	}
}
func TestInvalidShortLink(t *testing.T) {
	_, err := client.LinkStats(context.Background(), "asdf", StatOptions{LastNDays: 2})
	we := "short link must start with https://"
	if err == nil || err.Error() != we {
		t.Errorf("LinkStats(<invalid short link>) error = %q; want = %q", err, we)
	}
}

func TestInvalidLastNDays(t *testing.T) {
	_, err := client.LinkStats(context.Background(), "https://mock", StatOptions{LastNDays: -1})
	we := "last n days must be positive"
	if err == nil || err.Error() != we {
		t.Errorf("LinkStats(<invalid LastNDays>) error = %q; want = %q", err, we)
	}
}
