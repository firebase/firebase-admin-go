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

// Package firebase is the entry point to the Firebase Admin SDK.
// It provides functionality for initializing App instances using the app package.
package firebase

import (
	"context"
	// We will need to import the new app package
	"firebase.google.com/go/v4/app"
	// Other imports like option might still be needed if NewApp signature keeps them.
	"google.golang.org/api/option"
	// internal.FirebaseScopes was used by the old NewApp. app.New now handles this.
)

// Version of the Firebase Go Admin SDK.
const Version = "4.16.1"

// Config is an alias to app.Config for convenience.
// Users can import app.Config directly, but this provides a smoother transition for existing code.
type Config = app.Config

// NewApp creates a new default App instance.
//
// This function is a convenience wrapper around app.New().
// It initializes the default Firebase app. For named apps, or more control,
// use the app package directly.
//
// If the client options contain a valid credential (a service account file, a refresh token
// file or an oauth2.TokenSource) the App will be authenticated using that credential. Otherwise,
// NewApp attempts to authenticate the App with Google application default credentials.
// If `config` is nil, the SDK will attempt to load the config options from the
// `FIREBASE_CONFIG` environment variable.
func NewApp(ctx context.Context, config *Config, opts ...option.ClientOption) (*app.App, error) {
	// The app.New function now handles the logic of default scopes, config loading etc.
	return app.New(ctx, config, opts...)
}

// TODO: Future considerations:
// - Global app registry (appMap, GetApp, DeleteApp, NewAppWithName)
//   If this functionality is to be preserved, it would live in this firebase package,
//   managing instances of app.App. For now, this is simplified to just NewApp for the default app.
// - The original firebase.App struct also had methods like Auth(), Database(), etc.
//   These are intentionally removed as per the modularization goal. Users will now
//   create service clients directly, e.g., auth.NewClient(ctx, appInstance).
// - The firebaseEnvName and defaultAuthOverrides constants were part of the logic
//   moved to app/app.go.
// - The functions getConfigDefaults and getProjectID were moved to app/app.go.
// - Firestore, AppCheck, etc. client creations were methods on the old firebase.App.
//   These services will also follow the new pattern: servicePkg.NewClient(ctx, appInstance).
// - The imports for "cloud.google.com/go/firestore", "firebase.google.com/go/v4/auth", etc.
//   are no longer needed in this file as firebase.App no longer provides direct service client methods.
//   The `internal` import is also removed as its usage was tied to the old App methods.
//   The `app` package now handles its own dependencies, which might include `internal`.
// - The `firebase.App` type itself is removed. Functions now return `*app.App`.
//   If a distinct `firebase.App` type is needed (e.g. for app management), it would
//   be a new struct, likely embedding or holding an `*app.App`.
//   For this step, we assume `firebase.NewApp` returns `*app.App` directly.
// - Consider if `firebase.Option` (if it existed) should alias `app.Option` or `option.ClientOption`.
//   The `opts ...option.ClientOption` is from `google.golang.org/api/option`.
//   `app.Config` is the configuration struct.
//   The user's example `opt := option.WithCredentialsFile(...)` uses `google.golang.org/api/option`.
//   So `firebase.Config` aliasing `app.Config` seems correct.
