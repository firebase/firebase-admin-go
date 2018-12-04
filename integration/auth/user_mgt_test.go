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
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/auth"
	"firebase.google.com/go/auth/hash"
	"google.golang.org/api/iterator"
)

func TestGetUser(t *testing.T) {
	want := newUserWithParams(t)
	defer deleteUser(want.UID)

	cases := []struct {
		name  string
		getOp func(context.Context) (*auth.UserRecord, error)
	}{
		{
			"GetUser()",
			func(ctx context.Context) (*auth.UserRecord, error) {
				return client.GetUser(ctx, want.UID)
			},
		},
		{
			"GetUserByEmail()",
			func(ctx context.Context) (*auth.UserRecord, error) {
				return client.GetUserByEmail(ctx, want.Email)
			},
		},
		{
			"GetUserByPhoneNumber()",
			func(ctx context.Context) (*auth.UserRecord, error) {
				return client.GetUserByPhoneNumber(ctx, want.PhoneNumber)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.getOp(context.Background())
			if err != nil || !reflect.DeepEqual(*got, *want) {
				t.Errorf("%s = (%#v, %v); want = (%#v, nil)", tc.name, got, err, want)
			}
		})
	}
}

func TestGetNonExistingUser(t *testing.T) {
	user, err := client.GetUser(context.Background(), "non.existing")
	if user != nil || !auth.IsUserNotFound(err) {
		t.Errorf("GetUser(non.existing) = (%v, %v); want = (nil, error)", user, err)
	}

	user, err = client.GetUserByEmail(context.Background(), "non.existing@definitely.non.existing")
	if user != nil || !auth.IsUserNotFound(err) {
		t.Errorf("GetUserByEmail(non.existing) = (%v, %v); want = (nil, error)", user, err)
	}
}

func TestUpdateNonExistingUser(t *testing.T) {
	update := (&auth.UserToUpdate{}).Email("test@example.com")
	user, err := client.UpdateUser(context.Background(), "non.existing", update)
	if user != nil || !auth.IsUserNotFound(err) {
		t.Errorf("UpdateUser(non.existing) = (%v, %v); want = (nil, error)", user, err)
	}
}

func TestDeleteNonExistingUser(t *testing.T) {
	err := client.DeleteUser(context.Background(), "non.existing")
	if !auth.IsUserNotFound(err) {
		t.Errorf("DeleteUser(non.existing) = %v; want = error", err)
	}
}

func TestListUsers(t *testing.T) {
	newUsers := map[string]bool{}
	user := newUserWithParams(t)
	defer deleteUser(user.UID)
	newUsers[user.UID] = true

	user = newUserWithParams(t)
	defer deleteUser(user.UID)
	newUsers[user.UID] = true

	user = newUserWithParams(t)
	defer deleteUser(user.UID)
	newUsers[user.UID] = true

	// test regular iteration
	count := 0
	iter := client.Users(context.Background(), "")
	for {
		u, err := iter.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if _, ok := newUsers[u.UID]; ok {
			count++
			if u.PasswordHash == "" {
				t.Errorf("Users() PasswordHash = empty; want = non-empty")
			}
		}
	}
	if count < 3 {
		t.Errorf("Users() count = %d;  want >= 3", count)
	}

	// test paged iteration
	count = 0
	pageCount := 0
	iter = client.Users(context.Background(), "")
	pager := iterator.NewPager(iter, 2, "")
	for {
		pageCount++
		var users []*auth.ExportedUserRecord
		nextPageToken, err := pager.NextPage(&users)
		if err != nil {
			t.Fatal(err)
		}
		count += len(users)
		if nextPageToken == "" {
			break
		}
	}
	if count < 3 {
		t.Errorf("Users() count = %d;  want >= 3", count)
	}
	if pageCount < 2 {
		t.Errorf("NewPager() pages = %d;  want >= 2", pageCount)
	}
}

