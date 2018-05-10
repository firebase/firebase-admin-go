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
	"encoding/base64"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/api/iterator"

	"firebase.google.com/go/auth"
	"firebase.google.com/go/auth/hash"
)

var testFixtures = struct {
	uidList            []string
	sampleUserBlank    *auth.UserRecord
	sampleUserWithData *auth.UserRecord
}{}

func TestUserManagement(t *testing.T) {
	orderedRuns := []struct {
		name     string
		testFunc func(*testing.T)
	}{
		{"Create test users", testCreateUsers},
		{"Get user", testGetUser},
		{"Get user by phone", testGetUserByPhoneNumber},
		{"Get user by email", testGetUserByEmail},
		{"Iterate users", testUserIterator},
		{"Paged iteration", testPager},
		{"Disable user account", testDisableUser},
		{"Update user", testUpdateUser},
		{"Remove user attributes", testRemovePhonePhotoName},
		{"Remove custom claims", testRemoveCustomClaims},
		{"Import users", testImportUsers},
		{"Import users with password", testImportUsersWithPassword},
		{"Add custom claims", testAddCustomClaims},
		{"Delete test users", testDeleteUsers},
	}
	// The tests are meant to be run in sequence. A failure in creating the users
	// should be fatal so none of the other tests run.
	for _, run := range orderedRuns {
		t.Log(run.name)
		run.testFunc(t)
		if t.Failed() {
			t.Errorf("Failed run %v", run.name)
		}
	}
}

