// Copyright 2018 Google Inc. All Rights Reserved.
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
	"testing"
)

func TestAuthOverrideGet(t *testing.T) {
	mock := &mockServer{Resp: "data"}
	srv := mock.Start(aoClient)
	defer srv.Close()

	ref := aoClient.NewRef("peter")
	var got string
	if err := ref.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if got != "data" {
		t.Errorf("Ref(AuthOverride).Get() = %q; want = %q", got, "data")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"auth_variable_override": testAuthOverrides},
	})
}

func TestAuthOverrideSet(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(aoClient)
	defer srv.Close()

	ref := aoClient.NewRef("peter")
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	if err := ref.Set(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PUT",
		Body:   serialize(want),
		Path:   "/peter.json",
		Query:  map[string]string{"auth_variable_override": testAuthOverrides, "print": "silent"},
	})
}

func TestAuthOverrideQuery(t *testing.T) {
	mock := &mockServer{Resp: "data"}
	srv := mock.Start(aoClient)
	defer srv.Close()

	ref := aoClient.NewRef("peter")
	var got string
	if err := ref.OrderByChild("foo").Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if got != "data" {
		t.Errorf("Ref(AuthOverride).OrderByChild() = %q; want = %q", got, "data")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query: map[string]string{
			"auth_variable_override": testAuthOverrides,
			"orderBy":                "\"foo\"",
		},
	})
}

func TestAuthOverrideRangeQuery(t *testing.T) {
	mock := &mockServer{Resp: "data"}
	srv := mock.Start(aoClient)
	defer srv.Close()

	ref := aoClient.NewRef("peter")
	var got string
	if err := ref.OrderByChild("foo").StartAt(1).EndAt(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if got != "data" {
		t.Errorf("Ref(AuthOverride).OrderByChild() = %q; want = %q", got, "data")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query: map[string]string{
			"auth_variable_override": testAuthOverrides,
			"orderBy":                "\"foo\"",
			"startAt":                "1",
			"endAt":                  "10",
		},
	})
}
