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
	"os"
	"testing"

	"firebase.google.com/go/v4/internal"
	"google.golang.org/api/option"
)

var opts = []option.ClientOption{
	option.WithCredentialsFile("../testdata/service_account.json"),
}

func TestNewClientError(t *testing.T) {
	invalid := []option.ClientOption{
		option.WithCredentialsFile("../testdata/non_existing.json"),
	}
	client, err := NewClient(context.Background(), &internal.StorageConfig{
		Opts: invalid,
	})
	if client != nil || err == nil {
		t.Errorf("NewClient() = (%v, %v); want (nil, error)", client, err)
	}
}

func TestNewClientEmulatorHostEnvVar(t *testing.T) {
	emulatorHost := "localhost:9099"
	os.Setenv("FIREBASE_STORAGE_EMULATOR_HOST", emulatorHost)
	defer os.Unsetenv("FIREBASE_STORAGE_EMULATOR_HOST")
	os.Unsetenv("STORAGE_EMULATOR_HOST")
	defer os.Unsetenv("STORAGE_EMULATOR_HOST")

	_, err := NewClient(context.Background(), &internal.StorageConfig{
		Opts: opts,
	})
	if err != nil {
		t.Fatal(err)
	}

	if host := os.Getenv("STORAGE_EMULATOR_HOST"); host != emulatorHost {
		t.Errorf("emulator host: %q; want: %q", host, emulatorHost)
	}
}

func TestNoBucketName(t *testing.T) {
	client, err := NewClient(context.Background(), &internal.StorageConfig{
		Opts: opts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.DefaultBucket(); err == nil {
		t.Errorf("DefaultBucket() = nil; want error")
	}
}

func TestEmptyBucketName(t *testing.T) {
	client, err := NewClient(context.Background(), &internal.StorageConfig{
		Opts: opts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Bucket(""); err == nil {
		t.Errorf("Bucket('') = nil; want error")
	}
}

func TestDefaultBucket(t *testing.T) {
	client, err := NewClient(context.Background(), &internal.StorageConfig{
		Bucket: "bucket.name",
		Opts:   opts,
	})
	if err != nil {
		t.Fatal(err)
	}
	bucket, err := client.DefaultBucket()
	if bucket == nil || err != nil {
		t.Errorf("DefaultBucket() = (%v, %v); want: (bucket, nil)", bucket, err)
	}
}

func TestBucket(t *testing.T) {
	client, err := NewClient(context.Background(), &internal.StorageConfig{
		Opts: opts,
	})
	if err != nil {
		t.Fatal(err)
	}
	bucket, err := client.Bucket("bucket.name")
	if bucket == nil || err != nil {
		t.Errorf("Bucket() = (%v, %v); want: (bucket, nil)", bucket, err)
	}
}
