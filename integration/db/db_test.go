// Copyright 2018 Google Inc. All Rights Reserved.
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

// Package db contains integration tests for the firebase.google.com/go/db package.
package db

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"testing"

	"firebase.google.com/go"
	"firebase.google.com/go/db"
	"firebase.google.com/go/integration/internal"
)

var client *db.Client
var aoClient *db.Client
var guestClient *db.Client

var ref *db.Ref
var users *db.Ref
var dinos *db.Ref

var testData map[string]interface{}
var parsedTestData map[string]Dinosaur

const permDenied = "http error status: 401; reason: Permission denied"

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping database integration tests in short mode.")
		os.Exit(0)
	}

	pid, err := internal.ProjectID()
	if err != nil {
		log.Fatalln(err)
	}

	client, err = initClient(pid)
	if err != nil {
		log.Fatalln(err)
	}

	aoClient, err = initOverrideClient(pid)
	if err != nil {
		log.Fatalln(err)
	}

	guestClient, err = initGuestClient(pid)
	if err != nil {
		log.Fatalln(err)
	}

	ref = client.NewRef("_adminsdk/go/dinodb")
	dinos = ref.Child("dinosaurs")
	users = ref.Parent().Child("users")

	initRules()
	initData()

	os.Exit(m.Run())
}

func initClient(pid string) (*db.Client, error) {
	ctx := context.Background()
	app, err := internal.NewTestApp(ctx, &firebase.Config{
		DatabaseURL: fmt.Sprintf("https://%s.firebaseio.com", pid),
	})
	if err != nil {
		return nil, err
	}

	return app.Database(ctx)
}

func initOverrideClient(pid string) (*db.Client, error) {
	ctx := context.Background()
	ao := map[string]interface{}{"uid": "user1"}
	app, err := internal.NewTestApp(ctx, &firebase.Config{
		DatabaseURL:  fmt.Sprintf("https://%s.firebaseio.com", pid),
		AuthOverride: &ao,
	})
	if err != nil {
		return nil, err
	}

	return app.Database(ctx)
}

func initGuestClient(pid string) (*db.Client, error) {
	ctx := context.Background()
	var nullMap map[string]interface{}
	app, err := internal.NewTestApp(ctx, &firebase.Config{
		DatabaseURL:  fmt.Sprintf("https://%s.firebaseio.com", pid),
		AuthOverride: &nullMap,
	})
	if err != nil {
		return nil, err
	}

	return app.Database(ctx)
}

func initRules() {
	b, err := ioutil.ReadFile(internal.Resource("dinosaurs_index.json"))
	if err != nil {
		log.Fatalln(err)
	}

	pid, err := internal.ProjectID()
	if err != nil {
		log.Fatalln(err)
	}

	url := fmt.Sprintf("https://%s.firebaseio.com/.settings/rules.json", pid)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(b))
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("Content-Type", "application/json")

	hc, err := internal.NewHTTPClient(context.Background())
	if err != nil {
		log.Fatalln(err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	} else if resp.StatusCode != http.StatusOK {
		log.Fatalln("failed to update rules:", string(b))
	}
}

func initData() {
	b, err := ioutil.ReadFile(internal.Resource("dinosaurs.json"))
	if err != nil {
		log.Fatalln(err)
	}
	if err = json.Unmarshal(b, &testData); err != nil {
		log.Fatalln(err)
	}

	b, err = json.Marshal(testData["dinosaurs"])
	if err != nil {
		log.Fatalln(err)
	}
	if err = json.Unmarshal(b, &parsedTestData); err != nil {
		log.Fatalln(err)
	}

	if err = ref.Set(context.Background(), testData); err != nil {
		log.Fatalln(err)
	}
}

func TestRef(t *testing.T) {
	if ref.Key != "dinodb" {
		t.Errorf("Key = %q; want = %q", ref.Key, "dinodb")
	}
	if ref.Path != "/_adminsdk/go/dinodb" {
		t.Errorf("Path = %q; want = %q", ref.Path, "/_adminsdk/go/dinodb")
	}
}

