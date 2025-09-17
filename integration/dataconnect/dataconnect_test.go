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
package dataconnect

import (
	"context"
	"flag"
	"log"
	"os"
	"reflect"
	"testing"

	"firebase.google.com/go/v4/dataconnect"
	"firebase.google.com/go/v4/errorutils"
	"firebase.google.com/go/v4/integration/internal"
)

var client *dataconnect.Client

var connectorConfig = &dataconnect.ConnectorConfig{
	Location:  "us-west2",
	ServiceID: "my-service",
}

const (
	userID string = "QVBJcy5ndXJ3"

	queryListUsers   string = "query ListUsers @auth(level: PUBLIC) { users { uid, name, address } }"
	queryListEmails  string = "query ListEmails @auth(level: NO_ACCESS) { emails { id subject text date from { name } } }"
	queryGetUserByID string = "query GetUser($id: User_Key!) { user(key: $id) { uid name } }"
	mutation         string = "mutation user { user_insert(data: {uid: \"" + userID + "\", address: \"32 St\", name: \"Fred Car\"}) }"
	upsertUser       string = "mutation UpsertUser($id: String) { user_upsert(data: { uid: $id, address: \"32 St.\", name: \"Fred\" }) }"
	multipleQueries  string = queryListUsers + "\n" + queryListEmails
)

var (
	testUser = map[string]interface{}{
		"name":    "Fred",
		"address": "32 St.",
		"uid":     userID,
	}

	expectedUsers = []map[string]interface{}{
		testUser,
		{
			"name":    "Jeff",
			"address": "99 Oak St. N",
			"uid":     "QVBJcy5ndXJ1",
		},
	}
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("Skipping dataconnect integration tests in short mode.")
		return
	}

	ctx := context.Background()
	var err error
	app, err := internal.NewTestApp(ctx, nil)
	if err != nil {
		log.Fatalln(err)
	}

	client, err = app.DataConnect(context.Background(), connectorConfig)
	if err != nil {
		log.Fatalf("app.DataConnect() = %v", err)
	}
	os.Exit(m.Run())
}

// type User struct {
// 	UID     string `json:"uid"`
// 	Name    string `json:"name"`
// 	Address string `json:"address"`

// 	// Generated
// 	EmailsOnFrom []Email `json:"emails_on_from"`
// }

// type Email struct {
// 	Subject string `json:"subject"`
// 	Date    string `json:"date"`
// 	Text    string `json:"text"`
// 	From    string `json:"from"`

// 	// Generated
// 	ID string `json:"id"`
// }

func containsExpectedUser(usersSlice []interface{}, expectedUser map[string]interface{}) bool {
	for _, item := range usersSlice {
		userMap, ok := item.(map[string]interface{})
		if !ok {
			// Item in slice is not the expected type, so it can't be a match
			continue
		}
		if reflect.DeepEqual(userMap, expectedUser) {
			return true
		}
	}
	return false
}

func TestExecuteGraphqlRead(t *testing.T) {
	resp, err := client.ExecuteGraphqlRead(context.Background(), queryListUsers, nil)
	if err != nil {
		t.Fatalf("ExecuteGraphqlRead() error = %v", err)
	}

	if resp.Data == nil {
		t.Errorf("resp.Data is empty")
	}
	users, ok := resp.Data["users"]
	if !ok {
		t.Fatal("response data does not contain 'users' key")
	}
	usersSlice, ok := users.([]interface{})
	if !ok {
		t.Fatal("'users' field is not a slice")
	}
	if len(usersSlice) <= 1 {
		t.Errorf("len(resp.Data[\"users\"]) = %d; want > 1", len(usersSlice))
	}

	for _, expectedUser := range expectedUsers {
		if !containsExpectedUser(usersSlice, expectedUser) {
			t.Errorf("ExecuteGraphqlRead() response data does not contain expected user: %#v", expectedUser)
		}
	}
}

func TestExecuteGraphqlReadMutation(t *testing.T) {
	_, err := client.ExecuteGraphqlRead(context.Background(), mutation, nil)
	if err == nil {
		t.Fatalf("ExecuteGraphqlRead() expected error for read mutation, got nil")
	}
	if !errorutils.IsPermissionDenied(err) {
		t.Fatalf("ExecuteGraphqlRead() expected Permission Denied error for read mutation, got %s", err)
	}
}

