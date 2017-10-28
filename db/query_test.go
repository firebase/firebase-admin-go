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
	"reflect"
	"testing"

	"golang.org/x/net/context"
)

func TestQueryWithContext(t *testing.T) {
	q := client.NewRef("peter").OrderByChild("messages")
	if q.ctx != nil {
		t.Errorf("query = %v; want nil", q.ctx)
	}

	ctx, cancel := context.WithCancel(context.Background())
	q = q.WithContext(ctx)
	if q.ctx != ctx {
		t.Errorf("query.WithContext() = %v; want %v", q.ctx, ctx)
	}

	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("query.WithContext() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"orderBy": "\"messages\""},
	})

	cancel()
	got = nil
	if err := q.Get(&got); len(got) != 0 || err == nil {
		t.Errorf("query.WithContext() = (%v, %v); want = (empty, error)", got, err)
	}
}

func TestQueryFromRefWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q := client.NewRef("peter").WithContext(ctx).OrderByChild("messages")
	if q.ctx != ctx {
		t.Errorf("ref.WithContext().query = %v; want %v", q.ctx, ctx)
	}

	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("ref.WithContext().query = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"orderBy": "\"messages\""},
	})

	cancel()
	got = nil
	if err := q.Get(&got); len(got) != 0 || err == nil {
		t.Errorf("ref.WithContext().query = (%v, %v); want = (empty, error)", got, err)
	}
}

func TestQueryWithContextPrecedence(t *testing.T) {
	ctx1 := context.Background()
	ctx2, cancel := context.WithCancel(ctx1)

	r := client.NewRef("peter").WithContext(ctx1)
	q := r.OrderByChild("messages").WithContext(ctx2)
	if q.ctx != ctx2 {
		t.Errorf("Ctx = %v; want %v", q.ctx, ctx2)
	}

	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("ref.WithContext().query.WithContext() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"orderBy": "\"messages\""},
	})

	cancel()
	got = nil
	if err := q.Get(&got); len(got) != 0 || err == nil {
		t.Errorf("ref.WithContext().query.WithContext() = (%v, %v); want = (empty, error)", got, err)
	}
	if err := r.Get(&got); !reflect.DeepEqual(got, want) || err != nil {
		t.Errorf("ref.WithContext() = (%v, %v); want = (%v, nil)", got, err, want)
	}
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
		if err := testref.OrderByChild(tc).Get(&got); err != nil {
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
	if err := testref.OrderByChild("messages/ratings").Get(&got); err != nil {
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
	if err := q.Get(&got); err != nil {
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
		if err := r.OrderByChild(tc).Get(&got); got != "" || err == nil {
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
	if err := testref.OrderByKey().Get(&got); err != nil {
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
	if err := testref.OrderByValue().Get(&got); err != nil {
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
	if err := testref.OrderByChild("messages").WithLimitToFirst(10).Get(&got); err != nil {
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
	if err := testref.OrderByChild("messages").WithLimitToLast(10).Get(&got); err != nil {
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
		if err := tc.q.Get(&got); got != nil || err == nil {
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
	if err := testref.OrderByChild("messages").WithStartAt(10).Get(&got); err != nil {
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
	if err := testref.OrderByChild("messages").WithEndAt(10).Get(&got); err != nil {
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
	if err := testref.OrderByChild("messages").WithEqualTo(10).Get(&got); err != nil {
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
		if err := tc.q.Get(&got); got != nil || err == nil {
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
	if err := q.Get(&got); err != nil {
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
