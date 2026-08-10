// Copyright 2017 Google LLC All Rights Reserved.
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

package firestore

import (
	"context"
	"flag"
	"log"
	"os"
	"reflect"
	"testing"

	"firebase.google.com/go/v4/integration/internal"
)

const testDatabaseID = "testing-database"

var (
	cityData = map[string]interface{}{
		"name":       "Mountain View",
		"country":    "USA",
		"population": int64(77846),
		"capital":    false,
	}
	movieData = map[string]interface{}{
		"Name":                 "Interstellar",
		"Year":                 int64(2014),
		"Runtime":              "2h 49m",
		"Academy Award Winner": true,
	}
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping Firestore integration tests in short mode.")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestFirestore(t *testing.T) {
	ctx := context.Background()
	app, err := internal.NewTestApp(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	client, err := app.Firestore(ctx)
	if err != nil {
		t.Fatal(err)
	}

	doc := client.Collection("cities").Doc("Mountain View")
	if _, err := doc.Set(ctx, cityData); err != nil {
		t.Fatal(err)
	}
	defer doc.Delete(ctx)

	snap, err := doc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snap.Data(), cityData) {
		t.Errorf("Get() = %v; want %v", snap.Data(), cityData)
	}
}

func TestFirestoreWithDatabaseID(t *testing.T) {
	ctx := context.Background()
	app, err := internal.NewTestApp(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	// This test requires the target non-default database to exist in the project.
	// If it doesn't exist, this test will fail.
	client, err := app.FirestoreWithDatabaseID(ctx, testDatabaseID)
	if err != nil {
		t.Fatal(err)
	}

	doc := client.Collection("cities").NewDoc()
	if _, err := doc.Set(ctx, cityData); err != nil {
		t.Fatal(err)
	}
	defer doc.Delete(ctx)

	snap, err := doc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snap.Data(), cityData) {
		t.Errorf("Get() = %v; want %v", snap.Data(), cityData)
	}
}

func TestFirestoreMultiDB(t *testing.T) {
	ctx := context.Background()
	app, err := internal.NewTestApp(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	cityClient, err := app.Firestore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// This test requires the target non-default database to exist in the project.
	// If it doesn't exist, this test will fail.
	movieClient, err := app.FirestoreWithDatabaseID(ctx, testDatabaseID)
	if err != nil {
		t.Fatal(err)
	}

	cityDoc := cityClient.Collection("cities").NewDoc()
	movieDoc := movieClient.Collection("movies").NewDoc()

	if _, err := cityDoc.Set(ctx, cityData); err != nil {
		t.Fatal(err)
	}
	defer cityDoc.Delete(ctx)

	if _, err := movieDoc.Set(ctx, movieData); err != nil {
		t.Fatal(err)
	}
	defer movieDoc.Delete(ctx)

	citySnap, err := cityDoc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	movieSnap, err := movieDoc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(citySnap.Data(), cityData) {
		t.Errorf("City Get() = %v; want %v", citySnap.Data(), cityData)
	}
	if !reflect.DeepEqual(movieSnap.Data(), movieData) {
		t.Errorf("Movie Get() = %v; want %v", movieSnap.Data(), movieData)
	}
}
