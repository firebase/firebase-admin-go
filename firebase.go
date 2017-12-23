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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"

	"cloud.google.com/go/firestore"

	"firebase.google.com/go/auth"
	"firebase.google.com/go/iid"
	"firebase.google.com/go/internal"
	"firebase.google.com/go/storage"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

var firebaseScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/datastore",
	"https://www.googleapis.com/auth/devstorage.full_control",
	"https://www.googleapis.com/auth/firebase",
	"https://www.googleapis.com/auth/identitytoolkit",
	"https://www.googleapis.com/auth/userinfo.email",
}

// Version of the Firebase Go Admin SDK.
const Version = "2.2.1"

// firebaseEnvName is the name of the environment variable with the Config.
const firebaseEnvName = "FIREBASE_CONFIG"

// An App holds configuration and state common to all Firebase services that are exposed from the SDK.
type App struct {
	creds         *google.DefaultCredentials
	projectID     string
	storageBucket string
	opts          []option.ClientOption
}

var validConfigFieldNames = map[string]bool{
	"projectId":     true,
	"storageBucket": true,
}

// Config represents the configuration used to initialize an App.
type Config struct {
	ProjectID     string `json:"projectId"`
	StorageBucket string `json:"storageBucket"`
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

// NewApp creates a new App from the provided config and client options.
//
// If the client options contain a valid credential (a service account file, a refresh token file or an
// oauth2.TokenSource) the App will be authenticated using that credential. Otherwise, NewApp attempts to
// authenticate the App with Google application default credentials.
func NewApp(ctx context.Context, configOrig *Config, opts ...option.ClientOption) (*App, error) {
	o := []option.ClientOption{option.WithScopes(firebaseScopes...)}
	o = append(o, opts...)
	if configOrig == nil {
		configOrig = &Config{}
	}
	creds, err := transport.Creds(ctx, o...)
	if err != nil {
		return nil, err
	}
	config, err := amendConfigWithDefaults(configOrig)
	if err != nil {
		return nil, err
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
		creds:         creds,
		projectID:     pid,
		storageBucket: config.StorageBucket,
		opts:          o,
	}, nil
}

// amendConfigWithDefaults reads the default config file, defined by the FIREBASE_COFIG
// env variable, and uses those values where the config is missing values.
func amendConfigWithDefaults(config *Config) (*Config, error) {
	fbc := &Config{}
	confFileName := os.Getenv(firebaseEnvName)
	if confFileName == "" {
		return config, nil
	}
	dat, err := ioutil.ReadFile(confFileName)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(dat, fbc)
	if err != nil {
		return nil, err
	}
	jsonData := map[string]string{}
	json.Unmarshal(dat, &jsonData)
	// TODO: remove this after we support Go 1.10+, specifically "DisallowUnknownFields"
	//       https://github.com/golang/go/commit/2596a0c075aeddec571cd658f748ac7a712a2b69
	//  d := json.NewDecoder(bytes.NewReader(dat))
	//  d.DisallowUnknownFields()
	//  e := d.Decode(fbc)
	for k := range jsonData {
		if _, ok := validConfigFieldNames[k]; !ok {
			return nil, fmt.Errorf(`unexpected field %s in JSON config file`, k)
		}
	}
	if config != nil {
		updateConfig(config, fbc)
	}
	return fbc, nil
}

func updateConfig(source, target *Config) {
	s := reflect.ValueOf(source).Elem()
	t := reflect.ValueOf(target).Elem()
	for i := 0; i < s.NumField(); i++ {
		fsi := s.Field(i).Interface()
		ft := t.Field(i)
		switch fsi.(type) {
		case int:
			if fsi != 0 {
				ft.SetInt(int64(fsi.(int)))
			}
		case string:
			if fsi != "" {
				ft.SetString(fsi.(string))
			}
		default:
			panic("non implemented Config{} field type")
		}
	}
}
