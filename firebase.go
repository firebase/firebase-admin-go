// Package firebase provides an admin SDK for accessing Firebase features.
package firebase

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/creds"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"

	"cloud.google.com/go/storage"

	"github.com/firebase/firebase-admin-go/auth"
	"github.com/firebase/firebase-admin-go/database"
	"github.com/firebase/firebase-admin-go/internal"
)

const (
	userAgent = "firebase-admin-go/20170523"

	// ScopeReadWrite allows full read/write and management access to the
	// realtime database.
	ScopeReadWrite = "https://www.googleapis.com/auth/firebase"

	// ScopeReadOnly allows read-only access to the realtime database.
	ScopeReadOnly = "https://www.googleapis.com/auth/firebase.readonly"
)

// Config defines the Firebase admin configuration options.
// This is serializable from the config JSON provided by the UI, and
// specifically does not contain auth credentials.
type Config struct {
	DefaultDatabaseURL string `json:"databaseURL"`
	ProjectID          string `json:"projectId"`
}

// App represents a Firebase App.
type App struct {
	ctx   context.Context
	hc    *http.Client
	c     *Config
	dbURL *url.URL
	opts  []option.ClientOption
	creds *google.DefaultCredentials
}

// DefaultApp creates a new Firebase App with the default config.
func DefaultApp(ctx context.Context, opts ...option.ClientOption) (*App, error) {
	config, err := DefaultConfig()
	if err != nil {
		return nil, err
	}
	return New(ctx, config, opts...)
}

// NewApp creates a new Firebase App with the provided config.
func NewApp(ctx context.Context, config *Config, opts ...option.ClientOption) (*App, error) {
	if config == nil {
		return nil, fmt.Errorf("invalid config: config must not be nil")
	}
	o := []option.ClientOption{
		option.WithScopes(ScopeReadWrite, "https://www.googleapis.com/auth/userinfo.email" /* we should get rid of this requirement soon */),
		option.WithUserAgent(userAgent),
	}
	o = append(o, opts...)
	hc, _, err := transport.NewHTTPClient(ctx, o...)
	if err != nil {
		return nil, fmt.Errorf("dialing: %v", err)
	}
	u, err := url.Parse(config.DefaultDatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %v", err)
	}
	c, err := creds.Get(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("fetching credentials: %v", err)
	}
	if c != nil && c.ProjectID != "" {
		config.ProjectID = c.ProjectID
	}
	return &App{
		hc:    hc,
		c:     config,
		dbURL: u,
		opts:  opts,
		creds: c,
	}, nil
}

// DefaultConfig fetches the default config from the environment.
func DefaultConfig() (*Config, error) {
	c := os.Getenv("FIREBASE_PROJECT")
	config := &Config{}

	if c != "" {
		if err := json.NewDecoder(strings.NewReader(c)).Decode(config); err != nil {
			return nil, fmt.Errorf("decoding config: %v", err)
		}
	}

	if config.ProjectID == "" {
		config.ProjectID = os.Getenv("GCLOUD_PROJECT")
	}

	return config, nil
}

// Auth returns a new Auth client.
func (a *App) Auth() *auth.Client {
	return auth.NewClient(&internal.AuthConfig{
		Client:    a.hc,
		ProjectID: a.c.ProjectID,
		Creds:     a.creds,
	})
}

// Database returns a new Database client for the default db.
func (a *App) Database() *database.Client {
	return a.databaseWithURL(a.dbURL)
}

// DatabaseWithURL returns a new Database client for the specified URL.
func (a *App) DatabaseWithURL(databaseURL string) (*database.Client, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %v", err)
	}
	return a.databaseWithURL(u), nil
}

func (a *App) databaseWithURL(u *url.URL) *database.Client {
	return database.NewClient(&internal.DatabaseConfig{Client: a.hc, URL: u})
}

// Storage returns a Cloud storage client.
func (a *App) Storage() (*storage.Client, error) {
	return storage.NewClient(a.ctx, a.opts...)
}
