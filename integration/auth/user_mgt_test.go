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
	sampleUserNil      *auth.UserRecord
	sampleUserWithData *auth.UserRecord
}{}

func TestUserManagement(t *testing.T) {
	t.Run("add some users", createTestUsers)
	t.Run("created users", testCreatedAllUsers)
	t.Run("get user", testGetUser)
	t.Run("user iterator test", testUserIterator)
	t.Run("paging iterator test", testPager)
	t.Run("disable", testDisableUser)

	t.Run("update user", testUpdateUser)
	t.Run("disable", testDisableUser)

	t.Run("remove PhoneNumber, PhotoURL and DisplaName", testRemovePhonePhotoName)

	t.Run("Remove custom claims", testRemoveCustomClaims)
	t.Run("add custom claims", testAddCustomClaims)

	t.Run("delete all users", cleanupUsers)
}

func createTestUsers(t *testing.T) {
	for i := 0; i < 2; i++ {
		u, err := client.CreateUser(context.Background(),
			(&auth.UserToCreate{}).
				UID(fmt.Sprintf("tempTestUserID-%d", i)))
		if err != nil {
			t.Fatal("failed to create user", i, err)
		}
		testFixtures.uidList = append(testFixtures.uidList, u.UID)
	}
	u, err := client.CreateUser(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	testFixtures.sampleUserNil = u
	testFixtures.uidList = append(testFixtures.uidList, u.UID)

	u, err = client.CreateUser(context.Background(), (&auth.UserToCreate{}))
	if err != nil {
		t.Fatal(err)
	}
	testFixtures.sampleUserBlank = u
	testFixtures.uidList = append(testFixtures.uidList, u.UID)
	uid := "tempUserId1234"

	u, err = client.CreateUser(context.Background(), (&auth.UserToCreate{}).
		UID(uid).
		Email(uid+"email@test.com").
		DisplayName("display_name").
		Password("passawd"))
	if err != nil {
		t.Fatal(err)
	}
	testFixtures.sampleUserWithData = u
	testFixtures.uidList = append(testFixtures.uidList, u.UID)
}

func testCreatedAllUsers(t *testing.T) {
	for _, id := range testFixtures.uidList {
		_, err := client.GetUser(context.Background(), id)
		if err != nil {
			t.Errorf("GetUser(%q) user not found. %s", id, err)
		}
	}
}

func testGetUser(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserWithData.UID)
	if err != nil {
		t.Fatalf("error getting user %s", err)
	}
	if u.UID != testFixtures.sampleUserWithData.UID || u.Email != testFixtures.sampleUserWithData.Email {
		t.Errorf("GetUser() = %#v; want: %#v", testFixtures.sampleUserWithData, u.UserInfo)
	}
	if !reflect.DeepEqual(u, testFixtures.sampleUserWithData) {
		t.Errorf("GetUser(UID) = %#v; want: %#v", u, testFixtures.sampleUserWithData)
	}
}

func testGetUserByPhoneNumber(t *testing.T) {
	u, err := client.GetUserByPhoneNumber(context.Background(), testFixtures.sampleUserWithData.PhoneNumber)
	if err != nil {
		t.Fatalf("error getting user %s", err)
	}
	if u.UID != testFixtures.sampleUserWithData.UID || u.PhoneNumber != testFixtures.sampleUserWithData.PhoneNumber {
		t.Errorf("GetUserByPhoneNumber(%q) = %#v; want: %#v",
			testFixtures.sampleUserWithData.PhoneNumber, u, testFixtures.sampleUserWithData)
	}
	if !reflect.DeepEqual(u, testFixtures.sampleUserWithData) {
		t.Errorf("GetUserByPhoneNumber(%q) = %#v; want: %#v",
			testFixtures.sampleUserWithData.PhoneNumber, u, testFixtures.sampleUserWithData)
	}
}

func testGetUserByEmail(t *testing.T) {
	u, err := client.GetUserByEmail(context.Background(), testFixtures.sampleUserWithData.Email)
	if err != nil {
		t.Fatalf("error getting user %s", err)
	}
	if u.UID != testFixtures.sampleUserWithData.UID || u.Email != testFixtures.sampleUserWithData.Email {
		t.Errorf("GetUserByEmail(%q) = %#v; want: %#v",
			testFixtures.sampleUserWithData.Email, u, testFixtures.sampleUserWithData)
	}
	if !reflect.DeepEqual(u, testFixtures.sampleUserWithData) {
		t.Errorf("GetUserByEmail(%q) = %#v; want: %#v",
			testFixtures.sampleUserWithData.Email, u, testFixtures.sampleUserWithData)
	}
}

