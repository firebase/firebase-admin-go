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
	"context"
	"fmt"
	"net/http"
	"strings"
)

type Ref struct {
	Key  string
	Path string

	segs   []string
	client *Client
	ctx    context.Context
}

func (r *Ref) Parent() *Ref {
	l := len(r.segs)
	if l > 0 {
		path := strings.Join(r.segs[:l-1], "/")
		return r.client.NewRef(path)
	}
	return nil
}

func (r *Ref) Child(path string) *Ref {
	fp := fmt.Sprintf("%s/%s", r.Path, path)
	return r.client.NewRef(fp)
}

func (r *Ref) Get(v interface{}) error {
	resp, err := r.send("GET", nil)
	if err != nil {
		return err
	}
	return resp.CheckAndParse(http.StatusOK, v)
}

func (r *Ref) WithContext(ctx context.Context) Query {
	r2 := new(Ref)
	*r2 = *r
	r2.ctx = ctx
	return r2
}

func (r *Ref) GetWithETag(v interface{}) (string, error) {
	resp, err := r.send("GET", nil, withHeader("X-Firebase-ETag", "true"))
	if err != nil {
		return "", err
	} else if err := resp.CheckAndParse(http.StatusOK, v); err != nil {
		return "", err
	}
	return resp.Header.Get("Etag"), nil
}

func (r *Ref) GetIfChanged(etag string, v interface{}) (bool, string, error) {
	resp, err := r.send("GET", nil, withHeader("If-None-Match", etag))
	if err != nil {
		return false, "", err
	} else if err := resp.CheckAndParse(http.StatusOK, v); err == nil {
		return true, resp.Header.Get("ETag"), nil
	} else if err := resp.CheckStatus(http.StatusNotModified); err != nil {
		return false, "", err
	}
	return false, etag, nil
}

func (r *Ref) Set(v interface{}) error {
	resp, err := r.send("PUT", v, withQueryParam("print", "silent"))
	if err != nil {
		return err
	}
	return resp.CheckStatus(http.StatusNoContent)
}

func (r *Ref) SetIfUnchanged(etag string, v interface{}) (bool, error) {
	resp, err := r.send("PUT", v, withHeader("If-Match", etag))
	if err != nil {
		return false, err
	} else if err := resp.CheckStatus(http.StatusOK); err == nil {
		return true, nil
	} else if err := resp.CheckStatus(http.StatusPreconditionFailed); err != nil {
		return false, err
	}
	return false, nil
}

func (r *Ref) Push(v interface{}) (*Ref, error) {
	if v == nil {
		v = ""
	}
	resp, err := r.send("POST", v)
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

func (r *Ref) Update(v map[string]interface{}) error {
	if len(v) == 0 {
		return fmt.Errorf("value argument must be a non-empty map")
	}
	resp, err := r.send("PATCH", v, withQueryParam("print", "silent"))
	if err != nil {
		return err
	}
	return resp.CheckStatus(http.StatusNoContent)
}

type UpdateFn func(interface{}) (interface{}, error)

func (r *Ref) Transaction(fn UpdateFn) error {
	var curr interface{}
	etag, err := r.GetWithETag(&curr)
	if err != nil {
		return err
	}

	for i := 0; i < 20; i++ {
		new, err := fn(curr)
		if err != nil {
			return err
		}
		resp, err := r.send("PUT", new, withHeader("If-Match", etag))
		if err := resp.CheckStatus(http.StatusOK); err == nil {
			return nil
		} else if err := resp.CheckAndParse(http.StatusPreconditionFailed, &curr); err != nil {
			return err
		}
		etag = resp.Header.Get("ETag")
	}
	return fmt.Errorf("transaction aborted after failed retries")
}

func (r *Ref) Delete() error {
	resp, err := r.send("DELETE", nil)
	if err != nil {
		return err
	}
	return resp.CheckStatus(http.StatusOK)
}

func (r *Ref) send(method string, body interface{}, opts ...httpOption) (*response, error) {
	if strings.ContainsAny(r.Path, invalidChars) {
		return nil, fmt.Errorf("invalid path with illegal characters: %q", r.Path)
	}
	if r.ctx != nil {
		opts = append([]httpOption{withContext(r.ctx)}, opts...)
	}
	return r.client.send(method, r.Path, body, opts...)
}