// N.B if the tests are failing due to inability to create existing users, manual
// cleanup of the previus test run might be required, delete the unwanted users via:
// https://console.firebase.google.com/u/0/project/<project-id>/authentication/users
func testCreateUsers(t *testing.T) {
	// Create users with uid
	for i := 0; i < 3; i++ {
		uid := fmt.Sprintf("tempTestUserID-%d", i)
		params := (&auth.UserToCreate{}).UID(uid)
		u, err := client.CreateUser(context.Background(), params)
		if err != nil {
			t.Fatal(err)
		}
		testFixtures.uidList = append(testFixtures.uidList, u.UID)
		// make sure that the user.TokensValidAfterMillis is not in the future or stale.
		if u.TokensValidAfterMillis > time.Now().Unix()*1000 {
			t.Errorf("timestamp cannot be in the future")
		}
		if time.Now().Sub(time.Unix(u.TokensValidAfterMillis, 0)) > time.Hour {
			t.Errorf("timestamp should be recent")
		}

	}
	// Create user with no parameters (zero-value)
	u, err := client.CreateUser(context.Background(), (&auth.UserToCreate{}))
	if err != nil {
		t.Fatal(err)
	}
	testFixtures.sampleUserBlank = u
	testFixtures.uidList = append(testFixtures.uidList, u.UID)

	// Create user with parameters
	uid := "tempUserId1234"
	params := (&auth.UserToCreate{}).
		UID(uid).
		Email(uid + "email@test.com").
		DisplayName("display_name").
		Password("password").
		PhoneNumber("+12223334444")

	if u, err = client.CreateUser(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	testFixtures.sampleUserWithData = u
	testFixtures.uidList = append(testFixtures.uidList, u.UID)
}

func testGetUser(t *testing.T) {
	want := testFixtures.sampleUserWithData

	u, err := client.GetUser(context.Background(), want.UID)
	if err != nil {
		t.Fatalf("error getting user %s", err)
	}
	if !reflect.DeepEqual(u, want) {
		t.Errorf("GetUser(UID) = %#v; want = %#v", u, want)
	}
}

func testGetUserByPhoneNumber(t *testing.T) {
	want := testFixtures.sampleUserWithData
	u, err := client.GetUserByPhoneNumber(context.Background(), want.PhoneNumber)
	if err != nil {
		t.Fatalf("error getting user %s", err)
	}
	if !reflect.DeepEqual(u, want) {
		t.Errorf("GetUserByPhoneNumber(%q) = %#v; want = %#v", want.PhoneNumber, u, want)
	}
}

func testGetUserByEmail(t *testing.T) {
	want := testFixtures.sampleUserWithData
	u, err := client.GetUserByEmail(context.Background(), want.Email)
	if err != nil {
		t.Fatalf("error getting user %s", err)
	}
	if !reflect.DeepEqual(u, want) {
		t.Errorf("GetUserByEmail(%q) = %#v; want = %#v", want.Email, u, want)
	}
}

func testUserIterator(t *testing.T) {
	iter := client.Users(context.Background(), "")
	uids := map[string]bool{}
	count := 0

	for {
		u, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		count++
		uids[u.UID] = true
	}
	if count < 5 {
		t.Errorf("Users() count = %d; want >= 5", count)
	}
	// verify that all the expected users are present
	for _, uid := range testFixtures.uidList {
		if _, ok := uids[uid]; !ok {
			t.Errorf("Users() missing uid: %s", uid)
		}
	}
}

func testPager(t *testing.T) {
	iter := client.Users(context.Background(), "")
	pager := iterator.NewPager(iter, 2, "")
	userCount := 0
	pageCount := 0

	for {
		pageCount++
		var users []*auth.ExportedUserRecord
		nextPageToken, err := pager.NextPage(&users)
		if err != nil {
			t.Fatalf("paging error %v", err)
		}
		userCount += len(users)
		for _, u := range users {
			// this iterates over users in a page
			if u.UID == "something" {
				// do something
			}
		}
		if nextPageToken == "" {
			break
		}
	}
	if userCount < 5 {
		t.Errorf("Users() count = %d;  want >= 5", userCount)
	}
	if pageCount < 3 {
		t.Errorf("NewPager() count = %d;  want >= 3", pageCount)
	}
}

func testDisableUser(t *testing.T) {
	want := testFixtures.sampleUserBlank
	u, err := client.GetUser(context.Background(), want.UID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Disabled {
		t.Errorf("GetUser().Disabled = true; want = false")
	}

	params := (&auth.UserToUpdate{}).Disabled(true)
	u, err = client.UpdateUser(context.Background(), u.UID, params)
	if err != nil {
		t.Fatal(err)
	}
	if !u.Disabled {
		t.Errorf("UpdateUser(disable).Disabled = false; want = true")
	}

	params = (&auth.UserToUpdate{}).Disabled(false)
	u, err = client.UpdateUser(context.Background(), u.UID, params)
	if err != nil {
		t.Fatal(err)
	}
	if u.Disabled {
		t.Errorf("UpdateUser(disable).Disabled = true; want = false")
	}
}

func testUpdateUser(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if u == nil || err != nil {
		t.Fatalf("GetUser() = (%v, %v); want = (user, nil)", u, err)
	}

	want := &auth.UserRecord{
		UserInfo: &auth.UserInfo{
			UID:        testFixtures.sampleUserBlank.UID,
			ProviderID: "firebase",
		},
		TokensValidAfterMillis: u.TokensValidAfterMillis,
		UserMetadata: &auth.UserMetadata{
			CreationTimestamp: testFixtures.sampleUserBlank.UserMetadata.CreationTimestamp,
		},
	}
	if !reflect.DeepEqual(u, want) {
		t.Errorf("GetUser() = %#v; want = %#v", u, want)
	}

	params := (&auth.UserToUpdate{}).
		Disabled(false).
		DisplayName("name").
		PhoneNumber("+12345678901").
		PhotoURL("http://photo.png").
		Email("abc@ab.ab").
		EmailVerified(true).
		Password("wordpass").
		CustomClaims(map[string]interface{}{"custom": "claims"})
	u, err = client.UpdateUser(context.Background(), u.UID, params)
	if err != nil {
		t.Fatal(err)
	}

	want = &auth.UserRecord{
		UserInfo: &auth.UserInfo{
			UID:         testFixtures.sampleUserBlank.UID,
			DisplayName: "name",
			PhoneNumber: "+12345678901",
			PhotoURL:    "http://photo.png",
			ProviderID:  "firebase",
			Email:       "abc@ab.ab",
		},
		TokensValidAfterMillis: u.TokensValidAfterMillis,
		UserMetadata: &auth.UserMetadata{
			CreationTimestamp: testFixtures.sampleUserBlank.UserMetadata.CreationTimestamp,
		},
		Disabled:      false,
		EmailVerified: true,
		CustomClaims:  map[string]interface{}{"custom": "claims"},
	}

	testProviderInfo := func(pi []*auth.UserInfo, t *testing.T) {
		passwordUI := &auth.UserInfo{
			DisplayName: "name",
			Email:       "abc@ab.ab",
			PhotoURL:    "http://photo.png",
			ProviderID:  "password",
			UID:         "abc@ab.ab",
		}
		phoneUI := &auth.UserInfo{
			PhoneNumber: "+12345678901",
			ProviderID:  "phone",
			UID:         "+12345678901",
		}

		var compareWith *auth.UserInfo
		for _, ui := range pi {
			switch ui.ProviderID {
			case "password":
				compareWith = passwordUI
			case "phone":
				compareWith = phoneUI
			}
			if !reflect.DeepEqual(ui, compareWith) {
				t.Errorf("UpdateUser()got: %#v; \nwant: %#v", ui, compareWith)
			}
		}
	}

	// compare provider info separately since the order of the providers isn't guaranteed.
	testProviderInfo(u.ProviderUserInfo, t)

	// now compare the rest of the record, without the ProviderInfo
	u.ProviderUserInfo = nil
	if !reflect.DeepEqual(u, want) {
		t.Errorf("UpdateUser() = %#v; want = %#v", u, want)
	}
}

func testRemovePhonePhotoName(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Fatal(err)
	}
	if u.PhoneNumber == "" {
		t.Errorf("GetUser().PhoneNumber = empty; want = non-empty")
	}
	if len(u.ProviderUserInfo) != 2 {
		t.Errorf("GetUser().ProviderUserInfo = %d; want = 2", len(u.ProviderUserInfo))
	}
	if u.PhotoURL == "" {
		t.Errorf("GetUser().PhotoURL = empty; want = non-empty")
	}
	if u.DisplayName == "" {
		t.Errorf("GetUser().DisplayName = empty; want = non-empty")
	}

	params := (&auth.UserToUpdate{}).PhoneNumber("").PhotoURL("").DisplayName("")
	u, err = client.UpdateUser(context.Background(), u.UID, params)
	if err != nil {
		t.Fatal(err)
	}
	if u.PhoneNumber != "" {
		t.Errorf("UpdateUser().PhoneNumber = %q; want: %q", u.PhoneNumber, "")
	}
	if len(u.ProviderUserInfo) != 1 {
		t.Errorf("UpdateUser().ProviderUserInfo = %d, want = 1", len(u.ProviderUserInfo))
	}
	if u.DisplayName != "" {
		t.Errorf("UpdateUser().DisplayName = %q; want =%q", u.DisplayName, "")
	}
	if u.PhotoURL != "" {
		t.Errorf("UpdateUser().PhotoURL = %q; want = %q", u.PhotoURL, "")
	}
}

func testRemoveCustomClaims(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"custom": "claims"}
	if !reflect.DeepEqual(u.CustomClaims, want) {
		t.Errorf("CustomClaims = %v; want = %v", u.CustomClaims, want)
	}

	err = client.SetCustomUserClaims(context.Background(), u.UID, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Fatal(err)
	}
	if u.CustomClaims != nil {
		t.Errorf("CustomClaims() = %#v; want = nil", u.CustomClaims)
	}
}