func TestChild(t *testing.T) {
	c := ref.Child("dinosaurs")
	if c.Key != "dinosaurs" {
		t.Errorf("Key = %q; want = %q", c.Key, "dinosaurs")
	}
	if c.Path != "/_adminsdk/go/dinodb/dinosaurs" {
		t.Errorf("Path = %q; want = %q", c.Path, "/_adminsdk/go/dinodb/dinosaurs")
	}
}

func TestParent(t *testing.T) {
	p := ref.Parent()
	if p.Key != "go" {
		t.Errorf("Key = %q; want = %q", p.Key, "go")
	}
	if p.Path != "/_adminsdk/go" {
		t.Errorf("Path = %q; want = %q", p.Path, "/_adminsdk/go")
	}
}

func TestGet(t *testing.T) {
	var m map[string]interface{}
	if err := ref.Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("Get() = %v; want = %v", m, testData)
	}
}

func TestGetWithETag(t *testing.T) {
	var m map[string]interface{}
	etag, err := ref.GetWithETag(context.Background(), &m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("GetWithETag() = %v; want = %v", m, testData)
	}
	if etag == "" {
		t.Errorf("GetWithETag() = \"\"; want non-empty")
	}
}

func TestGetShallow(t *testing.T) {
	var m map[string]interface{}
	if err := ref.GetShallow(context.Background(), &m); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{}
	for k := range testData {
		want[k] = true
	}
	if !reflect.DeepEqual(want, m) {
		t.Errorf("GetShallow() = %v; want = %v", m, want)
	}
}

func TestGetIfChanged(t *testing.T) {
	var m map[string]interface{}
	ok, etag, err := ref.GetIfChanged(context.Background(), "wrong-etag", &m)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || etag == "" {
		t.Errorf("GetIfChanged() = (%v, %q); want = (%v, %q)", ok, etag, true, "non-empty")
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("GetWithETag() = %v; want = %v", m, testData)
	}

	var m2 map[string]interface{}
	ok, etag2, err := ref.GetIfChanged(context.Background(), etag, &m2)
	if err != nil {
		t.Fatal(err)
	}
	if ok || etag != etag2 {
		t.Errorf("GetIfChanged() = (%v, %q); want = (%v, %q)", ok, etag2, false, etag)
	}
	if len(m2) != 0 {
		t.Errorf("GetWithETag() = %v; want empty", m)
	}
}

func TestGetChildValue(t *testing.T) {
	c := ref.Child("dinosaurs")
	var m map[string]interface{}
	if err := c.Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData["dinosaurs"], m) {
		t.Errorf("Get() = %v; want = %v", m, testData["dinosaurs"])
	}
}

