// Package admin is the entry point to the Firebase Admin SDK. It provides functionality for initializing and managing
// App instances, which serve as central entities that provide access to various other Firebase services exposed from
// the SDK.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/firebase/firebase-admin-go/auth"
	"github.com/firebase/firebase-admin-go/internal"

	"io/ioutil"

	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/option"
)

var firebaseScopes = []string{
	"https://www.googleapis.com/auth/firebase",
	"https://www.googleapis.com/auth/userinfo.email",
}

// An App holds configuration and state common to all Firebase services that are exposed from the SDK.
type App struct {
	ctx       context.Context
	jwtConf   *jwt.Config
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
		Config:    a.jwtConf,
		ProjectID: a.projectID,
	}
	return auth.NewClient(conf)
}

// AppFromServiceAcctFile creates a new App from the provided service account JSON file.
//
// This calls AppFromServiceAcct internally, with the JSON bytes read from the specified file.
// Service account JSON files (also known as service account private keys) can be downloaded from the
// "Settings" tab of a Firebase project in the Firebase console (https://console.firebase.google.com). See
// https://firebase.google.com/docs/admin/setup for code samples and detailed documentation.
func AppFromServiceAcctFile(ctx context.Context, config *Config, file string) (*App, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return AppFromServiceAcct(ctx, config, b)
}

// AppFromServiceAcct creates a new App from the provided service account JSON.
//
// This can be used when the service account JSON is not loaded from the local file system, but is read
// from some other source.
func AppFromServiceAcct(ctx context.Context, config *Config, bytes []byte) (*App, error) {
	if config == nil {
		config = &Config{}
	}

	jc, err := google.JWTConfigFromJSON(bytes, firebaseScopes...)
	if err != nil {
		return nil, err
	}
	if jc.Email == "" {
		return nil, errors.New("'client_email' field not available")
	} else if jc.TokenURL == "" {
		return nil, errors.New("'token_uri' field not available")
	} else if jc.PrivateKey == nil {
		return nil, errors.New("'private_key' field not available")
	} else if jc.PrivateKeyID == "" {
		return nil, errors.New("'private_key_id' field not available")
	}

	pid := config.ProjectID
	if pid == "" {
		s := &struct {
			ProjectID string `json:"project_id"`
		}{}
		if err := json.Unmarshal(bytes, s); err != nil {
			return nil, err
		}
		pid = projectID(s.ProjectID)
	}

	opts := []option.ClientOption{
		option.WithScopes(firebaseScopes...),
		option.WithTokenSource(jc.TokenSource(ctx)),
	}
	return &App{
		ctx:       ctx,
		jwtConf:   jc,
		projectID: pid,
		opts:      opts,
	}, nil
}

// AppFromRefreshTokenFile creates a new App from the provided refresh token JSON file.
//
// The JSON file must contain refresh_token, client_id and client_secret fields in addition to a type
// field set to the value "authorized_user". These files are usually created and managed by the Google Cloud SDK.
// This function calls AppFromRefreshToken internally, with the JSON bytes read from the specified file.
func AppFromRefreshTokenFile(ctx context.Context, config *Config, file string) (*App, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return AppFromRefreshToken(ctx, config, b)
}

// AppFromRefreshToken creates a new App from the provided refresh token JSON.
//
// The refresh token JSON must contain refresh_token, client_id and client_secret fields in addition to a type
// field set to the value "authorized_user".
func AppFromRefreshToken(ctx context.Context, config *Config, bytes []byte) (*App, error) {
	if config == nil {
		config = &Config{}
	}
	rt := &struct {
		Type         string `json:"type"`
		ClientSecret string `json:"client_secret"`
		ClientID     string `json:"client_id"`
		RefreshToken string `json:"refresh_token"`
	}{}
	if err := json.Unmarshal(bytes, rt); err != nil {
		return nil, err
	}
	if rt.Type != "authorized_user" {
		return nil, fmt.Errorf("'type' field is %q (expected %q)", rt.Type, "authorized_user")
	} else if rt.ClientID == "" {
		return nil, fmt.Errorf("'client_id' field not available")
	} else if rt.ClientSecret == "" {
		return nil, fmt.Errorf("'client_secret' field not available")
	} else if rt.RefreshToken == "" {
		return nil, fmt.Errorf("'refresh_token' field not available")
	}
	oc := &oauth2.Config{
		ClientID:     rt.ClientID,
		ClientSecret: rt.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       firebaseScopes,
	}
	token := &oauth2.Token{
		RefreshToken: rt.RefreshToken,
	}

	opts := []option.ClientOption{
		option.WithScopes(firebaseScopes...),
		option.WithTokenSource(oc.TokenSource(ctx, token)),
	}
	return &App{
		ctx:       ctx,
		projectID: projectID(config.ProjectID),
		opts:      opts,
	}, nil
}

// NewApp creates a new App based on the runtime environment.
//
// NewApp inspects the runtime environment to fetch a valid set of authentication credentials. This is
// particularly useful when deployed in a managed cloud environment such as Google App Engine or Google Compute Engine.
// Refer https://developers.google.com/identity/protocols/application-default-credentials for more details on how
// application default credentials work.
func NewApp(ctx context.Context, config *Config, opts ...option.ClientOption) (*App, error) {
	if config == nil {
		config = &Config{}
	}

	// TODO: Use creds.Get() when it's available.
	cred, err := google.FindDefaultCredentials(ctx, firebaseScopes...)
	if err != nil {
		return nil, err
	}

	pid := config.ProjectID
	o := []option.ClientOption{option.WithScopes(firebaseScopes...)}
	if cred != nil {
		if pid == "" {
			pid = projectID(cred.ProjectID)
		}

		o = append(o, option.WithTokenSource(cred.TokenSource))
	}
	o = append(o, opts...)

	var jc *jwt.Config
	// TODO: Needs changes from Chris to make the following work.
	// jc := cred.JWTConfig

	return &App{
		ctx:       ctx,
		jwtConf:   jc,
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
