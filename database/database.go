// Package database provides admin access to the Firebase Realtime Database.
package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"golang.org/x/net/context"

	"github.com/firebase/firebase-admin-go/internal"
)

// Database provides methods for accessing the Firebase Realtime Database.
type Database struct {
	hc  *http.Client
	url *url.URL
}

// New creates a new Firebase Realtime Database client.
func New(config *internal.DatabaseConfig) *Database {
	return &Database{
		hc:  config.Client,
		url: config.URL,
	}
}

// Get fetches a value from the realtime database at the specified path.
func (d *Database) Get(ctx context.Context, path string, response interface{}) error {
	return d.request(ctx, path, http.MethodGet, nil, response)
}

// Set PUTs the contents of value at the specified path.
func (d *Database) Set(ctx context.Context, path string, value, response interface{}) error {
	b := &bytes.Buffer{}
	if err := json.NewEncoder(b).Encode(value); err != nil {
		return fmt.Errorf("encoding value: %v", err)
	}
	return d.request(ctx, path, http.MethodPut, b, response)
}

// Update performs a PATCH with the value at the specified path.
func (d *Database) Update(ctx context.Context, path string, value, response interface{}) error {
	b := &bytes.Buffer{}
	if err := json.NewEncoder(b).Encode(value); err != nil {
		return fmt.Errorf("encoding value: %v", err)
	}
	return d.request(ctx, path, http.MethodPatch, b, response)
}

// Push performs a POST with the value at the specified path.
func (d *Database) Push(ctx context.Context, path string, value, response interface{}) error {
	b := &bytes.Buffer{}
	if err := json.NewEncoder(b).Encode(value); err != nil {
		return fmt.Errorf("encoding value: %v", err)
	}
	return d.request(ctx, path, http.MethodPost, b, response)
}

// Delete removes the data at the specified path.
func (d *Database) Delete(ctx context.Context, path string) error {
	return d.request(ctx, path, http.MethodDelete, nil, nil)
}

func (d *Database) request(ctx context.Context, path, method string, value io.Reader, response interface{}) error {
	url, err := d.fullURL(path)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(method, url, value)
	if err != nil {
		return fmt.Errorf("generating request: %v", err)
	}

	resp, err := d.hc.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("performing request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		if reflect.TypeOf(response) != nil {
			if err = json.NewDecoder(resp.Body).Decode(response); err != nil {
				return fmt.Errorf("decoding JSON response: %v", err)
			}
		}
		return nil
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v, status code: %d", err, resp.StatusCode)
	}
	return fmt.Errorf("making request: response body: %q, status code: %d", resp.StatusCode, body)
}

func (d *Database) fullURL(path string) (string, error) {
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf(`database path must start with a "/"`)
	}
	if d.url.Scheme == "" || d.url.Host == "" {
		return "", fmt.Errorf("invalid database URL: %q", d.url.String())
	}
	if !strings.HasSuffix(path, ".json") {
		path += ".json"
	}
	u := url.URL{
		Scheme: d.url.Scheme,
		Host:   d.url.Host,
		Path:   path,
	}
	return u.String(), nil
}
