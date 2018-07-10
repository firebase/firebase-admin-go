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

// Package links contains integration tests for the firebase.google.com/go/links package.
package links

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/context"

	"firebase.google.com/go/integration/internal"
	"firebase.google.com/go/links"
)

var (
	client          *links.Client
	dynamicLinksURL string
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping links integration tests in short mode.")
		os.Exit(0)
	}

	ctx := context.Background()
	app, err := internal.NewTestApp(ctx, nil)
	if err != nil {
		log.Fatalln(err)
	}
	// Integration tests need some setting up. Once they're set up, it might take
	// up to 36 hours to get the desired results. Therefore if they haven't been
	// set up we do not want to fail.
	b, err := ioutil.ReadFile(internal.Resource("integration_dynamic_links.txt"))
	if err == nil {
		dynamicLinksURL = strings.TrimSpace(string(b))
	}

	client, err = app.DynamicLinks(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestLinkStats(t *testing.T) {
	if dynamicLinksURL == "" {
		log.Println("Integration tests for dynamic links not set up")
		return
	}
	ls, err := client.LinkStats(context.Background(), dynamicLinksURL, links.StatOptions{
		LastNDays: 4000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ls.EventStats) == 0 {
		t.Fatal("LinkStats() = empty; want = non-empty")
	}
	if ls.EventStats[0].Count == 0 {
		t.Fatal("EventStats[0].Count = 0; want = non zero")
	}
}

func TestLinkStatsInvalidLink(t *testing.T) {
	const shortLink = "https://fake1.app.gpp.gl/fake"
	_, err := client.LinkStats(context.Background(), shortLink, links.StatOptions{
		LastNDays: 3,
	})
	ws := "http error status: 403"
	if err == nil || !strings.Contains(err.Error(), ws) {
		t.Fatalf("LinkStats() err = %v, want = %q", err, ws)
	}
}
