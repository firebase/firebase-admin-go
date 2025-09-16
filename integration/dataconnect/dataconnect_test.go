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

// Package dataconnect_test contains integration tests for the Data Connect service.
package dataconnect_test

import (
	"context"
	"testing"

	"firebase.google.com/go/v4"
	"firebase.google.com/go/v4/dataconnect"
)

func TestDataConnect(t *testing.T) {
	// This test is a placeholder and is not expected to run in a CI environment.
	// It requires a valid Firebase project with Data Connect enabled and configured.
	// To run, set up application default credentials and update the placeholder values.
	t.Skip("Skipping Data Connect integration test because it requires a configured project.")

	app, err := firebase.NewApp(context.Background(), nil)
	if err != nil {
		t.Fatalf("firebase.NewApp() = %v; want nil", err)
	}

	connectorConfig := &dataconnect.ConnectorConfig{
		Location:  "us-central1", // Replace with your location
		ServiceID: "my-service",  // Replace with your service ID
	}

	client, err := app.DataConnect(context.Background(), connectorConfig)
	if err != nil {
		t.Fatalf("app.DataConnect() = %v; want nil", err)
	}

	// This query is hypothetical and depends on the user's schema.
	query := `query { users { id, name } }`
	resp, err := client.ExecuteGraphql(context.Background(), query, nil)
	if err != nil {
		// If the error is a query-error, it means the connection was successful,
		// but the query failed, which is a "successful" integration test for our purposes.
		if dataconnect.IsQueryError(err) {
			t.Logf("Received expected query error: %v", err)
			return
		}
		t.Fatalf("client.ExecuteGraphql() = %v; want nil or query-error", err)
	}

	if resp == nil || resp.Data == nil {
		t.Errorf("ExecuteGraphql() response or data is nil")
	}
	t.Logf("Received response: %#v", resp.Data)
}
