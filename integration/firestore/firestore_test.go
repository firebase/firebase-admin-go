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

package firestore

import (
	"context"
	"log"
	"reflect"
	"testing"

	"firebase.google.com/go/integration/internal"
)

func TestFirestore(t *testing.T) {
	if testing.Short() {
		log.Println("skipping Firestore integration tests in short mode.")
		return
	}
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
	data := map[string]interface{}{
		"name":       "Mountain View",
		"country":    "USA",
		"population": int64(77846),
		"capital":    false,
	}
	if _, err := doc.Set(ctx, data); err != nil {
		t.Fatal(err)
	}
	snap, err := doc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snap.Data(), data) {
		t.Errorf("Get() = %v; want %v", snap.Data(), data)
	}
	if _, err := doc.Delete(ctx); err != nil {
		t.Fatal(err)
	}
}
