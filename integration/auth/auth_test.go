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
	"testing"

	"firebase.google.com/go/p"
	"google.golang.org/api/iterator"

	"firebase.google.com/go/auth"
	"firebase.google.com/go/integration/internal"

	"golang.org/x/net/context"
)

const verifyCustomToken = "verifyCustomToken?key=%s"

var client *auth.Client

var testFixtures = struct {
	uidList            []string
	sampleUserBlank    *auth.UserRecord
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
	if ok := prepareTests(); !ok {
		os.Exit(11)
	}

	exitVal := m.Run()

	if ok := cleanupTests(); !ok {
		os.Exit(12)
	}
	os.Exit(exitVal)
}

var uid string

func prepareTests() bool {
	if ok := cleanupTests(); !ok {
		fmt.Println("trouble cleaning up from previous run.")
		return false
	}
	uid = "tefwfd1234"
	for i := 0; i < 1; i++ {
		u, err := client.CreateUser(context.Background(), &auth.UserCreateParams{UID: p.String(fmt.Sprintf("user -- %d.", i))})
		if err != nil {
			fmt.Println("trouble creating", i, err)
			return false
		}
		testFixtures.uidList = append(testFixtures.uidList, u.UID)
	}
	u, err := client.CreateUser(context.Background(), &auth.UserCreateParams{})
	if err != nil {
		fmt.Println(err)
		return false
	}
	testFixtures.sampleUserBlank = u
	u, err = client.CreateUser(context.Background(), &auth.UserCreateParams{
		UID:          p.String(uid),
		Email:        p.String(uid + "eml5f@test.com"),
		DisplayName:  p.String("display_name"),
		Password:     p.String("assawd"),
		CustomClaims: &auth.CustomClaimsMap{"asdf": true, "asdfdf": "ffd"},
	})
	if err != nil {
		fmt.Println(err, u)
		return false
	}
	fmt.Printf("DEBUG %#v \n", u)
	testFixtures.sampleUserWithData = u
	return true
}
func cleanupTests() bool {
	iter := client.Users(context.Background(), auth.WithMaxSize(19))
	var uids []string
loop:
	for {
		user, err := iter.Next()
		switch err {
		case nil:
			uids = append(uids, user.UID)
		case iterator.Done:
			break loop
		default:
			fmt.Println("error ", err)
			return false
		}
	}
	fmt.Println(uids)
	for i, uid := range uids {
		println("deleting ", i, uid)
		fmt.Println(uid)
		err := client.DeleteUser(context.Background(), uid)
		if err != nil {
			fmt.Println("error deleting uid ", uid, err)
			return false
		}
	}
	return true

}

func TestUserIterator(t *testing.T) {
	iter := client.Users(context.Background(), auth.WithMaxSize(2))
	var uids []string
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
		uids = append(uids, u.UID)
	}
	if gotCount != 5 {
		t.Errorf("\n-----hhh---\n%#v -MMMMM M MMM M- %v\n%d\n", iter, uids, gotCount)
	}
}
func TestIterPage(t *testing.T) {
	iter := client.Users(context.Background(), auth.WithMaxSize(2))
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
			fmt.Println(u)
		}
		if nextPageToken == "" {
			break
		}
	}
	if userCount != 5 || pageCount != 3 {
		t.Errorf("expecting %d pages with %d users, got %d with %d ", pageCount, userCount, 3, 5)
	}
}
func TestGetUser(t *testing.T) {

	u, err := client.GetUser(context.Background(), uid)

	if err != nil {
		t.Errorf("error getting user %s", err)
	}
	if u.UID != testFixtures.sampleUserWithData.UID || u.Email != testFixtures.sampleUserWithData.Email {
		t.Errorf("expecting %#v got %#v", testFixtures.sampleUserWithData, u.UserInfo)
	}
}

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
	resp, err := postRequest(fmt.Sprintf(auth.IDToolKitURL()+verifyCustomToken, apiKey), req)
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
