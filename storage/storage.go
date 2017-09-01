package storage

import (
	"errors"

	"cloud.google.com/go/storage"
	"firebase.google.com/go/internal"
)

// Client is the interface for the Firebase Storage service.
type Client struct {
	client *storage.Client
	bucket string
}

// NewClient creates a new instance of the Firebase Storage Client.
//
// This function can only be invoked from within the SDK. Client applications should access the
// the Storage service through firebase.App.
func NewClient(c *internal.StorageConfig) (*Client, error) {
	client, err := storage.NewClient(c.Ctx, c.Opts...)
	if err != nil {
		return nil, err
	}
	return &Client{client: client, bucket: c.Bucket}, nil
}

// DefaultBucket returns a handle to the default Cloud Storage bucket.
//
// To use this method, the default bucket name must be specified via firebase.Config when
// initializing the App.
func (c *Client) DefaultBucket() (*storage.BucketHandle, error) {
	return c.Bucket(c.bucket)
}

// Bucket returns a handle to the specified Cloud Storage bucket.
func (c *Client) Bucket(name string) (*storage.BucketHandle, error) {
	if name == "" {
		return nil, errors.New("bucket name not specified")
	}
	return c.client.Bucket(name), nil
}
