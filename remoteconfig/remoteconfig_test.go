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

package remoteconfig

import (
	"context"
	"testing"

	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
)

var (
	client *Client

	testOpts = []option.ClientOption{
		option.WithTokenSource(&internal.MockTokenSource{AccessToken: "mock-token"}),
	}
)

// Test NewClient with valid config
func TestNewClientSuccess(t *testing.T) {
	ctx := context.Background()
	config := &internal.RemoteConfigClientConfig{
		ProjectID: "test-project",
		Opts:      testOpts,
		Version:   "1.2.3",
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if client == nil {
		t.Error("NewClient returned nil client")
	}
}

// Test NewClient with missing Project ID
func TestNewClientMissingProjectID(t *testing.T) {
	ctx := context.Background()
	config := &internal.RemoteConfigClientConfig{}
	_, err := NewClient(ctx, config)
	if err == nil {
		t.Fatal("NewClient should have failed with missing project ID")
	}
}
