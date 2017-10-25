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

// Package firebase is the entry point to the Firebase Admin SDK. It provides functionality for initializing App
// instances, which serve as the central entities that provide access to various other Firebase services exposed
// from the SDK.
package firebase

import (
	"firebase.google.com/go/auth"
	"firebase.google.com/go/db"
	"firebase.google.com/go/internal"
	"firebase.google.com/go/storage"

	"os"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

// Version of the Firebase Go Admin SDK.
const Version = "2.0.0"

// An App holds configuration and state common to all Firebase services that are exposed from the SDK.
type App struct {
	authOverrides map[string]interface{}
	creds         *google.DefaultCredentials
	dbURL         string
	projectID     string
	storageBucket string
	opts          []option.ClientOption
}

// Config represents the configuration used to initialize an App.
type Config struct {
	AuthOverrides map[string]interface{}
	DatabaseURL   string
	ProjectID     string
	StorageBucket string
}

// Auth returns an instance of auth.Client.
func (a *App) Auth(ctx context.Context) (*auth.Client, error) {
	conf := &internal.AuthConfig{
		Creds:     a.creds,
		ProjectID: a.projectID,
		Opts:      a.opts,
	}
	return auth.NewClient(ctx, conf)
}

// Database returns an instance of db.Client.
func (a *App) Database(ctx context.Context) (*db.Client, error) {
	conf := &internal.DatabaseConfig{
		AuthOverrides: a.authOverrides,
		BaseURL:       a.dbURL,
		Opts:          a.opts,
		Version:       Version,
	}
	return db.NewClient(ctx, conf)
}

// Storage returns a new instance of storage.Client.
func (a *App) Storage(ctx context.Context) (*storage.Client, error) {
	conf := &internal.StorageConfig{
		Opts:   a.opts,
		Bucket: a.storageBucket,
	}
	return storage.NewClient(ctx, conf)
}

// NewApp creates a new App from the provided config and client options.
//
// If the client options contain a valid credential (a service account file, a refresh token file or an
// oauth2.TokenSource) the App will be authenticated using that credential. Otherwise, NewApp attempts to
// authenticate the App with Google application default credentials.
func NewApp(ctx context.Context, config *Config, opts ...option.ClientOption) (*App, error) {
	o := []option.ClientOption{option.WithScopes(internal.FirebaseScopes...)}
	o = append(o, opts...)

	creds, err := transport.Creds(ctx, o...)
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = &Config{}
	}

	var pid string
	if config.ProjectID != "" {
		pid = config.ProjectID
	} else if creds.ProjectID != "" {
		pid = creds.ProjectID
	} else {
		pid = os.Getenv("GCLOUD_PROJECT")
	}

	return &App{
		authOverrides: config.AuthOverrides,
		creds:         creds,
		dbURL:         config.DatabaseURL,
		projectID:     pid,
		storageBucket: config.StorageBucket,
		opts:          o,
	}, nil
}