func TestGetGrandChildValue(t *testing.T) {
	c := ref.Child("dinosaurs/lambeosaurus")
	var got Dinosaur
	if err := c.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	want := parsedTestData["lambeosaurus"]
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestGetNonExistingChild(t *testing.T) {
	c := ref.Child("non_existing")
	var i interface{}
	if err := c.Get(context.Background(), &i); err != nil {
		t.Fatal(err)
	}
	if i != nil {
		t.Errorf("Get() = %v; want nil", i)
	}
}

func TestPush(t *testing.T) {
	u, err := users.Push(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/_adminsdk/go/users/"+u.Key {
		t.Errorf("Push() = %q; want = %q", u.Path, "/_adminsdk/go/users/"+u.Key)
	}

	var i interface{}
	if err := u.Get(context.Background(), &i); err != nil {
		t.Fatal(err)
	}
	if i != "" {
		t.Errorf("Get() = %v; want empty string", i)
	}
}

func TestPushWithValue(t *testing.T) {
	want := User{"Luis Alvarez", 1911}
	u, err := users.Push(context.Background(), &want)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/_adminsdk/go/users/"+u.Key {
		t.Errorf("Push() = %q; want = %q", u.Path, "/_adminsdk/go/users/"+u.Key)
	}

	var got User
	if err := u.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestSetPrimitiveValue(t *testing.T) {
	u, err := users.Push(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Set(context.Background(), "value"); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := u.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Errorf("Get() = %q; want = %q", got, "value")
	}
}

func TestSetComplexValue(t *testing.T) {
	u, err := users.Push(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	want := User{"Mary Anning", 1799}
	if err := u.Set(context.Background(), &want); err != nil {
		t.Fatal(err)
	}
	var got User
	if err := u.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestUpdateChildren(t *testing.T) {
	u, err := users.Push(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]interface{}{
		"name":  "Robert Bakker",
		"since": float64(1945),
	}
	if err := u.Update(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := u.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestUpdateChildrenWithExistingValue(t *testing.T) {
	u, err := users.Push(context.Background(), map[string]interface{}{
		"name":  "Edwin Colbert",
		"since": float64(1900),
	})
	if err != nil {
		t.Fatal(err)
	}

	update := map[string]interface{}{"since": float64(1905)}
	if err := u.Update(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := u.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"name":  "Edwin Colbert",
		"since": float64(1905),
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestUpdateNestedChildren(t *testing.T) {
	edward, err := users.Push(context.Background(), map[string]interface{}{
		"name": "Edward Cope", "since": float64(1800),
	})
	if err != nil {
		t.Fatal(err)
	}
	jack, err := users.Push(context.Background(), map[string]interface{}{
		"name": "Jack Horner", "since": float64(1940),
	})
	if err != nil {
		t.Fatal(err)
	}
	delta := map[string]interface{}{
		fmt.Sprintf("%s/since", edward.Key): 1840,
		fmt.Sprintf("%s/since", jack.Key):   1946,
	}
	if err := users.Update(context.Background(), delta); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := edward.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"name": "Edward Cope", "since": float64(1840)}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}

	if err := jack.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	want = map[string]interface{}{"name": "Jack Horner", "since": float64(1946)}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestSetIfChanged(t *testing.T) {
	edward, err := users.Push(context.Background(), &User{"Edward Cope", 1800})
	if err != nil {
		t.Fatal(err)
	}

	update := User{"Jack Horner", 1940}
	ok, err := edward.SetIfUnchanged(context.Background(), "invalid-etag", &update)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("SetIfUnchanged() = %v; want = %v", ok, false)
	}

	var u User
	etag, err := edward.GetWithETag(context.Background(), &u)
	if err != nil {
		t.Fatal(err)
	}
	ok, err = edward.SetIfUnchanged(context.Background(), etag, &update)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("SetIfUnchanged() = %v; want = %v", ok, true)
	}

	if err := edward.Get(context.Background(), &u); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(update, u) {
		t.Errorf("Get() = %v; want = %v", u, update)
	}
}

func TestTransaction(t *testing.T) {
	u, err := users.Push(context.Background(), &User{Name: "Richard"})
	if err != nil {
		t.Fatal(err)
	}
	fn := func(t db.TransactionNode) (interface{}, error) {
		var user User
		if err := t.Unmarshal(&user); err != nil {
			return nil, err
		}
		user.Name = "Richard Owen"
		user.Since = 1804
		return &user, nil
	}
	if err := u.Transaction(context.Background(), fn); err != nil {
		t.Fatal(err)
	}
	var got User
	if err := u.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	want := User{"Richard Owen", 1804}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
}

func TestTransactionScalar(t *testing.T) {
	cnt := users.Child("count")
	if err := cnt.Set(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	fn := func(t db.TransactionNode) (interface{}, error) {
		var snap float64
		if err := t.Unmarshal(&snap); err != nil {
			return nil, err
		}
		return snap + 1, nil
	}
	if err := cnt.Transaction(context.Background(), fn); err != nil {
		t.Fatal(err)
	}
	var got float64
	if err := cnt.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if got != 43.0 {
		t.Errorf("Get() = %v; want = %v", got, 43.0)
	}
}

func TestDelete(t *testing.T) {
	u, err := users.Push(context.Background(), "foo")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := u.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if got != "foo" {
		t.Errorf("Get() = %q; want = %q", got, "foo")
	}
	if err := u.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}

	var got2 string
	if err := u.Get(context.Background(), &got2); err != nil {
		t.Fatal(err)
	}
	if got2 != "" {
		t.Errorf("Get() = %q; want = %q", got2, "")
	}
}

func TestNoAccess(t *testing.T) {
	r := aoClient.NewRef(protectedRef(t, "_adminsdk/go/admin"))
	var got string
	if err := r.Get(context.Background(), &got); err == nil || got != "" {
		t.Errorf("Get() = (%q, %v); want = (empty, error)", got, err)
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}
	if err := r.Set(context.Background(), "update"); err == nil {
		t.Errorf("Set() = nil; want = error")
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}
}

func TestReadAccess(t *testing.T) {
	r := aoClient.NewRef(protectedRef(t, "_adminsdk/go/protected/user2"))
	var got string
	if err := r.Get(context.Background(), &got); err != nil || got != "test" {
		t.Errorf("Get() = (%q, %v); want = (%q, nil)", got, err, "test")
	}
	if err := r.Set(context.Background(), "update"); err == nil {
		t.Errorf("Set() = nil; want = error")
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}
}

func TestReadWriteAccess(t *testing.T) {
	r := aoClient.NewRef(protectedRef(t, "_adminsdk/go/protected/user1"))
	var got string
	if err := r.Get(context.Background(), &got); err != nil || got != "test" {
		t.Errorf("Get() = (%q, %v); want = (%q, nil)", got, err, "test")
	}
	if err := r.Set(context.Background(), "update"); err != nil {
		t.Errorf("Set() = %v; want = nil", err)
	}
}

func TestQueryAccess(t *testing.T) {
	r := aoClient.NewRef("_adminsdk/go/protected")
	got := make(map[string]interface{})
	if err := r.OrderByKey().LimitToFirst(2).Get(context.Background(), &got); err == nil {
		t.Errorf("OrderByQuery() = nil; want = error")
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}
}

func TestGuestAccess(t *testing.T) {
	r := guestClient.NewRef(protectedRef(t, "_adminsdk/go/public"))
	var got string
	if err := r.Get(context.Background(), &got); err != nil || got != "test" {
		t.Errorf("Get() = (%q, %v); want = (%q, nil)", got, err, "test")
	}
	if err := r.Set(context.Background(), "update"); err == nil {
		t.Errorf("Set() = nil; want = error")
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}

	got = ""
	r = guestClient.NewRef("_adminsdk/go")
	if err := r.Get(context.Background(), &got); err == nil || got != "" {
		t.Errorf("Get() = (%q, %v); want = (empty, error)", got, err)
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}

	c := r.Child("protected/user2")
	if err := c.Get(context.Background(), &got); err == nil || got != "" {
		t.Errorf("Get() = (%q, %v); want = (empty, error)", got, err)
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}

	c = r.Child("admin")
	if err := c.Get(context.Background(), &got); err == nil || got != "" {
		t.Errorf("Get() = (%q, %v); want = (empty, error)", got, err)
	} else if err.Error() != permDenied {
		t.Errorf("Error = %q; want = %q", err.Error(), permDenied)
	}
}

func TestWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var m map[string]interface{}
	if err := ref.Get(ctx, &m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData, m) {
		t.Errorf("Get() = %v; want = %v", m, testData)
	}

	cancel()
	m = nil
	if err := ref.Get(ctx, &m); len(m) != 0 || err == nil {
		t.Errorf("Get() = (%v, %v); want = (empty, error)", m, err)
	}
}

func protectedRef(t *testing.T, p string) string {
	r := client.NewRef(p)
	if err := r.Set(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	return p
}

type Dinosaur struct {
	Appeared float64 `json:"appeared"`
	Height   float64 `json:"height"`
	Length   float64 `json:"length"`
	Order    string  `json:"order"`
	Vanished float64 `json:"vanished"`
	Weight   int     `json:"weight"`
	Ratings  Ratings `json:"ratings"`
}

type Ratings struct {
	Pos int `json:"pos"`
}

type User struct {
	Name  string `json:"name"`
	Since int    `json:"since"`
}