func testAddCustomClaims(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Fatal(err)
	}
	if u.CustomClaims != nil {
		t.Errorf("GetUser().CustomClaims = %v; want = nil", u.CustomClaims)
	}

	want := map[string]interface{}{"2custom": "2claims"}
	params := (&auth.UserToUpdate{}).CustomClaims(want)
	u, err = client.UpdateUser(context.Background(), u.UID, params)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(u.CustomClaims, want) {
		t.Errorf("CustomClaims = %v; want = %v", u.CustomClaims, want)
	}
}

func testDeleteUsers(t *testing.T) {
	for _, id := range testFixtures.uidList {
		if err := client.DeleteUser(context.Background(), id); err != nil {
			t.Errorf("DeleteUser(%q) = %v; want = nil", id, err)
		}

		u, err := client.GetUser(context.Background(), id)
		if u != nil || err == nil {
			t.Errorf("GetUser(non-existing) = (%v, %v); want = (nil, error)", u, err)
		}
	}
}

func testImportUsers(t *testing.T) {
	randomUID := randomString(24)
	randomEmail := strings.ToLower("test" + randomUID[:12] + "@example." + randomUID[12:] + ".com")
	user := (&auth.UserToImport{}).UID(randomUID).Email(randomEmail)
	result, err := client.ImportUsers(context.Background(), []*auth.UserToImport{user})
	if err != nil {
		t.Fatal(err)
	}
	if result.SuccessCount != 1 || result.FailureCount != 0 {
		t.Errorf("ImportUsers() = %#v; want = {SuccessCount: 1, FailureCount: 0}", result)
	}
	testFixtures.uidList = append(testFixtures.uidList, randomUID)

	savedUser, err := client.GetUser(context.Background(), randomUID)
	if err != nil {
		t.Fatal(err)
	}
	if savedUser.Email != randomEmail {
		t.Errorf("GetUser() = %q; want = %q", savedUser.Email, randomEmail)
	}
}