func testUserIterator(t *testing.T) {
	iter := client.Users(context.Background(), "")
	uids := map[string]bool{}
	gotCount := 0

	for {
		u, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		gotCount++
		uids[u.UID] = true
	}
	if gotCount < 5 {
		t.Errorf("Users() got %d users; want: at least 5 users", gotCount)
	}
	// verify that all the expected users are present
	for _, uid := range testFixtures.uidList {
		if _, ok := uids[uid]; !ok {
			t.Errorf("Users() missing UID %s; want: UID %s", uid, uid)
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
	if userCount < 5 || pageCount < 3 {
		t.Errorf("Users(), NewPager() got %d pages with %d users;  want at least %d pages with at least %d users,", pageCount, userCount, 3, 5)
	}
}

func testDisableUser(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Disabled {
		t.Errorf("GetUser() user is disabled; want: user not disabled")
	}

	_, err = client.UpdateUser(context.Background(), u.UID,
		(&auth.UserToUpdate{}).Disabled(true))
	if err != nil {
		t.Fatal(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if !u.Disabled {
		t.Errorf("Disabled() user is not disabled; want: user disabled")
	}
	_, err = client.UpdateUser(context.Background(), u.UID,
		(&auth.UserToUpdate{}).Disabled(false))
	if err != nil {
		t.Fatal(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Disabled {
		t.Errorf("Disabled() user is disabled; want: user not disabled")
	}
}

func testUpdateUser(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)

	if err != nil || u == nil {
		t.Fatalf("error getting user %s", err)
	}
	refU := &auth.UserRecord{
		UserInfo: &auth.UserInfo{UID: testFixtures.sampleUserBlank.UID},
		UserMetadata: &auth.UserMetadata{
			CreationTimestamp: testFixtures.sampleUserBlank.UserMetadata.CreationTimestamp,
		},
	}
	if !reflect.DeepEqual(u, refU) {
		t.Errorf("GetUser() got = %s; \nwant: %s", toString(refU), toString(u))
	}
	utup := (&auth.UserToUpdate{}).
		Disabled(false).
		DisplayName("name").
		PhoneNumber("+12345678901").
		PhotoURL("http://photo.png").
		Email("abc@ab.ab").
		EmailVerified(true).
		Password("wordpass").
		CustomClaims(map[string]interface{}{"custom": "claims"})

	u, err = client.UpdateUser(context.Background(), u.UID, utup)
	if err != nil {
		t.Fatal(err)
	}

	refU = &auth.UserRecord{
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
	// compare provider info seperatley since the order of the providers isn't guaranteed.
	testProviderInfo(u.ProviderUserInfo,
		&auth.UserInfo{
			DisplayName: "name",
			Email:       "abc@ab.ab",
			PhotoURL:    "http://photo.png",
			ProviderID:  "password"},
		&auth.UserInfo{
			PhoneNumber: "+12345678901",
			ProviderID:  "phone"},
		t)
	// now compare the rest of the record, without the ProviderInfo
	u.ProviderUserInfo = nil
	if !reflect.DeepEqual(u, refU) {
		t.Errorf("UpdateUser() got = %s\nexpecting: %s", toString(u), toString(refU))
	}
}

func testProviderInfo(pi []*auth.UserInfo, passwordUI, phoneUI *auth.UserInfo, t *testing.T) {
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

func testRemovePhonePhotoName(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Fatal(err)
	}
	if u.PhoneNumber == "" {
		t.Errorf("GetUser() expecting non empty PhoneNumber")
	}
	if len(u.ProviderUserInfo) != 2 {
		t.Errorf("GetUser() expecting 2 providers")
	}
	if u.PhotoURL == "" {
		t.Errorf("GetUser() expecting non empty PhotoURL")
	}
	if u.DisplayName == "" {
		t.Errorf("GetUser() expecting non empty DisplayName")
	}

	_, err = client.UpdateUser(context.Background(), u.UID,
		(&auth.UserToUpdate{}).PhoneNumber("").PhotoURL("").DisplayName(""))
	if err != nil {
		t.Fatal(err)
	}

	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if u.PhoneNumber != "" {
		t.Errorf("UpdateUser() [remove PhoneNumber] PhoneNumber = %q; want: PhoneNumber=%q", u.PhoneNumber, "")
	}
	if len(u.ProviderUserInfo) != 1 {
		t.Errorf("UpdateUser() [remove PhoneNumber] got %d ProviderUserInfo records, want:want 1 provider", len(u.ProviderUserInfo))
	}
	if u.DisplayName != "" {
		t.Errorf("UpdateUser() [remove DisplayName] DisplayName = %q; want: DisplayName=%q", u.DisplayName, "")
	}
	if u.PhotoURL != "" {
		t.Errorf("UpdateUser() [remove PhotoURL] PhotoURL = %q; want: PhotoURL=%q", u.PhotoURL, "")
	}
}

func testRemoveCustomClaims(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(u.CustomClaims,
		map[string]interface{}{"custom": "claims"}) {
		t.Errorf("expecting CustomClaims")
	}

	err = client.SetCustomUserClaims(context.Background(), u.UID, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if u.CustomClaims != nil {
		t.Errorf("CustomClaims() = %#v; want {}", u.CustomClaims)

	}
}

func testAddCustomClaims(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Fatal(err)
	}
	if u.CustomClaims != nil {
		t.Errorf("GetUser(), CustomClaims = %q; want <nil>", u.CustomClaims)
	}

	_, err = client.UpdateUser(context.Background(), u.UID,
		(&auth.UserToUpdate{}).CustomClaims(map[string]interface{}{"2custom": "2claims"}))
	if err != nil {
		t.Fatal(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if u.CustomClaims == nil {
		t.Errorf("CustomClaims = <nil>; want: {\"2custom\": \"2claims\"}")
	}
}

func provString(e *auth.UserRecord) string {
	providerStr := ""
	if e.ProviderUserInfo != nil {
		for _, info := range e.ProviderUserInfo {
			providerStr += fmt.Sprintf("\n            %#v", info)
		}
	}
	return providerStr
}

func cleanupUsers(t *testing.T) {
	for _, id := range testFixtures.uidList {
		err := client.DeleteUser(context.Background(), id)
		if err != nil {
			t.Errorf("DeleteUser(%s) error deleting user, %s", id, err)
		}
	}
}

func toString(e *auth.UserRecord) string {
	return fmt.Sprintf(
		"    UserRecord: %#v\n"+
			"        UserInfo: %#v\n"+
			"        MetaData: %#v\n"+
			"        CustomClaims: %#v\n"+
			"        ProviderData: %#v %s",
		e,
		e.UserInfo,
		e.UserMetadata,
		e.CustomClaims,
		e.ProviderUserInfo,
		provString(e))
}
