package storage

import (
	"testing"

	"firebase.google.com/go/internal"
	"golang.org/x/net/context"
)

func TestNoBucketName(t *testing.T) {
	client, err := NewClient(&internal.StorageConfig{
		Ctx: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.DefaultBucket(); err == nil {
		t.Errorf("DefaultBucket() = nil; want error")
	}
}

func TestEmptyBucketName(t *testing.T) {
	client, err := NewClient(&internal.StorageConfig{
		Ctx: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Bucket(""); err == nil {
		t.Errorf("Bucket('') = nil; want error")
	}
}

func TestDefaultBucket(t *testing.T) {
	client, err := NewClient(&internal.StorageConfig{
		Ctx:    context.Background(),
		Bucket: "bucket.name",
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
	client, err := NewClient(&internal.StorageConfig{
		Ctx: context.Background(),
	})
	if err != nil {
		t.Fatal(err)
	}
	bucket, err := client.Bucket("bucket.name")
	if bucket == nil || err != nil {
		t.Errorf("Bucket() = (%v, %v); want: (bucket, nil)", bucket, err)
	}
}
