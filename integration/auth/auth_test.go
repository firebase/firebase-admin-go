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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"testing"

	"google.golang.org/api/iterator"

	"firebase.google.com/go/auth"
	"firebase.google.com/go/integration/internal"

	"golang.org/x/net/context"
)

var client *auth.Client

var testFixtures = struct {
	uidList            []string
	sampleUserBlank    *auth.UserRecord
	sampleUserNil      *auth.UserRecord
	sampleUserWithData *auth.UserRecord
}{}

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping auth integration tests in short mode.")
		os.Exit(0)
	}

	ctx := context.Background()
	app, err := internal.NewTestApp(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	client, err = app.Auth(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(m.Run())
}

func TestUserManagement(t *testing.T) {
	t.Run("add some users", populateSomeUsers)
	t.Run("created users", createdUsers)
	t.Run("get user", testGetUser)
	t.Run("user iterator test", testUserIterator)
	t.Run("paging iterator test", testIterPage)
	t.Run("disable", testDisableUser)

	t.Run("update user", testUpdateUser)
	t.Run("disable", testDisableUser)

	t.Run("remove Display Name", testRemoveDisplayName)
	t.Run("remove PhotoURL", testRemovePhotoURL)
	t.Run("remove PhoneNumber", testRemovePhone)

	t.Run("Remove custom claims", testRemoveCustomClaims)
	t.Run("add custom claims", testAddCustomClaims)

	t.Run("delete all users", cleanupUsers)
}

func createdUsers(t *testing.T) {
	for _, id := range testFixtures.uidList {
		_, err := client.GetUser(context.Background(), id)
		if err != nil {
			t.Errorf("can't find User uid %s, %s", id, err)
		}
	}

}

func populateSomeUsers(t *testing.T) {
	for i := 0; i < 2; i++ {
		u, err := client.CreateUser(context.Background(), (&auth.UserToCreate{}).UID(fmt.Sprintf("userid-%d", i)))
		if err != nil {
			t.Fatal("trouble creating", i, err)
		}
		testFixtures.uidList = append(testFixtures.uidList, u.UID)
	}
	u, err := client.CreateUser(context.Background(), nil)
	if err != nil {
		t.Error(err)
	}
	testFixtures.sampleUserNil = u
	testFixtures.uidList = append(testFixtures.uidList, u.UID)
	u, err = client.CreateUser(context.Background(), &auth.UserToCreate{})
	if err != nil {
		t.Error(err)
	}
	testFixtures.sampleUserBlank = u
	testFixtures.uidList = append(testFixtures.uidList, u.UID)
	uid := "tefwfd1234"
	u, err = client.CreateUser(context.Background(), (&auth.UserToCreate{}).
		UID(uid).
		Email(uid+"eml5f@test.com").
		DisplayName("display_name").
		Password("passawd"))

	if err != nil {
		t.Error(err)
	}
	testFixtures.sampleUserWithData = u
	testFixtures.uidList = append(testFixtures.uidList, u.UID)
}

func testGetUser(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserWithData.UID)
	if err != nil {
		t.Errorf("error getting user %s", err)
	}
	if u.UID != testFixtures.sampleUserWithData.UID || u.Email != testFixtures.sampleUserWithData.Email {
		t.Errorf("expecting %#v got %#v", testFixtures.sampleUserWithData, u.UserInfo)
	}
	if !reflect.DeepEqual(u, testFixtures.sampleUserWithData) {
		t.Errorf("expecting %#v got %#v", testFixtures.sampleUserWithData, u)
	}
}
func TestClean(t *testing.T) {
	iter := client.Users(context.Background(), "")
	for {
		u, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		client.DeleteUser(context.Background(), u.UID)
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
		t.Errorf("expecting at least 5 users got %d", gotCount)
	}
	// verify that all the expected users are present
	for _, uid := range testFixtures.uidList {
		if _, ok := uids[uid]; !ok {
			t.Errorf("missing wanted UID %s", uid)
		}
	}
}

