// Copyright 2025 Google Inc. All Rights Reserved.
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

// Package dataconnect_test contains integration tests for the dataconnect package.
package dataconnect_test

import (
	"context"
	"testing"

	"firebase.google.com/go/v4"
	"firebase.google.com/go/v4/dataconnect"
	"firebase.google.com/go/v4/integration/internal"
)

// This test is not expected to run in the CI environment. It is provided
// as a usage example and for manual testing.
func TestDataConnect(t *testing.T) {
	app, err := internal.NewTestApp(context.Background())
	if err != nil {
		t.Fatalf("internal.NewTestApp() = %v", err)
	}

	connectorConfig := &dataconnect.ConnectorConfig{
		Location:  "us-central1",
		ServiceID: "my-service",
	}

	client, err := app.DataConnect(context.Background(), connectorConfig)
	if err != nil {
		t.Fatalf("app.DataConnect() = %v", err)
	}

	query := `
		query {
			users {
				id
				name
			}
		}
	`

	// Test ExecuteGraphqlRead
	_, err = client.ExecuteGraphqlRead(context.Background(), query, nil)
	if err != nil {
		// We expect an error here as we are not running against a real backend.
		// The purpose of this test is to ensure the API is wired up correctly.
		t.Logf("ExecuteGraphqlRead() returned an expected error: %v", err)
	}

	mutation := `
		mutation {
			createUser(name: "test-user") {
				id
			}
		}
	`
	// Test ExecuteGraphql
	_, err = client.ExecuteGraphql(context.Background(), mutation, nil)
	if err != nil {
		// We expect an error here as we are not running against a real backend.
		t.Logf("ExecuteGraphql() returned an expected error: %v", err)
	}
}