func TestCreateUser(t *testing.T) {
	user, err := client.CreateUser(context.Background(), nil)
	if err != nil {
		t.Fatalf("CreateUser() = %v; want = nil", err)
	}
	defer deleteUser(user.UID)
	want := auth.UserRecord{
		UserInfo: &auth.UserInfo{
			UID:        user.UID,
			ProviderID: "firebase",
		},
		UserMetadata: &auth.UserMetadata{
			CreationTimestamp: user.UserMetadata.CreationTimestamp,
		},
		TokensValidAfterMillis: user.TokensValidAfterMillis,
	}
	if !reflect.DeepEqual(*user, want) {
		t.Errorf("CreateUser() = %#v; want = %#v", *user, want)
	}

	user, err = client.CreateUser(context.Background(), (&auth.UserToCreate{}).UID(user.UID))
	if err == nil || user != nil || !auth.IsUIDAlreadyExists(err) {
		t.Errorf("CreateUser(existing-uid) = (%#v, %v); want = (nil, error)", user, err)
	}
}

func TestUpdateUser(t *testing.T) {
	user := newUserWithParams(t)
	defer deleteUser(user.UID)

	uid := randomUID()
	newEmail := randomEmail(uid)
	newPhone := randomPhoneNumber()
	want := auth.UserInfo{
		UID:         user.UID,
		Email:       newEmail,
		PhoneNumber: newPhone,
		DisplayName: "Updated Name",
		ProviderID:  "firebase",
		PhotoURL:    "https://example.com/updated.png",
	}
	params := (&auth.UserToUpdate{}).
		Email(newEmail).
		PhoneNumber(newPhone).
		DisplayName("Updated Name").
		PhotoURL("https://example.com/updated.png").
		EmailVerified(true).
		Password("newpassowrd")
	got, err := client.UpdateUser(context.Background(), user.UID, params)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*got.UserInfo, want) {
		t.Errorf("UpdateUser().UserInfo = (%#v, %v); want = (%#v, nil)", *got.UserInfo, err, want)
	}
	if !got.EmailVerified {
		t.Error("UpdateUser().EmailVerified = false; want = true")
	}
}

func TestDisableUser(t *testing.T) {
	user := newUserWithParams(t)
	defer deleteUser(user.UID)
	if user.Disabled {
		t.Errorf("NewUser.Disabled = true; want = false")
	}

	params := (&auth.UserToUpdate{}).Disabled(true)
	got, err := client.UpdateUser(context.Background(), user.UID, params)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Disabled {
		t.Errorf("UpdateUser().Disabled = false; want = true")
	}

	params = (&auth.UserToUpdate{}).Disabled(false)
	got, err = client.UpdateUser(context.Background(), user.UID, params)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disabled {
		t.Errorf("UpdateUser().Disabled = true; want = false")
	}
}

func TestRemovePhonePhotoName(t *testing.T) {
	user := newUserWithParams(t)
	defer deleteUser(user.UID)
	if user.PhoneNumber == "" {
		t.Errorf("NewUser.PhoneNumber = empty; want = non-empty")
	}
	if len(user.ProviderUserInfo) != 2 {
		t.Errorf("NewUser.ProviderUserInfo = %d; want = 2", len(user.ProviderUserInfo))
	}
	if user.PhotoURL == "" {
		t.Errorf("NewUser.PhotoURL = empty; want = non-empty")
	}
	if user.DisplayName == "" {
		t.Errorf("NewUser.DisplayName = empty; want = non-empty")
	}

	params := (&auth.UserToUpdate{}).PhoneNumber("").PhotoURL("").DisplayName("")
	got, err := client.UpdateUser(context.Background(), user.UID, params)
	if err != nil {
		t.Fatal(err)
	}
	if got.PhoneNumber != "" {
		t.Errorf("UpdateUser().PhoneNumber = %q; want: %q", got.PhoneNumber, "")
	}
	if len(got.ProviderUserInfo) != 1 {
		t.Errorf("UpdateUser().ProviderUserInfo = %d, want = 1", len(got.ProviderUserInfo))
	}
	if got.DisplayName != "" {
		t.Errorf("UpdateUser().DisplayName = %q; want =%q", got.DisplayName, "")
	}
	if got.PhotoURL != "" {
		t.Errorf("UpdateUser().PhotoURL = %q; want = %q", got.PhotoURL, "")
	}
}

