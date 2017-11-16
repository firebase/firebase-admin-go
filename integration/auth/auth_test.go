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

	"firebase.google.com/go/utils"

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
	for i := 0; i < 3; i++ {
		u, err := client.CreateUser(context.Background(), nil)
		if err != nil {
			return false
		}
		testFixtures.uidList = append(testFixtures.uidList, u.UID)
	}
	u, err := client.CreateUser(context.Background(), &auth.UserCreateParams{DisplayName: utils.StringP("stringh")})
	if err != nil {
		fmt.Println(90)
		return false
	}
	testFixtures.sampleUserBlank = u
	u, err = client.CreateUser(context.Background(), &auth.UserCreateParams{
		UID:         utils.StringP(uid),
		Email:       utils.StringP(uid + "eml5f@test.com"),
		DisplayName: utils.StringP("display_name"),
		Password:    utils.StringP("assawd"),
	})
	if err != nil {
		fmt.Println(100)
		fmt.Println(err, u)
		return false
	}
	testFixtures.sampleUserWithData = u
	return true
}
func cleanupTests() bool {
	lp, err := client.ListUsersWithMaxResults(context.Background(), "", 4)
	if err != nil {
		return false
	}
	for ui := range lp.IterateAll(context.Background()) {
		u, err := ui.Value()
		if err != nil {
			return false
		}
		err = client.DeleteUser(context.Background(), u.UID)
		if err != nil {
			return false
		}
	}
	return true
}
func TestListUsers(t *testing.T) {
	page, _ := client.ListUsersWithMaxResults(context.Background(), "", 2)
	num := 0
	for i, u := range page.Users {
		fmt.Printf("%#v %#v\n", i, u)
		num++
	}
	if num != 2 {
		t.Errorf("expecting %d users got %d", 2, num)
	}
}
func TestPagingMax(t *testing.T) {
	page, _ := client.ListUsersWithMaxResults(context.Background(), "", 2)
	npages := 0
	var err error
	for page != nil {
		fmt.Printf("page++  %d -- - \n", npages)
		fmt.Printf("%#v \n%#v\n-\n", page, page.Users)
		for _, u := range page.Users {
			fmt.Printf(" - - %#v\n", u.UserInfo)
		}
		npages++
		page, err = page.Next(context.Background())
		if err != nil {
			t.Error(err)
		}
	}
	fmt.Printf("page  %d -- == \n", npages)
	fmt.Printf("%#v \n ", page)

	if npages != 4 {
		t.Errorf("expecting %d pages, seen %d", 4, npages) // the last page is not nil, but contains 0 users
	}
}
func TestPaging(t *testing.T) {
	page, _ := client.ListUsersWithMaxResults(context.Background(), "", 10)
	npages := 0
	var err error
	for page != nil {
		fmt.Printf("page  %d -- - \n", npages)
		fmt.Printf("%#v \n%#v\n-\n", page, page.Users)
		for _, u := range page.Users {
			fmt.Printf(" - - %#v\n", u.UserInfo)
		}
		npages++
		page, err = page.Next(context.Background())
		if err != nil {
			t.Error(err)
		}
	}
	fmt.Printf("page  %d -- == \n", npages)
	fmt.Printf("%#v \n ", page)

	if npages != 2 {
		t.Errorf("expecting %d pages, seen %d", 2, npages) // the last page is not nil, but contains 0 users
	}
}
func TestIterator(t *testing.T) {
	page, _ := client.ListUsersWithMaxResults(context.Background(), "", 2)
	nitems := 0
	for ui := range page.IterateAll(context.Background()) {
		_, err := ui.Value()
		if err != nil {
			t.Error(err)
		}
		nitems++
	}
	if nitems != 5 {
		t.Errorf("expecting %d items, seen %d", 5, nitems)
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
