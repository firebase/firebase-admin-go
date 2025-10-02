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

/**
 * // Schema
 * type User @table(key: ["id"]) {
 *   id: String!
 *   name: String!
 *   address: String!
 * }
 */
type User struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Name    string `json:"name"`
	// Generated
	EmailsOnFrom []Email `json:"emails_on_from"`
}

/**
 * // Schema
 * type Email @table {
 * 	id: String!
 * 	subject: String!
 * 	date: Date!
 * 	text: String!
 * 	from: User!
 * }
 */
type Email struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
	Text    string `json:"text"`
	From    User   `json:"from"`
}

type GetUserResponse struct {
	User User `json:"user"`
}

type ListUsersResponse struct {
	Users []User `json:"users"`
}

type UserUpsertResponse struct {
	UserUpsert struct {
		ID string `json:"id"`
	} `json:"user_upsert"`
}

type UserUpdateResponse struct {
	UserUpdate struct {
		ID string `json:"id"`
	} `json:"user_update"`
}

type EmailUpsertResponse struct {
	EmailUpsert struct {
		ID string `json:"id"`
	} `json:"email_upsert"`
}

type ListEmailsResponse struct {
	Emails []Email `json:"emails"`
}

type GetUserVariables struct {
	ID struct {
		ID string `json:"id"`
	} `json:"id"`
}

type DeleteResponse struct {
	EmailDeleteMany int `json:"email_deleteMany"`
	UserDeleteMany  int `json:"user_deleteMany"`
}

var (
	fredUser = User{
		ID:      "fred_id",
		Address: "32 Elm St.",
		Name:    "Fred",
	}

	jeffUser = User{
		ID:      "jeff_id",
		Address: "99 Oak St.",
		Name:    "Jeff",
	}

	fredEmail = Email{
		ID:      "email_id",
		Subject: "free bitcoin inside",
		Date:    "1999-12-31",
		Text:    "get pranked! LOL!",
		From:    User{ID: fredUser.ID},
	}

	initialState = struct {
		Users  []User  `json:"users"`
		Emails []Email `json:"emails"`
	}{
		Users:  []User{fredUser, jeffUser},
		Emails: []Email{fredEmail},
	}

	queryListUsers   string = "query ListUsers @auth(level: PUBLIC) { users { id, name, address } }"
	queryListEmails  string = "query ListEmails @auth(level: NO_ACCESS) { emails { id subject text date from { id } } }"
	queryGetUserById string = "query GetUser($id: User_Key!) { user(key: $id) { id name address } }"
	multipleQueries  string = queryListUsers + "\n" + queryListEmails
	upsertFredUser   string = "mutation user { user_upsert(data: {id: \"" + fredUser.ID + "\", address: \"" + fredUser.Address + "\", name: \"" + fredUser.Name + "\"})}"
	upsertJeffUser   string = "mutation user { user_upsert(data: {id: \"" + jeffUser.ID + "\", address: \"" + jeffUser.Address + "\", name: \"" + jeffUser.Name + "\"})}"
	upsertFredEmail  string = "mutation email {" + "email_upsert(data: {" + "id:\"" + fredEmail.ID + "\"," + "subject: \"" + fredEmail.Subject + "\"," + "date: \"" + fredEmail.Date + "\"," + "text: \"" + fredEmail.Text + "\"," + "fromId: \"" + fredEmail.From.ID + "\"" + "})}"
	deleteAll        string = `mutation delete { email_deleteMany(all: true) user_deleteMany(all: true) }`
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

func initializeDatabase(t *testing.T) {
	var resp1 UserUpsertResponse
	err := client.ExecuteGraphql(context.Background(), upsertFredUser, nil, &resp1)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
	if resp1.UserUpsert.ID != fredUser.ID {
		t.Errorf("ExecuteGraphql() User = %#v; want = %#v", resp1.UserUpsert.ID, fredUser.ID)
	}

	var resp2 UserUpsertResponse
	err = client.ExecuteGraphql(context.Background(), upsertJeffUser, nil, &resp2)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}

	var resp3 EmailUpsertResponse
	err = client.ExecuteGraphql(context.Background(), upsertFredEmail, nil, &resp3)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
}

func cleanupDatabase(t *testing.T) {
	var resp1 DeleteResponse
	err := client.ExecuteGraphql(context.Background(), deleteAll, nil, &resp1)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
}

func containsExpectedUser(users []User, expectedUser User) bool {
	for _, user := range users {
		if reflect.DeepEqual(user, expectedUser) {
			return true
		}
	}
	return false
}
func TestExecuteGraphql(t *testing.T) {
	initializeDatabase(t)
	// defer cleanupDatabase(t)

	var resp ListUsersResponse
	err := client.ExecuteGraphql(context.Background(), queryListUsers, nil, &resp)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}

	if len(resp.Users) != len(initialState.Users) {
		t.Errorf("len(resp.Users) = %d; want > %d", len(resp.Users), len(initialState.Users))
	}

	for _, user := range resp.Users {
		if !containsExpectedUser(initialState.Users, user) {
			t.Errorf("User from response was not found in expected initial state: %#v", user)
		}
	}
}

