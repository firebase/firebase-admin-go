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
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/api/iterator"

	"firebase.google.com/go/auth"

	"golang.org/x/net/context"
)

var testFixtures = struct {
	uidList            []string
	sampleUserBlank    *auth.UserRecord
	sampleUserWithData *auth.UserRecord
}{}

func TestUserManagement(t *testing.T) {
	t.Run("Create test users", testCreateUsers)
	t.Run("Get user", testGetUser)
	t.Run("Iterate users", testUserIterator)
	t.Run("Paged iteration", testPager)
	t.Run("Disable user account", testDisableUser)
	t.Run("Update user", testUpdateUser)
	t.Run("Remove user attributes", testRemovePhonePhotoName)
	t.Run("Remove custom claims", testRemoveCustomClaims)
	t.Run("Add custom claims", testAddCustomClaims)
	t.Run("Delete test users", testDeleteUsers)
}

// N.B if the tests are failing due to inability to create existing users, manual
// cleanup of the previus test run might be required, delete the unwanted users via:
// https://console.firebase.google.com/u/0/project/<project-id>/authentication/users
func testCreateUsers(t *testing.T) {
	// Create users with uid
	for i := 0; i < 3; i++ {
		params := (&auth.UserToCreate{}).UID(fmt.Sprintf("tempTestUserID-%d", i))
		u, err := client.CreateUser(context.Background(), params)
		if err != nil {
			t.Fatal("failed to create user", i, err)
		}
		testFixtures.uidList = append(testFixtures.uidList, u.UID)
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
		Password("password")
	u, err = client.CreateUser(context.Background(), params)
	if err != nil {
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
		UserInfo: &auth.UserInfo{UID: testFixtures.sampleUserBlank.UID},
		UserMetadata: &auth.UserMetadata{
			CreationTimestamp: testFixtures.sampleUserBlank.UserMetadata.CreationTimestamp,
		},
	}
	if !reflect.DeepEqual(u, want) {
		t.Errorf("GetUser() = %v; want = %v", u, want)
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
			Email:       "abc@ab.ab",
		},
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
		}
		phoneUI := &auth.UserInfo{
			PhoneNumber: "+12345678901",
			ProviderID:  "phone",
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
		t.Errorf("UpdateUser() = %v; want = %v", u, want)
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
		err := client.DeleteUser(context.Background(), id)
		if err != nil {
			t.Errorf("DeleteUser(%q) = %v; want = nil", id, err)
		}

		u, err := client.GetUser(context.Background(), id)
		if u != nil || err == nil {
			t.Errorf("GetUser(non-existing) = (%v, %v); want = (nil, error)", u, err)
		}
	}
}
