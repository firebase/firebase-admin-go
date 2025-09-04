// Copyright 2024 Google Inc. All Rights Reserved.
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

// Package dataconnect contains integration tests for the firebase.google.com/go/dataconnect package.
package dataconnect

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/dataconnect"
	"firebase.google.com/go/v4/integration/internal"
)

var app *firebase.App

// This test assumes that the DATA_CONNECT_EMULATOR_HOST environment variable is set.
// If it is not set, it will attempt to connect to the live Data Connect service,
// which requires a valid project configuration and credentials.
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping dataconnect integration tests in short mode.")
		os.Exit(0)
	}

	var err error
	app, err = internal.NewTestApp(context.Background(), nil)
	if err != nil {
		log.Fatalln(err)
	}

	os.Exit(m.Run())
}

func TestExecuteGraphQL(t *testing.T) {
	var client *dataconnect.Client
	var err error
	client, err = app.DataConnect(context.Background(), &firebase.ConnectorConfig{
		Location:  "us-central1",
		ServiceId: "gke-service",
	})
	if err != nil {
		t.Fatalf("DataConnect() = %v", err)
	}

	query := "query { __typename }"
	resp, err := client.ExecuteGraphQL(context.Background(), query, nil)
	if err != nil {
		t.Fatalf("ExecuteGraphQL() = %v", err)
	}

	if resp.Data == nil {
		t.Errorf("ExecuteGraphQL() response data = nil; want non-nil")
	}
}

func TestExecuteGraphQLRead(t *testing.T) {
	var client *dataconnect.Client
	var err error
	client, err = app.DataConnect(context.Background(), &firebase.ConnectorConfig{
		Location:  "us-central1",
		ServiceId: "gke-service",
	})
	if err != nil {
		t.Fatalf("DataConnect() = %v", err)
	}

	query := "query { __typename }"
	resp, err := client.ExecuteGraphQLRead(context.Background(), query, nil)
	if err != nil {
		t.Fatalf("ExecuteGraphQLRead() = %v", err)
	}

	if resp.Data == nil {
		t.Errorf("ExecuteGraphQLRead() response data = nil; want non-nil")
	}
}
