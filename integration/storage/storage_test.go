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

package storage

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	gcs "cloud.google.com/go/storage"
	"firebase.google.com/go"
	"firebase.google.com/go/integration/internal"
	"firebase.google.com/go/storage"
)

var ctx context.Context
var client *storage.Client

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		log.Println("skipping storage integration tests in short mode.")
		os.Exit(0)
	}

	pid, err := internal.ProjectID()
	if err != nil {
		log.Fatalln(err)
	}

	ctx = context.Background()
	app, err := internal.NewTestApp(ctx, &firebase.Config{
		StorageBucket: fmt.Sprintf("%s.appspot.com", pid),
	})
	if err != nil {
		log.Fatalln(err)
	}

	client, err = app.Storage(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	os.Exit(m.Run())
}

func TestDefaultBucket(t *testing.T) {
	bucket, err := client.DefaultBucket()
	if bucket == nil || err != nil {
		t.Errorf("DefaultBucket() = (%v, %v); want (bucket, nil)", bucket, err)
	}
	if err := verifyBucket(bucket); err != nil {
		t.Fatal(err)
	}
}

func TestCustomBucket(t *testing.T) {
	pid, err := internal.ProjectID()
	if err != nil {
		t.Fatal(err)
	}

	bucket, err := client.Bucket(pid + ".appspot.com")
	if bucket == nil || err != nil {
		t.Errorf("Bucket() = (%v, %v); want (bucket, nil)", bucket, err)
	}
	if err := verifyBucket(bucket); err != nil {
		t.Fatal(err)
	}
}

func TestNonExistingBucket(t *testing.T) {
	bucket, err := client.Bucket("non-existing")
	if bucket == nil || err != nil {
		t.Errorf("Bucket() = (%v, %v); want (bucket, nil)", bucket, err)
	}
	if _, err := bucket.Attrs(context.Background()); err == nil {
		t.Errorf("bucket.Attr() = nil; want error")
	}
}

func verifyBucket(bucket *gcs.BucketHandle) error {
	const expected = "Hello World"

	// Create new object
	o := bucket.Object("data")
	w := o.NewWriter(ctx)
	w.ContentType = "text/plain"
	if _, err := w.Write([]byte(expected)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	// Read the created object
	r, err := o.NewReader(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	if string(b) != expected {
		return fmt.Errorf("fetched content: %q; want: %q", string(b), expected)
	}

	// Delete the object
	return o.Delete(ctx)
}