func testImportUsersWithPassword(t *testing.T) {
	const (
		rawScryptKey    = "jxspr8Ki0RYycVU8zykbdLGjFQ3McFUH0uiiTvC8pVMXAn210wjLNmdZJzxUECKbm0QsEmYUSDzZvpjeJ9WmXA=="
		rawPasswordHash = "V358E8LdWJXAO7muq0CufVpEOXaj8aFiC7T/rcaGieN04q/ZPJ08WhJEHGjj9lz/2TT+/86N5VjVoc5DdBhBiw=="
		rawSeparator    = "Bw=="
	)
	scryptKey, err := base64.StdEncoding.DecodeString(rawScryptKey)
	if err != nil {
		t.Fatal(err)
	}
	saltSeparator, err := base64.StdEncoding.DecodeString(rawSeparator)
	if err != nil {
		t.Fatal(err)
	}
	scrypt := hash.Scrypt{
		Key:           scryptKey,
		SaltSeparator: saltSeparator,
		Rounds:        8,
		MemoryCost:    14,
	}

	randomUID := randomString(24)
	randomEmail := strings.ToLower("test" + randomUID[:12] + "@example." + randomUID[12:] + ".com")
	passwordHash, err := base64.StdEncoding.DecodeString(rawPasswordHash)
	if err != nil {
		t.Fatal(err)
	}
	user := (&auth.UserToImport{}).
		UID(randomUID).
		Email(randomEmail).
		PasswordHash(passwordHash).
		PasswordSalt([]byte("NaCl"))
	result, err := client.ImportUsers(context.Background(), []*auth.UserToImport{user}, auth.WithHash(scrypt))
	if err != nil {
		t.Fatal(err)
	}
	if result.SuccessCount != 1 || result.FailureCount != 0 {
		t.Errorf("ImportUsers() = %#v; want = {SuccessCount: 1, FailureCount: 0}", result)
	}
	testFixtures.uidList = append(testFixtures.uidList, randomUID)

	savedUser, err := client.GetUser(context.Background(), randomUID)
	if err != nil {
		t.Fatal(err)
	}
	if savedUser.Email != randomEmail {
		t.Errorf("GetUser() = %q; want = %q", savedUser.Email, randomEmail)
	}
	idToken, err := signInWithPassword(randomEmail, "password")
	if err != nil {
		t.Fatal(err)
	}
	if idToken == "" {
		t.Errorf("ID Token = empty; want = non-empty")
	}
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
