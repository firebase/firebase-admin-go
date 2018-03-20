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

// Package auth contains integration tests for the firebase.google.com/go/auth package.
package auth

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

var client *links.Client
var ctx context.Context
var dynamicLinksE2EURL []byte

var e2eWarning = "End to end tests not set up, see CONTRIBUTING.md file."

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping auth integration tests in short mode.")
		os.Exit(0)
	}

	ctx = context.Background()
	app, err := internal.NewTestApp(ctx, nil)
	if err != nil {
		log.Fatalln(err)
	}

	dynamicLinksE2EURL, _ = ioutil.ReadFile(internal.Resource("dynamic_links_e2e_url.txt"))
	client, err = app.DynamicLinks(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestE2EGetLinkStats(t *testing.T) {
	if dynamicLinksE2EURL == nil {
		log.Println(e2eWarning)
		return
	}
	shortLink := strings.Trim(string(dynamicLinksE2EURL), "\n ")
	ls, err := client.LinkStats(ctx, shortLink, links.StatOptions{DurationDays: 4000})
	if err != nil {
		t.Error(err)
	}
	if len(ls.EventStats) == 0 {
		t.Fatalf("expecting results. %s", e2eWarning)
	}
	if ls.EventStats[0].Count == 0 {
		t.Errorf("expecting non zero count %v", ls.EventStats[0])
	}

}

func TestGetLinkStats(t *testing.T) {
	_, err := client.LinkStats(ctx, "https://fake1.app.gpp.gl/fake", links.StatOptions{DurationDays: 3})
	ws := "http error status: 403"
	if err == nil || !strings.Contains(err.Error(), ws) {
		t.Fatalf("accessing data for someone else's short link, err: %q, want substring: %q", err, ws)
	}
}
