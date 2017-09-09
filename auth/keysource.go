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

package auth

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

// httpKeySource fetches RSA public keys from a remote HTTP server, and caches them in
// memory. It also handles cache! invalidation and refresh based on the standard HTTP
// cache-control headers.
type httpKeySource struct {
	KeyURI     string
	HTTPClient *http.Client
	CachedKeys []*publicKey
	ExpiryTime time.Time
	Clock      clock
	Mutex      *sync.Mutex
}

func newHTTPKeySource(ctx context.Context, uri string, opts ...option.ClientOption) (*httpKeySource, error) {
	var hc *http.Client
	if ctx != nil && len(opts) > 0 {
		var err error
		hc, _, err = transport.NewHTTPClient(ctx, opts...)
		if err != nil {
			return nil, err
		}
	} else {
		hc = http.DefaultClient
	}

	return &httpKeySource{
		KeyURI:     uri,
		HTTPClient: hc,
		Clock:      systemClock{},
		Mutex:      &sync.Mutex{},
	}, nil
}

// Keys returns the RSA Public Keys hosted at this key source's URI. Refreshes the data if
// the cache is stale.
func (k *httpKeySource) Keys() ([]*publicKey, error) {
	k.Mutex.Lock()
	defer k.Mutex.Unlock()
	if len(k.CachedKeys) == 0 || k.hasExpired() {
		err := k.refreshKeys()
		if err != nil && len(k.CachedKeys) == 0 {
			return nil, err
		}
	}
	return k.CachedKeys, nil
}

// hasExpired indicates whether the cache has expired.
func (k *httpKeySource) hasExpired() bool {
	return k.Clock.Now().After(k.ExpiryTime)
}

func (k *httpKeySource) refreshKeys() error {
	k.CachedKeys = nil
	resp, err := k.HTTPClient.Get(k.KeyURI)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	newKeys, err := parsePublicKeys(contents)
	if err != nil {
		return err
	}

	maxAge, err := findMaxAge(resp)
	if err != nil {
		return err
	}

	k.CachedKeys = append([]*publicKey(nil), newKeys...)
	k.ExpiryTime = k.Clock.Now().Add(*maxAge)
	return nil
}

func findMaxAge(resp *http.Response) (*time.Duration, error) {
	cc := resp.Header.Get("cache-control")
	for _, value := range strings.Split(cc, ", ") {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "max-age") {
			sep := strings.Index(value, "=")
			if sep == -1 {
				return nil, errors.New("Malformed cache-control header")
			}
			seconds, err := strconv.ParseInt(value[sep+1:], 10, 64)
			if err != nil {
				return nil, err
			}
			duration := time.Duration(seconds) * time.Second
			return &duration, nil
		}
	}
	return nil, errors.New("Could not find expiry time from HTTP headers")
}
