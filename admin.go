// Package admin is the entry point to the Firebase Admin SDK. It provides functionality for initializing and managing
// App instances, which serve as central entities that provide access to various other Firebase services exposed from
// the SDK.
package admin

import (
	"context"

	"github.com/firebase/firebase-admin-go/auth"
	"github.com/firebase/firebase-admin-go/internal"

	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

var firebaseScopes = []string{
	"https://www.googleapis.com/auth/firebase",
	"https://www.googleapis.com/auth/userinfo.email",
}

// An App holds configuration and state common to all Firebase services that are exposed from the SDK.
type App struct {
	ctx       context.Context
	creds     *google.DefaultCredentials
	projectID string
	opts      []option.ClientOption
}

// Config represents the configuration used to initialize an App.
type Config struct {
	ProjectID string
}

// Auth returns an instance of auth.Client.
func (a *App) Auth() (*auth.Client, error) {
	conf := &internal.AuthConfig{
		Creds:     a.creds,
		ProjectID: a.projectID,
	}
	return auth.NewClient(conf)
}

// NewApp creates a new App from the provided config and options.
//
// If the client options contain a credential file (a service account file or a refresh token
// file), the App will be authenticated using that credential. Otherwise, NewApp inspects the
// runtime environment to fetch Google application default credentials.
func NewApp(ctx context.Context, config *Config, opts ...option.ClientOption) (*App, error) {
	o := []option.ClientOption{option.WithScopes(firebaseScopes...)}
	o = append(o, opts...)

	creds, err := transport.Creds(ctx, o...)
	if err != nil {
		return nil, err
	}

	var pid string
	if config != nil && config.ProjectID != "" {
		pid = config.ProjectID
	} else {
		pid = projectID(creds.ProjectID)
	}

	return &App{
		ctx:       ctx,
		creds:     creds,
		projectID: pid,
		opts:      o,
	}, nil
}

func projectID(def string) string {
	if def == "" {
		return os.Getenv("GCLOUD_PROJECT")
	}
	return def
}