func TestExecuteGraphqlQueryError(t *testing.T) {
	_, err := client.ExecuteGraphql(context.Background(), mutation, nil)
	if err == nil {
		t.Fatalf("ExecuteGraphql() expected error for bad query, got nil")
	}
	if !dataconnect.IsQueryError(err) {
		t.Fatalf("ExecuteGraphql() expected query error, got %s", err)
	}
}

func TestExecuteGraphqlMutation(t *testing.T) {
	opts := &dataconnect.GraphqlOptions{
		Variables: map[string]interface{}{
			"id": userID,
		},
	}
	resp, err := client.ExecuteGraphql(context.Background(), upsertUser, opts)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
	want := &dataconnect.ExecuteGraphqlResponse{
		Data: map[string]interface{}{
			"user_upsert": map[string]interface{}{
				"uid": userID,
			},
		},
	}
	if resp.Data == nil {
		t.Errorf("resp.Data is empty")
	}
	if !reflect.DeepEqual(resp, want) {
		t.Errorf("ExecuteGraphql() response = %#v; want = %#v", resp, want)
	}
}

func TestExecuteGraphqlListUsers(t *testing.T) {
	resp, err := client.ExecuteGraphql(context.Background(), queryListUsers, nil)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
	if resp.Data == nil {
		t.Errorf("resp.Data is empty")
	}
	users, ok := resp.Data["users"]
	if !ok {
		t.Fatal("response data does not contain 'users' key")
	}
	usersSlice, ok := users.([]interface{})
	if !ok {
		t.Fatal("'users' field is not a slice")
	}
	if len(usersSlice) <= 1 {
		t.Errorf("len(resp.Data[\"users\"]) = %d; want > 1", len(usersSlice))
	}

	for _, expectedUser := range expectedUsers {
		if !containsExpectedUser(usersSlice, expectedUser) {
			t.Errorf("ExecuteGraphql() response data does not contain expected user: %#v", expectedUser)
		}
	}
}

func TestExecuteGraphqlWithVariables(t *testing.T) {
	opts := &dataconnect.GraphqlOptions{
		Variables: map[string]interface{}{
			"id": map[string]interface{}{
				"uid": userID,
			},
		},
	}
	resp, err := client.ExecuteGraphql(context.Background(), queryGetUserByID, opts)
	if err != nil {
		t.Fatalf("ExecuteGraphql() with variables error = %v", err)
	}

	want := &dataconnect.ExecuteGraphqlResponse{
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"uid":  testUser["uid"],
				"name": testUser["name"],
			},
		},
	}

	if resp.Data == nil {
		t.Errorf("resp.Data is empty")
	}
	if !reflect.DeepEqual(resp, want) {
		t.Errorf("ExecuteGraphql() response = %#v; want = %#v", resp, want)
	}
}

func TestExecuteGraphqlWithOperationName(t *testing.T) {
	opts := &dataconnect.GraphqlOptions{
		OperationName: "ListEmails",
	}
	resp, err := client.ExecuteGraphql(context.Background(), multipleQueries, opts)
	if err != nil {
		t.Fatalf("ExecuteGraphql() with operationName error = %v", err)
	}

	if resp.Data == nil {
		t.Errorf("resp.Data is empty")
	}

	emails, ok := resp.Data["emails"]
	if !ok {
		t.Fatal("response data does not contain 'emails' key")
	}
	emailsSlice, ok := emails.([]interface{})
	if !ok {
		t.Fatal("'emails' field is not a slice")
	}
	if len(emailsSlice) != 1 {
		t.Fatalf("len(emails) = %d; want 1", len(emailsSlice))
	}
	email, ok := emailsSlice[0].(map[string]interface{})
	if !ok {
		t.Fatal("email item is not a map")
	}

	if email["id"] == nil {
		t.Error("email.id is nil, expected not undefined")
	}
	from, ok := email["from"].(map[string]interface{})
	if !ok {
		t.Fatal("email.from is not a map")
	}
	if from["name"] != "Jeff" {
		t.Errorf("email.from.name = %q; want \"Jeff\"", from["name"])
	}
}
