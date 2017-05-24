// Package admin provides an admin SDK for accessing Firebase features.
package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/firebase/firebase-admin-go/database"
	"github.com/firebase/firebase-admin-go/internal"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
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
	DatabaseURL string `json:"databaseURL"`
	ProjectID   string `json:"projectId"`
}

// App represents a Firebase App.
type App struct {
	hc    *http.Client
	c     *Config
	dbURL *url.URL
}

// NewApp creates a new Firebase App with the default config.
func NewApp(ctx context.Context, opts ...option.ClientOption) (*App, error) {
	config, err := DefaultConfig()
	if err != nil {
		return nil, err
	}
	return NewAppWithConfig(ctx, config, opts...)
}

// NewAppWithConfig creates a new Firebase App with the provided config.
func NewAppWithConfig(ctx context.Context, config *Config, opts ...option.ClientOption) (*App, error) {
	o := []option.ClientOption{
		option.WithScopes(ScopeReadWrite, "https://www.googleapis.com/auth/userinfo.email" /* we should get rid of this requirement soon */),
		option.WithUserAgent(userAgent),
	}
	opts = append(o, opts...)
	hc, _, err := transport.NewHTTPClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("dialing: %v", err)
	}
	u, err := url.Parse(config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %v", err)
	}
	return &App{
		hc:    hc,
		c:     config,
		dbURL: u,
	}, nil
}

// DefaultConfig fetches the default config from the environment.
func DefaultConfig() (*Config, error) {
	c := os.Getenv("FIREBASE_PROJECT")
	config := &Config{}

	if c == "" {
		return config, nil
	}

	if err := json.NewDecoder(strings.NewReader(c)).Decode(config); err != nil {
		return nil, fmt.Errorf("decoding config: %v", err)
	}

	return config, nil
}

// Client returns an http.Client for accessing Google APIs.
func (a *App) Client() *http.Client {
	return a.hc
}

// Database returns a new Database client for the default db.
func (a *App) Database() *database.Database {
	return a.databaseWithURL(a.dbURL)
}

// DatabaseWithURL returns a new Database client for the specified URL.
func (a *App) DatabaseWithURL(databaseURL string) (*database.Database, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %v", err)
	}
	return a.databaseWithURL(u), nil
}

func (a *App) databaseWithURL(u *url.URL) *database.Database {
	return database.New(&internal.DatabaseConfig{Client: a.hc, URL: u})
}