func TestSetCustomClaims(t *testing.T) {
	user := newUserWithParams(t)
	defer deleteUser(user.UID)
	if user.CustomClaims != nil {
		t.Fatalf("NewUser.CustomClaims = %#v; want = nil", user.CustomClaims)
	}

	setAndVerifyClaims := func(claims map[string]interface{}) {
		if err := client.SetCustomUserClaims(context.Background(), user.UID, claims); err != nil {
			t.Fatalf("SetCustomUserClaims() = %v; want = nil", err)
		}
		got, err := client.GetUser(context.Background(), user.UID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.CustomClaims, claims) {
			t.Errorf("SetCustomUserClaims().CustomClaims = %#v; want = %#v", got.CustomClaims, claims)
		}
	}
	setAndVerifyClaims(map[string]interface{}{
		"admin":   true,
		"package": "gold",
	})
	setAndVerifyClaims(map[string]interface{}{
		"admin":        false,
		"subscription": "guest",
	})
	setAndVerifyClaims(nil)
}

func TestDeleteUser(t *testing.T) {
	user := newUserWithParams(t)
	if err := client.DeleteUser(context.Background(), user.UID); err != nil {
		t.Fatalf("DeleteUser() = %v; want = nil", err)
	}
	got, err := client.GetUser(context.Background(), user.UID)
	if err == nil || got != nil || !auth.IsUserNotFound(err) {
		t.Errorf("GetUser(deleted) = (%#v, %v); want = (nil, error)", got, err)
	}
}

func TestImportUsers(t *testing.T) {
	uid := randomUID()
	email := randomEmail(uid)
	user := (&auth.UserToImport{}).UID(uid).Email(email)
	result, err := client.ImportUsers(context.Background(), []*auth.UserToImport{user})
	if err != nil {
		t.Fatal(err)
	}
	defer deleteUser(uid)
	if result.SuccessCount != 1 || result.FailureCount != 0 {
		t.Errorf("ImportUsers() = %#v; want = {SuccessCount: 1, FailureCount: 0}", result)
	}

	savedUser, err := client.GetUser(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	if savedUser.Email != email {
		t.Errorf("GetUser(imported) = %q; want = %q", savedUser.Email, email)
	}
}

func TestImportUsersWithPassword(t *testing.T) {
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

	uid := randomUID()
	email := randomEmail(uid)
	passwordHash, err := base64.StdEncoding.DecodeString(rawPasswordHash)
	if err != nil {
		t.Fatal(err)
	}
	user := (&auth.UserToImport{}).
		UID(uid).
		Email(email).
		PasswordHash(passwordHash).
		PasswordSalt([]byte("NaCl"))
	result, err := client.ImportUsers(context.Background(), []*auth.UserToImport{user}, auth.WithHash(scrypt))
	if err != nil {
		t.Fatal(err)
	}
	defer deleteUser(uid)
	if result.SuccessCount != 1 || result.FailureCount != 0 {
		t.Errorf("ImportUsers() = %#v; want = {SuccessCount: 1, FailureCount: 0}", result)
	}

	savedUser, err := client.GetUser(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	if savedUser.Email != email {
		t.Errorf("GetUser(imported) = %q; want = %q", savedUser.Email, email)
	}
	idToken, err := signInWithPassword(email, "password")
	if err != nil {
		t.Fatal(err)
	}
	if idToken == "" {
		t.Errorf("ID Token = empty; want = non-empty")
	}
}

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomUID() string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, 32)
	for i := range b {
		b[i] = letters[seededRand.Intn(len(letters))]
	}
	return string(b)
}

func randomPhoneNumber() string {
	var digits = []rune("0123456789")
	b := make([]rune, 10)
	for i := range b {
		b[i] = digits[rand.Intn(len(digits))]
	}
	return "+1" + string(b)
}

func randomEmail(uid string) string {
	return strings.ToLower(fmt.Sprintf("%s@example.%s.com", uid[:12], uid[12:]))
}

func newUserWithParams(t *testing.T) *auth.UserRecord {
	uid := randomUID()
	email := randomEmail(uid)
	phone := randomPhoneNumber()
	params := (&auth.UserToCreate{}).
		UID(uid).
		Email(email).
		PhoneNumber(phone).
		DisplayName("Random User").
		PhotoURL("https://example.com/photo.png").
		EmailVerified(true).
		Password("password")
	user, err := client.CreateUser(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	return user
}