func testIterPage(t *testing.T) {
	iter := client.Users(context.Background(), "")
	pager := iterator.NewPager(iter, 2, "")
	userCount := 0
	pageCount := 0

	for {
		pageCount++
		var users []*auth.ExportedUserRecord
		nextPageToken, err := pager.NextPage(&users)
		userCount += len(users)
		if err != nil {
			t.Errorf("paging error %v", err)
		}
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
		t.Errorf("expecting at least %d pages with at least %d users, got %d with %d ", 3, 5, pageCount, userCount)
	}
}

func testDisableUser(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Error(err)
	}
	if u.Disabled {
		t.Errorf("expecting user not disabled")
	}

	_, err = client.UpdateUser(context.Background(), u.UID,
		(&auth.UserToUpdate{}).Disabled(true))
	if err != nil {
		t.Error(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if !u.Disabled {
		t.Errorf("expecting user disabled")
	}
	_, err = client.UpdateUser(context.Background(), u.UID,
		(&auth.UserToUpdate{}).Disabled(false))
	if err != nil {
		t.Error(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Error(err)
	}
	if u.Disabled {
		t.Errorf("expecting user enabled")
	}
}

func testUpdateUser(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)

	if err != nil || u == nil {
		t.Errorf("error getting user %s", err)
	}
	refU := &auth.UserRecord{
		UserInfo: &auth.UserInfo{UID: testFixtures.sampleUserBlank.UID},
		UserMetadata: &auth.UserMetadata{
			CreationTimestamp: testFixtures.sampleUserBlank.UserMetadata.CreationTimestamp,
		},
	}
	if !reflect.DeepEqual(u, refU) {
		t.Errorf("\ngot %s, \nexpecting %s", toString(refU), toString(u))
	}
	up := (&auth.UserToUpdate{}).
		Disabled(false).
		DisplayName("name").
		PhoneNumber("+12345678901").
		PhotoURL("http://photo.png").
		Email("abc@ab.ab").
		EmailVerified(true).
		Password("wordpass").
		CustomClaims(map[string]interface{}{"custom": "claims"})

	_, err = client.UpdateUser(context.Background(), u.UID, up)
	if err != nil {
		t.Error(err)
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
	u, err = client.GetUser(context.Background(), u.UID)
	testPI(u.ProviderUserInfo,
		&auth.UserInfo{
			DisplayName: "name",
			Email:       "abc@ab.ab",
			PhotoURL:    "http://photo.png",
			ProviderID:  "password"},
		&auth.UserInfo{
			PhoneNumber: "+12345678901",
			ProviderID:  "phone"},
		t)
	u.ProviderUserInfo = nil
	if !reflect.DeepEqual(u, refU) {
		t.Errorf("\ngot %s\nexpecting %s", toString(u), toString(refU))
	}
}

func testPI(pi []*auth.UserInfo, passwordUI, phoneUI *auth.UserInfo, t *testing.T) {
	var compareWith *auth.UserInfo
	for _, ui := range pi {
		switch ui.ProviderID {
		case "password":
			compareWith = passwordUI
		case "phone":
			compareWith = phoneUI
		}
		if !reflect.DeepEqual(ui, compareWith) {
			t.Errorf("\ngot %#v, \nexpecting %#v", ui, compareWith)
		}
	}

}

func testRemoveDisplayName(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Error(err)
	}
	if u.DisplayName == "" {
		t.Errorf("expecting non empty display name")
	}

	_, err = client.UpdateUser(context.Background(), u.UID, (&auth.UserToUpdate{}).DisplayName(""))
	if err != nil {
		t.Error(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if u.DisplayName != "" {
		t.Errorf("expecting non empty display name")
	}
}

func testRemovePhotoURL(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Error(err)
	}
	if u.PhotoURL == "" {
		t.Errorf("expecting non empty display name")
	}

	_, err = client.UpdateUser(context.Background(), u.UID, (&auth.UserToUpdate{}).PhotoURL(""))
	if err != nil {
		t.Error(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if u.PhotoURL != "" {
		t.Errorf("expecting non empty display name")
	}
}

func testRemovePhone(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Error(err)
	}
	if u.PhoneNumber == "" {
		t.Errorf("expecting non empty display name")
	}
	if len(u.ProviderUserInfo) != 2 {
		t.Errorf("expecting 2 providers")
	}

	_, err = client.UpdateUser(context.Background(), u.UID,
		(&auth.UserToUpdate{}).PhoneNumber(""))
	if err != nil {
		t.Error(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if u.PhoneNumber != "" {
		t.Errorf("expecting non empty display name")
	}
	if len(u.ProviderUserInfo) != 1 {
		t.Errorf("expecting 1 provider")
	}
}

func testRemoveCustomClaims(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(u.CustomClaims,
		map[string]interface{}{"custom": "claims"}) {
		t.Errorf("expecting CustomClaims")
	}

	err = client.SetCustomUserClaims(context.Background(), u.UID, nil)
	if err != nil {
		t.Error(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if u.CustomClaims != nil {
		t.Errorf("expecting empty CC, \n\n \n--%T %v-\n%s\n", u.CustomClaims, u.CustomClaims, toString(u))

	}
}

func testAddCustomClaims(t *testing.T) {
	u, err := client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if err != nil {
		t.Error(err)
	}
	if u.CustomClaims != nil {
		t.Errorf("expecting CustomClaims empty")
	}

	_, err = client.UpdateUser(context.Background(), u.UID,
		(&auth.UserToUpdate{}).CustomClaims(map[string]interface{}{"2custom": "2claims"}))
	if err != nil {
		t.Error(err)
	}
	u, err = client.GetUser(context.Background(), testFixtures.sampleUserBlank.UID)
	if u.CustomClaims == nil {
		t.Errorf("expecting non  empty Email")
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
			t.Errorf("error deleting uid %s, %s", id, err)
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

// Tokens

func TestCustomToken(t *testing.T) {
	ct, err := client.CustomToken("user1")

	if err != nil {
		t.Fatal(err)
	}

	idt, err := signInWithCustomToken(ct)
	if err != nil {
		t.Fatal(err)
	}

	vt, err := client.VerifyIDToken(idt)
	if err != nil {
		t.Fatal(err)
	}
	if vt.UID != "user1" {
		t.Errorf("UID = %q; want UID = %q", vt.UID, "user1")
	}
}

func TestCustomTokenWithClaims(t *testing.T) {
	ct, err := client.CustomTokenWithClaims("user1", map[string]interface{}{
		"premium": true,
		"package": "gold",
	})
	if err != nil {
		t.Fatal(err)
	}

	idt, err := signInWithCustomToken(ct)
	if err != nil {
		t.Fatal(err)
	}

	vt, err := client.VerifyIDToken(idt)
	if err != nil {
		t.Fatal(err)
	}
	if vt.UID != "user1" {
		t.Errorf("UID = %q; want UID = %q", vt.UID, "user1")
	}
	if premium, ok := vt.Claims["premium"].(bool); !ok || !premium {
		t.Errorf("Claims['premium'] = %v; want Claims['premium'] = true", vt.Claims["premium"])
	}
	if pkg, ok := vt.Claims["package"].(string); !ok || pkg != "gold" {
		t.Errorf("Claims['package'] = %v; want Claims['package'] = \"gold\"", vt.Claims["package"])
	}
}

func signInWithCustomToken(token string) (string, error) {
	req, err := json.Marshal(map[string]interface{}{
		"token":             token,
		"returnSecureToken": true,
	})
	if err != nil {
		return "", err
	}

	apiKey, err := internal.APIKey()
	if err != nil {
		return "", err
	}
	resp, err := postRequest(fmt.Sprintf("https://www.googleapis.com/identitytoolkit/v3/relyingparty/verifyCustomToken?key=%s", apiKey), req)
	if err != nil {
		return "", err
	}
	var respBody struct {
		IDToken string `json:"idToken"`
	}
	if err := json.Unmarshal(resp, &respBody); err != nil {
		return "", err
	}
	return respBody.IDToken, err
}

func postRequest(url string, req []byte) ([]byte, error) {
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(req))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}
