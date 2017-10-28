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

package db

import (
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/context"
)

const txnRetries = 25

// Ref represents a node in the Firebase Realtime Database.
type Ref struct {
	Key  string
	Path string

	segs   []string
	client *Client
	ctx    context.Context
}

// Parent returns a reference to the parent of the current node.
//
// If the current reference points to the root of the database, Parent returns nil.
func (r *Ref) Parent() *Ref {
	l := len(r.segs)
	if l > 0 {
		path := strings.Join(r.segs[:l-1], "/")
		return r.client.NewRef(path)
	}
	return nil
}

// Child returns a reference to the specified child node.
func (r *Ref) Child(path string) *Ref {
	fp := fmt.Sprintf("%s/%s", r.Path, path)
	return r.client.NewRef(fp)
}

// Get retrieves the value at the current database location, and stores it in the value pointed to
// by v.
//
// Data deserialization is performed using https://golang.org/pkg/encoding/json/#Unmarshal, and
// therefore v has the same requirements as the json package. Specifically, it must be a pointer,
// and it must not be nil.
func (r *Ref) Get(v interface{}) error {
	resp, err := r.send("GET")
	if err != nil {
		return err
	}
	return resp.CheckAndParse(http.StatusOK, v)
}

// WithContext returns a shallow copy of this Ref with its context changed to ctx.
//
// The resulting Ref will use ctx for all subsequent RPC calls.
func (r *Ref) WithContext(ctx context.Context) *Ref {
	r2 := new(Ref)
	*r2 = *r
	r2.ctx = ctx
	return r2
}

// GetWithETag retrieves the value at the current database location, along with its ETag.
func (r *Ref) GetWithETag(v interface{}) (string, error) {
	resp, err := r.send("GET", withHeader("X-Firebase-ETag", "true"))
	if err != nil {
		return "", err
	} else if err := resp.CheckAndParse(http.StatusOK, v); err != nil {
		return "", err
	}
	return resp.Header.Get("Etag"), nil
}

// GetIfChanged retrieves the value and ETag of the current database location only if the specified
// ETag does not match.
//
// If the specified ETag does not match, returns true along with the latest ETag of the database
// location. The value of the database location will be stored in v just like a regular Get() call.
// If the etag matches, returns false along with the same ETag passed into the function. No data
// will be stored in v in this case.
func (r *Ref) GetIfChanged(etag string, v interface{}) (bool, string, error) {
	resp, err := r.send("GET", withHeader("If-None-Match", etag))
	if err != nil {
		return false, "", err
	} else if err := resp.CheckAndParse(http.StatusOK, v); err == nil {
		return true, resp.Header.Get("ETag"), nil
	} else if err := resp.CheckStatus(http.StatusNotModified); err != nil {
		return false, "", err
	}
	return false, etag, nil
}

// Set stores the value v in the current database node.
//
// Set uses https://golang.org/pkg/encoding/json/#Marshal to serialize values into JSON. Therefore
// v has the same requirements as the json package. Values like functions and channels cannot be
// saved into Realtime Database.
func (r *Ref) Set(v interface{}) error {
	resp, err := r.sendWithBody("PUT", v, withQueryParam("print", "silent"))
	if err != nil {
		return err
	}
	return resp.CheckStatus(http.StatusNoContent)
}

// SetIfUnchanged conditionally sets the data at this location to the given value.
//
// Sets the data at this location to v only if the specified ETag matches. Returns true if the
// value is written. Returns false if no changes are made to the database.
func (r *Ref) SetIfUnchanged(etag string, v interface{}) (bool, error) {
	resp, err := r.sendWithBody("PUT", v, withHeader("If-Match", etag))
	if err != nil {
		return false, err
	} else if err := resp.CheckStatus(http.StatusOK); err == nil {
		return true, nil
	} else if err := resp.CheckStatus(http.StatusPreconditionFailed); err != nil {
		return false, err
	}
	return false, nil
}

// Push creates a new child node at the current location, and returns a reference to it.
//
// If v is not nil, it will be set as the initial value of the new child node. If v is nil, the
// new child node will be created with empty string as the value.
func (r *Ref) Push(v interface{}) (*Ref, error) {
	if v == nil {
		v = ""
	}
	resp, err := r.sendWithBody("POST", v)
	if err != nil {
		return nil, err
	}
	var d struct {
		Name string `json:"name"`
	}
	if err := resp.CheckAndParse(http.StatusOK, &d); err != nil {
		return nil, err
	}
	return r.Child(d.Name), nil
}

// Update modifies the specified child keys of the current location to the provided values.
func (r *Ref) Update(v map[string]interface{}) error {
	if len(v) == 0 {
		return fmt.Errorf("value argument must be a non-empty map")
	}
	resp, err := r.sendWithBody("PATCH", v, withQueryParam("print", "silent"))
	if err != nil {
		return err
	}
	return resp.CheckStatus(http.StatusNoContent)
}

type UpdateFn func(interface{}) (interface{}, error)

// Transaction atomically modifies the data at this location.
//
// Unlike a normal Set(), which just overwrites the data regardless of its previous state,
// Transaction() is used to modify the existing value to a new value, ensuring there are no
// conflicts with other clients simultaneously writing to the same location.
//
// This is accomplished by passing an update function which is used to transform the current value
// of this reference into a new value. If another client writes to this location before the new
// value is successfully saved, the update function is called again with the new current value, and
// the write will be retried. In case of repeated failures, this method will retry the transaction up
// to 25 times before giving up and returning an error.
//
// The update function may also force an early abort by returning an error instead of returning a
// value.
func (r *Ref) Transaction(fn UpdateFn) error {
	var curr interface{}
	etag, err := r.GetWithETag(&curr)
	if err != nil {
		return err
	}

	for i := 0; i < txnRetries; i++ {
		new, err := fn(curr)
		if err != nil {
			return err
		}
		resp, err := r.sendWithBody("PUT", new, withHeader("If-Match", etag))
		if err != nil {
			return err
		} else if err := resp.CheckStatus(http.StatusOK); err == nil {
			return nil
		} else if err := resp.CheckAndParse(http.StatusPreconditionFailed, &curr); err != nil {
			return err
		}
		etag = resp.Header.Get("ETag")
	}
	return fmt.Errorf("transaction aborted after failed retries")
}

// Delete removes this node from the database.
func (r *Ref) Delete() error {
	resp, err := r.send("DELETE")
	if err != nil {
		return err
	}
	return resp.CheckStatus(http.StatusOK)
}

func (r *Ref) send(method string, opts ...httpOption) (*response, error) {
	return r.sendWithBody(method, nil, opts...)
}

func (r *Ref) sendWithBody(method string, body interface{}, opts ...httpOption) (*response, error) {
	req := &request{
		Method: method,
		Body:   body,
		Path:   r.Path,
		Opts:   opts,
	}
	return r.client.send(r.ctx, req)
}
