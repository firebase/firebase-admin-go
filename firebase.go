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
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"cloud.google.com/go/firestore"

	"firebase.google.com/go/auth"
	"firebase.google.com/go/db"
	"firebase.google.com/go/iid"
	"firebase.google.com/go/internal"
	"firebase.google.com/go/messaging"
	"firebase.google.com/go/storage"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

var defaultAuthOverrides = make(map[string]interface{})

// Version of the Firebase Go Admin SDK.
const Version = "2.6.0"

// firebaseEnvName is the name of the environment variable with the Config.
const firebaseEnvName = "FIREBASE_CONFIG"

// An App holds configuration and state common to all Firebase services that are exposed from the SDK.
type App struct {
	authOverride  map[string]interface{}
	creds         *google.DefaultCredentials
	dbURL         string
	projectID     string
	storageBucket string
	opts          []option.ClientOption
}

// Config represents the configuration used to initialize an App.
type Config struct {
	AuthOverride  *map[string]interface{} `json:"databaseAuthVariableOverride"`
	DatabaseURL   string                  `json:"databaseURL"`
	ProjectID     string                  `json:"projectId"`
	StorageBucket string                  `json:"storageBucket"`
}

// Auth returns an instance of auth.Client.
func (a *App) Auth(ctx context.Context) (*auth.Client, error) {
	conf := &internal.AuthConfig{
		Creds:     a.creds,
		ProjectID: a.projectID,
		Opts:      a.opts,
		Version:   Version,
	}
	return auth.NewClient(ctx, conf)
}

// Database returns an instance of db.Client.
func (a *App) Database(ctx context.Context) (*db.Client, error) {
	conf := &internal.DatabaseConfig{
		AuthOverride: a.authOverride,
		URL:          a.dbURL,
		Opts:         a.opts,
		Version:      Version,
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

// Firestore returns a new firestore.Client instance from the https://godoc.org/cloud.google.com/go/firestore
// package.
func (a *App) Firestore(ctx context.Context) (*firestore.Client, error) {
	if a.projectID == "" {
		return nil, errors.New("project id is required to access Firestore")
	}
	return firestore.NewClient(ctx, a.projectID, a.opts...)
}

// InstanceID returns an instance of iid.Client.
func (a *App) InstanceID(ctx context.Context) (*iid.Client, error) {
	conf := &internal.InstanceIDConfig{
		ProjectID: a.projectID,
		Opts:      a.opts,
	}
	return iid.NewClient(ctx, conf)
}

// Messaging returns an instance of messaging.Client.
func (a *App) Messaging(ctx context.Context) (*messaging.Client, error) {
	conf := &internal.MessagingConfig{
		ProjectID: a.projectID,
		Opts:      a.opts,
		Version:   Version,
	}
	return messaging.NewClient(ctx, conf)
}

// NewApp creates a new App from the provided config and client options.
//
// If the client options contain a valid credential (a service account file, a refresh token
// file or an oauth2.TokenSource) the App will be authenticated using that credential. Otherwise,
// NewApp attempts to authenticate the App with Google application default credentials.
// If `config` is nil, the SDK will attempt to load the config options from the
// `FIREBASE_CONFIG` environment variable. If the value in it starts with a `{` it is parsed as a
// JSON object, otherwise it is assumed to be the name of the JSON file containing the options.
func NewApp(ctx context.Context, config *Config, opts ...option.ClientOption) (*App, error) {
	o := []option.ClientOption{option.WithScopes(internal.FirebaseScopes...)}
	o = append(o, opts...)
	creds, err := transport.Creds(ctx, o...)
	if err != nil {
		return nil, err
	}
	if config == nil {
		if config, err = getConfigDefaults(); err != nil {
			return nil, err
		}
	}

	var pid string
	if config.ProjectID != "" {
		pid = config.ProjectID
	} else if creds.ProjectID != "" {
		pid = creds.ProjectID
	} else {
		pid = os.Getenv("GCLOUD_PROJECT")
	}

	ao := defaultAuthOverrides
	if config.AuthOverride != nil {
		ao = *config.AuthOverride
	}

	return &App{
		authOverride:  ao,
		creds:         creds,
		dbURL:         config.DatabaseURL,
		projectID:     pid,
		storageBucket: config.StorageBucket,
		opts:          o,
	}, nil
}

// getConfigDefaults reads the default config file, defined by the FIREBASE_CONFIG
// env variable, used only when options are nil.
func getConfigDefaults() (*Config, error) {
	fbc := &Config{}
	confFileName := os.Getenv(firebaseEnvName)
	if confFileName == "" {
		return fbc, nil
	}
	var dat []byte
	if confFileName[0] == byte('{') {
		dat = []byte(confFileName)
	} else {
		var err error
		if dat, err = ioutil.ReadFile(confFileName); err != nil {
			return nil, err
		}
	}
	if err := json.Unmarshal(dat, fbc); err != nil {
		return nil, err
	}

	// Some special handling necessary for db auth overrides
	var m map[string]interface{}
	if err := json.Unmarshal(dat, &m); err != nil {
		return nil, err
	}
	if ao, ok := m["databaseAuthVariableOverride"]; ok && ao == nil {
		// Auth overrides are explicitly set to null
		var nullMap map[string]interface{}
		fbc.AuthOverride = &nullMap
	}
	return fbc, nil
}