func TestExecuteGraphqlRead(t *testing.T) {
	initializeDatabase(t)
	defer cleanupDatabase(t)

	var resp ListUsersResponse
	err := client.ExecuteGraphqlRead(context.Background(), queryListUsers, nil, &resp)
	if err != nil {
		t.Fatalf("ExecuteGraphqlRead() error = %v", err)
	}

	if resp.Users == nil {
		t.Fatal("response data does not contain 'users' key")
	}
	if len(resp.Users) != len(initialState.Users) {
		t.Errorf("len(resp.Users) = %d; want > %d", len(resp.Users), len(initialState.Users))
	}

	for _, user := range resp.Users {
		if !containsExpectedUser(initialState.Users, user) {
			t.Errorf("User from response was not found in expected initial state: %#v", user)
		}
	}
}

func TestExecuteGraphqlWithVariables(t *testing.T) {
	initializeDatabase(t)
	defer cleanupDatabase(t)

	var resp GetUserResponse
	opts := &dataconnect.GraphqlOptions{
		Variables: GetUserVariables{
			ID: struct {
				ID string `json:"id"`
			}{
				ID: initialState.Users[0].ID,
			},
		},
	}
	err := client.ExecuteGraphql(context.Background(), queryGetUserById, opts, &resp)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}

	if !reflect.DeepEqual(resp.User, initialState.Users[0]) {
		t.Errorf("ExecuteGraphql() User = %#v; want = %#v", resp.User, initialState.Users[0])
	}
}

func TestExecuteGraphqlMutation(t *testing.T) {
	initializeDatabase(t)
	defer cleanupDatabase(t)

	var resp1 UserUpsertResponse
	err := client.ExecuteGraphql(context.Background(), upsertFredUser, nil, &resp1)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
	if resp1.UserUpsert.ID != fredUser.ID {
		t.Errorf("ExecuteGraphql() User = %#v; want = %#v", resp1.UserUpsert.ID, fredUser.ID)
	}

	var resp2 UserUpsertResponse
	err = client.ExecuteGraphql(context.Background(), upsertJeffUser, nil, &resp2)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
	if resp2.UserUpsert.ID != jeffUser.ID {
		t.Errorf("ExecuteGraphql() User = %#v; want = %#v", resp2.UserUpsert.ID, jeffUser.ID)
	}

	var resp3 EmailUpsertResponse
	err = client.ExecuteGraphql(context.Background(), upsertFredEmail, nil, &resp3)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
	if resp3.EmailUpsert.ID == "" {
		t.Errorf("ExecuteGraphql() Email = %#v; Expected non-empty ID string", resp3.EmailUpsert.ID)
	}

	var resp4 DeleteResponse
	err = client.ExecuteGraphql(context.Background(), deleteAll, nil, &resp4)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
	if resp4.UserDeleteMany == 0 {
		t.Errorf("ExecuteGraphql() Expected non-zero users deleted")
	}
	if resp4.EmailDeleteMany == 0 {
		t.Errorf("ExecuteGraphql() Expected non-zero emails deleted")
	}
}

func TestExecuteGraphqlOperationNameWithMultipleQueries(t *testing.T) {
	initializeDatabase(t)
	defer cleanupDatabase(t)

	opts := &dataconnect.GraphqlOptions{
		OperationName: "ListEmails",
	}

	var resp ListEmailsResponse
	err := client.ExecuteGraphql(context.Background(), multipleQueries, opts, &resp)
	if err != nil {
		t.Fatalf("ExecuteGraphql() error = %v", err)
	}
	if !reflect.DeepEqual(resp.Emails, initialState.Emails) {
		t.Errorf("ExecuteGraphql() Emails = %#v; want = %#v", resp.Emails, initialState.Emails)
	}
}

func TestExecuteGraphqlReadMutationError(t *testing.T) {
	initializeDatabase(t)
	defer cleanupDatabase(t)
	var resp UserUpsertResponse
	err := client.ExecuteGraphqlRead(context.Background(), upsertFredUser, nil, &resp)
	if err == nil {
		t.Fatalf("ExecuteGraphqlRead() expected error for read mutation, got nil")
	}
	if !errorutils.IsPermissionDenied(err) {
		t.Fatalf("ExecuteGraphqlRead() expected Permission Denied error for read mutation, got %s", err)
	}
}

func TestExecuteGraphqlQueryErrorWithoutVariables(t *testing.T) {
	initializeDatabase(t)
	defer cleanupDatabase(t)

	var resp GetUserResponse
	err := client.ExecuteGraphql(context.Background(), queryGetUserById, nil, &resp)
	if err == nil {
		t.Fatalf("ExecuteGraphql() expected error for bad query, got nil")
	}
	if !dataconnect.IsQueryError(err) {
		t.Fatalf("ExecuteGraphql() expected query error, got %s", err)
	}
}
