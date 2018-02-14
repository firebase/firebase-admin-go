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
	"reflect"
	"testing"

	"golang.org/x/net/context"
)

var sortableResp = map[string]interface{}{
	"bob":     person{Name: "bob", Age: 20},
	"alice":   person{Name: "alice", Age: 30},
	"charlie": person{Name: "charlie", Age: 15},
	"dave":    person{Name: "dave", Age: 25},
	"ernie":   person{Name: "ernie"},
}

func TestChildQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	cases := []string{
		"messages", "messages/", "/messages",
	}
	var reqs []*testReq
	for _, tc := range cases {
		var got map[string]interface{}
		if err := testref.OrderByChild(tc).Get(context.Background(), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Errorf("OrderByChild(%q) = %v; want = %v", tc, got, want)
		}
		reqs = append(reqs, &testReq{
			Method: "GET",
			Path:   "/peter.json",
			Query:  map[string]string{"orderBy": "\"messages\""},
		})
	}

	checkAllRequests(t, mock.Reqs, reqs)
}

func TestNestedChildQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := testref.OrderByChild("messages/ratings").Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("OrderByChild(%q) = %v; want = %v", "messages/ratings", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"orderBy": "\"messages/ratings\""},
	})
}

func TestChildQueryWithParams(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q := testref.OrderByChild("messages").WithStartAt("m4").WithEndAt("m50").WithLimitToFirst(10)
	var got map[string]interface{}
	if err := q.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("OrderByChild() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query: map[string]string{
			"orderBy":      "\"messages\"",
			"startAt":      "\"m4\"",
			"endAt":        "\"m50\"",
			"limitToFirst": "10",
		},
	})
}

func TestInvalidOrderByChild(t *testing.T) {
	mock := &mockServer{Resp: "test"}
	srv := mock.Start(client)
	defer srv.Close()

	r := client.NewRef("/")
	cases := []string{
		"", "/", "foo$", "foo.", "foo#", "foo]",
		"foo[", "$key", "$value", "$priority",
	}
	for _, tc := range cases {
		var got string
		if err := r.OrderByChild(tc).Get(context.Background(), &got); got != "" || err == nil {
			t.Errorf("OrderByChild(%q) = (%q, %v); want = (%q, error)", tc, got, err, "")
		}
	}
	if len(mock.Reqs) != 0 {
		t.Errorf("OrderByChild() = %v; want = empty", mock.Reqs)
	}
}

func TestKeyQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := testref.OrderByKey().Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("OrderByKey() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"orderBy": "\"$key\""},
	})
}

func TestValueQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := testref.OrderByValue().Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("OrderByValue() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"orderBy": "\"$value\""},
	})
}

func TestLimitFirstQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := testref.OrderByChild("messages").WithLimitToFirst(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("query.WithLimitToFirst() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"limitToFirst": "10", "orderBy": "\"messages\""},
	})
}

func TestLimitLastQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := testref.OrderByChild("messages").WithLimitToLast(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("query.WithLimitToLast() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"limitToLast": "10", "orderBy": "\"messages\""},
	})
}

func TestInvalidLimitQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q := testref.OrderByChild("messages")
	cases := []struct {
		name string
		q    *Query
	}{
		{"BothLimits", q.WithLimitToFirst(10).WithLimitToLast(10)},
		{"NegativeFirst", q.WithLimitToFirst(-10)},
		{"NegativeLast", q.WithLimitToLast(-10)},
	}
	for _, tc := range cases {
		var got map[string]interface{}
		if err := tc.q.Get(context.Background(), &got); got != nil || err == nil {
			t.Errorf("OrderByChild(%q) = (%v, %v); want = (nil, error)", tc.name, got, err)
		}
		if len(mock.Reqs) != 0 {
			t.Errorf("OrderByChild(%q): %v; want: empty", tc.name, mock.Reqs)
		}
	}
}

func TestStartAtQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := testref.OrderByChild("messages").WithStartAt(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("WithStartAt() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"startAt": "10", "orderBy": "\"messages\""},
	})
}

func TestEndAtQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := testref.OrderByChild("messages").WithEndAt(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("WithEndAt() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"endAt": "10", "orderBy": "\"messages\""},
	})
}

func TestEqualToQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := testref.OrderByChild("messages").WithEqualTo(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("WithEqualTo() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"equalTo": "10", "orderBy": "\"messages\""},
	})
}

func TestInvalidFilterQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q := testref.OrderByChild("messages")
	cases := []struct {
		name string
		q    *Query
	}{
		{"InvalidStartAt", q.WithStartAt(func() {})},
		{"InvalidEndAt", q.WithEndAt(func() {})},
		{"InvalidEqualTo", q.WithEqualTo(func() {})},
	}
	for _, tc := range cases {
		var got map[string]interface{}
		if err := tc.q.Get(context.Background(), &got); got != nil || err == nil {
			t.Errorf("OrderByChild(%q) = (%v, %v); want = (nil, error)", tc.name, got, err)
		}
		if len(mock.Reqs) != 0 {
			t.Errorf("OrdderByChild(%q) = %v; want = empty", tc.name, mock.Reqs)
		}
	}
}

func TestAllParamsQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q := testref.OrderByChild("messages").WithLimitToFirst(100).WithStartAt("bar").WithEndAt("foo")
	var got map[string]interface{}
	if err := q.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("OrderByChild(AllParams) = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query: map[string]string{
			"limitToFirst": "100",
			"startAt":      "\"bar\"",
			"endAt":        "\"foo\"",
			"orderBy":      "\"messages\"",
		},
	})
}

func TestOrderedChildQuery(t *testing.T) {
	mock := &mockServer{Resp: sortableResp}
	srv := mock.Start(client)
	defer srv.Close()

	cases := []struct {
		child string
		want  []string
	}{
		{"age", []string{"ernie", "charlie", "bob", "dave", "alice"}},
		{"name", []string{"alice", "bob", "charlie", "dave", "ernie"}},
	}

	var reqs []*testReq
	for _, tc := range cases {
		var result []person
		if err := testref.OrderByChild(tc.child).GetOrdered(context.Background(), &result); err != nil {
			t.Fatal(err)
		}
		reqs = append(reqs, &testReq{
			Method: "GET",
			Path:   "/peter.json",
			Query:  map[string]string{"orderBy": fmt.Sprintf("%q", tc.child)},
		})

		var got []string
		for _, r := range result {
			got = append(got, r.Name)
		}
		if !reflect.DeepEqual(tc.want, got) {
			t.Errorf("GetOrdered(child: %q) = %v; want = %v", "age", got, tc.want)
		}
	}

	checkAllRequests(t, mock.Reqs, reqs)
}

func TestOrderedKeyQuery(t *testing.T) {
	mock := &mockServer{Resp: sortableResp}
	srv := mock.Start(client)
	defer srv.Close()

	var result []person
	if err := testref.OrderByKey().GetOrdered(context.Background(), &result); err != nil {
		t.Fatal(err)
	}
	req := &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"orderBy": "\"$key\""},
	}

	var got []string
	for _, r := range result {
		got = append(got, r.Name)
	}

	want := []string{"alice", "bob", "charlie", "dave", "ernie"}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("GetOrdered(child: %q) = %v; want = %v", "age", got, want)
	}

	checkOnlyRequest(t, mock.Reqs, req)
}
