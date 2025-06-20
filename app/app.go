// Copyright 2023 Google Inc. All Rights Reserved.
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

// Package app provides the core Firebase App type and initialization functionality.
// It is the entry point for initializing a Firebase application and serves as a
// container for common configuration and state shared across Firebase services.
//
// To initialize an app, use the New function. This returns an App instance
// which can then be passed to the NewClient functions of individual service packages
// (e.g., auth.NewClient, database.NewClient).
package app

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	// Import "firebase.google.com/go/v4/internal" once it's clear what's needed.
	// For now, direct imports will be adjusted as we refactor.
	"firebase.google.com/go/v4/internal" // Placeholder, will need adjustment
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

var defaultAuthOverrides = make(map[string]interface{})

// firebaseEnvName is the name of the environment variable that can be used to specify
// the path to a JSON file containing Firebase Config properties.
const firebaseEnvName = "FIREBASE_CONFIG"

// App represents a Firebase application instance. It holds configuration and state
// common to all Firebase services. An App instance is required to interact with
// any of the Firebase services.
type App struct {
	name             string // Name of the app, typically "[DEFAULT]" for the default app.
	projectID        string
	serviceAccountID string
	dbURL            string
	storageBucket    string
	authOverride     map[string]interface{}
	opts             []option.ClientOption // Combined client options for this app.
}

// Config represents the configuration used to initialize an App.
// These values are typically found in the service account JSON file.
type Config struct {
	// ProjectID is the Google Cloud Project ID.
	ProjectID string `json:"projectId"`
	// ServiceAccountID is the email address of the service account used for signing tokens.
	// This is only relevant if the SDK is not initialized with service account credentials.
	ServiceAccountID string `json:"serviceAccountId"`
	// DatabaseURL is the URL of the Firebase Realtime Database.
	DatabaseURL string `json:"databaseURL"`
	// StorageBucket is the name of the Google Cloud Storage bucket used for Firebase Storage.
	StorageBucket string `json:"storageBucket"`
	// AuthOverride is used to set the auth variable override for Realtime Database Rules.
	AuthOverride *map[string]interface{} `json:"databaseAuthVariableOverride"`
}

// New initializes a new Firebase App instance.
//
// New attempts to authenticate the App with Google application default credentials if no
// explicit credential options (e.g., option.WithCredentialsFile(), option.WithTokenSource())
// are provided in `opts`.
//
// If `config` is nil, New attempts to load configuration from the `FIREBASE_CONFIG`
// environment variable. If `FIREBASE_CONFIG` is set to a file path, the configuration is
// loaded from that file. If `FIREBASE_CONFIG` is set to a JSON string, it is parsed as
// the configuration.
//
// The returned App instance is thread-safe.
func New(ctx context.Context, config *Config, opts ...option.ClientOption) (*App, error) {
	// TODO: Consider how named apps (e.g., NewWithName) should be handled in this modular design.
	// The original SDK had a global app registry. For now, New creates a standalone App instance.

	// Prepend default Firebase scopes. User-provided scopes in opts will be appended.
	// Note: internal.FirebaseScopes will need to be accessible.
	// This might mean app needs to import internal, or FirebaseScopes is moved/duplicated.
	scopedOpts := []option.ClientOption{option.WithScopes(internal.FirebaseScopes...)}
	scopedOpts = append(scopedOpts, opts...)

	if config == nil {
		var err error
		if config, err = getConfigDefaults(); err != nil {
			return nil, err
		}
	}

	pid := getProjectID(ctx, config, scopedOpts...)
	ao := defaultAuthOverrides
	if config.AuthOverride != nil {
		ao = *config.AuthOverride
	}

	// The name "[DEFAULT]" is often used implicitly.
	// The original firebase.go `NewAppWithName` handles named apps.
	// For now, sticking to the user's example of `app.New(...)`
	return &App{
		name:             "[DEFAULT]", // Assuming default name for now
		authOverride:     ao,
		dbURL:            config.DatabaseURL,
		projectID:        pid,
		serviceAccountID: config.ServiceAccountID,
		storageBucket:    config.StorageBucket,
		opts:             scopedOpts,
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

func getProjectID(ctx context.Context, config *Config, opts ...option.ClientOption) string {
	if config.ProjectID != "" {
		return config.ProjectID
	}

	creds, _ := transport.Creds(ctx, opts...)
	if creds != nil && creds.ProjectID != "" {
		return creds.ProjectID
	}

	if pid := os.Getenv("GOOGLE_CLOUD_PROJECT"); pid != "" {
		return pid
	}

	return os.Getenv("GCLOUD_PROJECT")
}

// ProjectID returns the Project ID associated with this App.
// This value is inferred from the credentials or environment if not explicitly set in Config.
func (a *App) ProjectID() string {
	return a.projectID
}

// Name returns the name of this App. For an app initialized via `app.New()`,
// this is typically "[DEFAULT]". Named apps support might be added later.
func (a *App) Name() string {
	return a.name
}

// Options returns the `option.ClientOption`s used to initialize this App.
// These options include credentials and other transport configurations.
// Service clients use these options to make authenticated calls to Firebase services.
// The returned slice is a copy to prevent modification.
func (a *App) Options() []option.ClientOption {
	optsCopy := make([]option.ClientOption, len(a.opts))
	copy(optsCopy, a.opts)
	return optsCopy
}

// ServiceAccountID returns the service account ID associated with this App, if one was
// explicitly provided in the Config or discoverable through other means by the SDK.
// This is primarily used by the Auth service for token signing if full service account
// credentials are not available but a service account email is.
func (a *App) ServiceAccountID() string {
	return a.serviceAccountID
}

// DatabaseURL returns the Firebase Realtime Database URL associated with this App, if configured.
func (a *App) DatabaseURL() string {
	return a.dbURL
}

// StorageBucket returns the Google Cloud Storage bucket name for Firebase Storage, if configured.
func (a *App) StorageBucket() string {
	return a.storageBucket
}

// AuthOverride returns the database auth override settings for Realtime Database Rules, if configured.
// The returned map is a copy to prevent modification.
func (a *App) AuthOverride() map[string]interface{} {
	if a.authOverride == nil {
		return nil
	}
	overrideCopy := make(map[string]interface{})
	for k, v := range a.authOverride {
		overrideCopy[k] = v
	}
	return overrideCopy
}

// TODO: Consider adding a Delete() method for app lifecycle management,
// particularly if a global app registry is reintroduced.

// SDKVersion returns the version of the Firebase Admin Go SDK.
// This is used by service clients, for example, to set User-Agent headers.
func (a *App) SDKVersion() string {
	// This version should ideally be managed centrally, perhaps via build flags
	// or a const in the top-level `firebase` package that `app.App` can access.
	// For now, keeping it hardcoded as it was in the initial refactor.
	// The top-level `firebase.Version` constant can serve as the single source of truth.
	// This method ensures services can get it via the App instance.
	return "4.16.1" // TODO: Centralize this version string, possibly by referencing firebase.Version.
}
