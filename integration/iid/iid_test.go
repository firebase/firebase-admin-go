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

// Package iid contains integration tests for the firebase.google.com/go/iid package.
package iid

import (
	"flag"
	"log"
	"os"
	"testing"

	"firebase.google.com/go/iid"
	"firebase.google.com/go/integration/internal"

	"golang.org/x/net/context"
)

var client *iid.Client

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping instance ID integration tests in short mode.")
		os.Exit(0)
	}

	ctx := context.Background()
	app, err := internal.NewTestApp(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	client, err = app.InstanceID(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	os.Exit(m.Run())
}

func TestNonExisting(t *testing.T) {
	err := client.DeleteInstanceID(context.Background(), "dnon-existY")
	if err == nil {
		t.Errorf("DeleteInstanceID(\"dnon-existY\") = nil; want error")
	}
	want := `instance id "dnon-existY": failed to find the instance id`
	if err.Error() != want {
		t.Errorf("DeleteInstanceID(\"dnon-existY\") = %v; want = %v", err, want)
	}
}
