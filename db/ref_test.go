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
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

type refOp func(r *Ref) error

var testOps = []struct {
	name string
	resp interface{}
	op   refOp
}{
	{
		"Get()",
		"test",
		func(r *Ref) error {
			var got string
			return r.Get(context.Background(), &got)
		},
	},
	{
		"GetWithETag()",
		"test",
		func(r *Ref) error {
			var got string
			_, err := r.GetWithETag(context.Background(), &got)
			return err
		},
	},
	{
		"GetShallow()",
		"test",
		func(r *Ref) error {
			var got string
			return r.GetShallow(context.Background(), &got)
		},
	},
	{
		"GetIfChanged()",
		"test",
		func(r *Ref) error {
			var got string
			_, _, err := r.GetIfChanged(context.Background(), "etag", &got)
			return err
		},
	},
	{
		"Set()",
		nil,
		func(r *Ref) error {
			return r.Set(context.Background(), "foo")
		},
	},
	{
		"SetIfUnchanged()",
		nil,
		func(r *Ref) error {
			_, err := r.SetIfUnchanged(context.Background(), "etag", "foo")
			return err
		},
	},
	{
		"Push()",
		map[string]interface{}{"name": "test"},
		func(r *Ref) error {
			_, err := r.Push(context.Background(), "foo")
			return err
		},
	},
	{
		"Update()",
		nil,
		func(r *Ref) error {
			return r.Update(context.Background(), map[string]interface{}{"foo": "bar"})
		},
	},
	{
		"Delete()",
		nil,
		func(r *Ref) error {
			return r.Delete(context.Background())
		},
	},
	{
		"Transaction()",
		nil,
		func(r *Ref) error {
			fn := func(t TransactionNode) (interface{}, error) {
				var v interface{}
				if err := t.Unmarshal(&v); err != nil {
					return nil, err
				}
				return v, nil
			}
			return r.Transaction(context.Background(), fn)
		},
	},
}

func TestGet(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	cases := []interface{}{
		nil, float64(1), true, "foo",
		map[string]interface{}{"name": "Peter Parker", "age": float64(17)},
	}
	var want []*testReq
	for _, tc := range cases {
		mock.Resp = tc
		var got interface{}
		if err := testref.Get(context.Background(), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tc, got) {
			t.Errorf("Get() = %v; want = %v", got, tc)
		}
		want = append(want, &testReq{Method: "GET", Path: "/peter.json"})
	}
	checkAllRequests(t, mock.Reqs, want)
}

func TestInvalidGet(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	got := func() {}
	if err := testref.Get(context.Background(), &got); err == nil {
		t.Errorf("Get(func) = nil; want error")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestGetWithStruct(t *testing.T) {
	want := person{Name: "Peter Parker", Age: 17}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got person
	if err := testref.Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Errorf("Get(struct) = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json"})
}

func TestGetShallow(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	cases := []interface{}{
		nil, float64(1), true, "foo",
		map[string]interface{}{"name": "Peter Parker", "age": float64(17)},
		map[string]interface{}{"name": "Peter Parker", "nestedChild": true},
	}
	wantQuery := map[string]string{"shallow": "true"}
	var want []*testReq
	for _, tc := range cases {
		mock.Resp = tc
		var got interface{}
		if err := testref.GetShallow(context.Background(), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tc, got) {
			t.Errorf("GetShallow() = %v; want = %v", got, tc)
		}
		want = append(want, &testReq{Method: "GET", Path: "/peter.json", Query: wantQuery})
	}
	checkAllRequests(t, mock.Reqs, want)
}

func TestGetWithETag(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{
		Resp:   want,
		Header: map[string]string{"ETag": "mock-etag"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	etag, err := testref.GetWithETag(context.Background(), &got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("GetWithETag() = %v; want = %v", got, want)
	}
	if etag != "mock-etag" {
		t.Errorf("GetWithETag() = %q; want = %q", etag, "mock-etag")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Header: http.Header{"X-Firebase-ETag": []string{"true"}},
	})
}

func TestGetIfChanged(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{
		Resp:   want,
		Header: map[string]string{"ETag": "new-etag"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	ok, etag, err := testref.GetIfChanged(context.Background(), "old-etag", &got)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("GetIfChanged() = %v; want = %v", ok, true)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("GetIfChanged() = %v; want = %v", got, want)
	}
	if etag != "new-etag" {
		t.Errorf("GetIfChanged() = %q; want = %q", etag, "new-etag")
	}

	mock.Status = http.StatusNotModified
	mock.Resp = nil
	var got2 map[string]interface{}
	ok, etag, err = testref.GetIfChanged(context.Background(), "new-etag", &got2)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("GetIfChanged() = %v; want = %v", ok, false)
	}
	if got2 != nil {
		t.Errorf("GetIfChanged() = %v; want nil", got2)
	}
	if etag != "new-etag" {
		t.Errorf("GetIfChanged() = %q; want = %q", etag, "new-etag")
	}

	checkAllRequests(t, mock.Reqs, []*testReq{
		{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"If-None-Match": []string{"old-etag"}},
		},
		{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"If-None-Match": []string{"new-etag"}},
		},
	})
}

func TestWelformedHttpError(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"error": "test error"}, Status: 500}
	srv := mock.Start(client)
	defer srv.Close()

	want := "http error status: 500; reason: test error"
	for _, tc := range testOps {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.op(testref)
			if err == nil || err.Error() != want {
				t.Errorf("%s = %v; want = %v", tc.name, err, want)
			}
		})
	}

	wantReqs := len(testOps) * (1 + defaultMaxRetries)
	if len(mock.Reqs) != wantReqs {
		t.Errorf("Requests = %d; want = %d", len(mock.Reqs), wantReqs)
	}
}

func TestUnexpectedHttpError(t *testing.T) {
	mock := &mockServer{Resp: "unexpected error", Status: 500}
	srv := mock.Start(client)
	defer srv.Close()

	want := "http error status: 500; reason: \"unexpected error\""
	for _, tc := range testOps {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.op(testref)
			if err == nil || err.Error() != want {
				t.Errorf("%s = %v; want = %v", tc.name, err, want)
			}
		})
	}

	wantReqs := len(testOps) * (1 + defaultMaxRetries)
	if len(mock.Reqs) != wantReqs {
		t.Errorf("Requests = %d; want = %d", len(mock.Reqs), wantReqs)
	}
}

func TestInvalidPath(t *testing.T) {
	mock := &mockServer{Resp: "test"}
	srv := mock.Start(client)
	defer srv.Close()

	cases := []string{
		"foo$", "foo.", "foo#", "foo]", "foo[",
	}
	for _, tc := range cases {
		r := client.NewRef(tc)
		for _, o := range testOps {
			err := o.op(r)
			if err == nil {
				t.Errorf("%s = nil; want = error", o.name)
			}
		}
	}

	if len(mock.Reqs) != 0 {
		t.Errorf("Requests = %v; want = empty", mock.Reqs)
	}
}

func TestInvalidChildPath(t *testing.T) {
	mock := &mockServer{Resp: "test"}
	srv := mock.Start(client)
	defer srv.Close()

	cases := []string{
		"foo$", "foo.", "foo#", "foo]", "foo[",
	}
	for _, tc := range cases {
		r := testref.Child(tc)
		for _, o := range testOps {
			err := o.op(r)
			if err == nil {
				t.Errorf("%s = nil; want = error", o.name)
			}
		}
	}

	if len(mock.Reqs) != 0 {
		t.Errorf("Requests = %v; want = empty", mock.Reqs)
	}
}

func TestSet(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	cases := []interface{}{
		1,
		true,
		"foo",
		map[string]interface{}{"name": "Peter Parker", "age": float64(17)},
		&person{"Peter Parker", 17},
	}
	var want []*testReq
	for _, tc := range cases {
		if err := testref.Set(context.Background(), tc); err != nil {
			t.Fatal(err)
		}
		want = append(want, &testReq{
			Method: "PUT",
			Path:   "/peter.json",
			Body:   serialize(tc),
			Query:  map[string]string{"print": "silent"},
		})
	}
	checkAllRequests(t, mock.Reqs, want)
}

func TestInvalidSet(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	cases := []interface{}{
		func() {},
		make(chan int),
	}
	for _, tc := range cases {
		if err := testref.Set(context.Background(), tc); err == nil {
			t.Errorf("Set(%v) = nil; want = error", tc)
		}
	}
	if len(mock.Reqs) != 0 {
		t.Errorf("Set() = %v; want = empty", mock.Reqs)
	}
}

func TestSetIfUnchanged(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	want := &person{"Peter Parker", 17}
	ok, err := testref.SetIfUnchanged(context.Background(), "mock-etag", &want)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("SetIfUnchanged() = %v; want = %v", ok, true)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PUT",
		Path:   "/peter.json",
		Body:   serialize(want),
		Header: http.Header{"If-Match": []string{"mock-etag"}},
	})
}

func TestSetIfUnchangedError(t *testing.T) {
	mock := &mockServer{
		Status: http.StatusPreconditionFailed,
		Resp:   &person{"Tony Stark", 39},
	}
	srv := mock.Start(client)
	defer srv.Close()

	want := &person{"Peter Parker", 17}
	ok, err := testref.SetIfUnchanged(context.Background(), "mock-etag", &want)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("SetIfUnchanged() = %v; want = %v", ok, false)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PUT",
		Path:   "/peter.json",
		Body:   serialize(want),
		Header: http.Header{"If-Match": []string{"mock-etag"}},
	})
}

func TestPush(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"name": "new_key"}}
	srv := mock.Start(client)
	defer srv.Close()

	child, err := testref.Push(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if child.Key != "new_key" {
		t.Errorf("Push() = %q; want = %q", child.Key, "new_key")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "POST",
		Path:   "/peter.json",
		Body:   serialize(""),
	})
}

func TestPushWithValue(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"name": "new_key"}}
	srv := mock.Start(client)
	defer srv.Close()

	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	child, err := testref.Push(context.Background(), want)
	if err != nil {
		t.Fatal(err)
	}

	if child.Key != "new_key" {
		t.Errorf("Push() = %q; want = %q", child.Key, "new_key")
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "POST",
		Path:   "/peter.json",
		Body:   serialize(want),
	})
}

func TestUpdate(t *testing.T) {
	want := map[string]interface{}{"name": "Peter Parker", "age": float64(17)}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	if err := testref.Update(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "PATCH",
		Path:   "/peter.json",
		Body:   serialize(want),
		Query:  map[string]string{"print": "silent"},
	})
}

func TestInvalidUpdate(t *testing.T) {
	cases := []map[string]interface{}{
		nil,
		make(map[string]interface{}),
		{"foo": func() {}},
	}
	for _, tc := range cases {
		if err := testref.Update(context.Background(), tc); err == nil {
			t.Errorf("Update(%v) = nil; want error", tc)
		}
	}
}

func TestTransaction(t *testing.T) {
	mock := &mockServer{
		Resp:   &person{"Peter Parker", 17},
		Header: map[string]string{"ETag": "mock-etag"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	var fn UpdateFn = func(t TransactionNode) (interface{}, error) {
		var p person
		if err := t.Unmarshal(&p); err != nil {
			return nil, err
		}
		p.Age++
		return &p, nil
	}
	if err := testref.Transaction(context.Background(), fn); err != nil {
		t.Fatal(err)
	}
	checkAllRequests(t, mock.Reqs, []*testReq{
		{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"X-Firebase-ETag": []string{"true"}},
		},
		{
			Method: "PUT",
			Path:   "/peter.json",
			Body: serialize(map[string]interface{}{
				"name": "Peter Parker",
				"age":  18,
			}),
			Header: http.Header{"If-Match": []string{"mock-etag"}},
		},
	})
}

func TestTransactionRetry(t *testing.T) {
	mock := &mockServer{
		Resp:   &person{"Peter Parker", 17},
		Header: map[string]string{"ETag": "mock-etag1"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	cnt := 0
	var fn UpdateFn = func(t TransactionNode) (interface{}, error) {
		if cnt == 0 {
			mock.Status = http.StatusPreconditionFailed
			mock.Header = map[string]string{"ETag": "mock-etag2"}
			mock.Resp = &person{"Peter Parker", 19}
		} else if cnt == 1 {
			mock.Status = http.StatusOK
		}
		cnt++
		var p person
		if err := t.Unmarshal(&p); err != nil {
			return nil, err
		}
		p.Age++
		return &p, nil
	}
	if err := testref.Transaction(context.Background(), fn); err != nil {
		t.Fatal(err)
	}
	if cnt != 2 {
		t.Errorf("Transaction() retries = %d; want = %d", cnt, 2)
	}
	checkAllRequests(t, mock.Reqs, []*testReq{
		{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"X-Firebase-ETag": []string{"true"}},
		},
		{
			Method: "PUT",
			Path:   "/peter.json",
			Body: serialize(map[string]interface{}{
				"name": "Peter Parker",
				"age":  18,
			}),
			Header: http.Header{"If-Match": []string{"mock-etag1"}},
		},
		{
			Method: "PUT",
			Path:   "/peter.json",
			Body: serialize(map[string]interface{}{
				"name": "Peter Parker",
				"age":  20,
			}),
			Header: http.Header{"If-Match": []string{"mock-etag2"}},
		},
	})
}

func TestTransactionError(t *testing.T) {
	mock := &mockServer{
		Resp:   &person{"Peter Parker", 17},
		Header: map[string]string{"ETag": "mock-etag1"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	cnt := 0
	want := "user error"
	var fn UpdateFn = func(t TransactionNode) (interface{}, error) {
		if cnt == 0 {
			mock.Status = http.StatusPreconditionFailed
			mock.Header = map[string]string{"ETag": "mock-etag2"}
			mock.Resp = &person{"Peter Parker", 19}
		} else if cnt == 1 {
			return nil, fmt.Errorf(want)
		}
		cnt++
		var p person
		if err := t.Unmarshal(&p); err != nil {
			return nil, err
		}
		p.Age++
		return &p, nil
	}
	if err := testref.Transaction(context.Background(), fn); err == nil || err.Error() != want {
		t.Errorf("Transaction() = %v; want = %q", err, want)
	}
	if cnt != 1 {
		t.Errorf("Transaction() retries = %d; want = %d", cnt, 1)
	}
	checkAllRequests(t, mock.Reqs, []*testReq{
		{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"X-Firebase-ETag": []string{"true"}},
		},
		{
			Method: "PUT",
			Path:   "/peter.json",
			Body: serialize(map[string]interface{}{
				"name": "Peter Parker",
				"age":  18,
			}),
			Header: http.Header{"If-Match": []string{"mock-etag1"}},
		},
	})
}

func TestTransactionAbort(t *testing.T) {
	mock := &mockServer{
		Resp:   &person{"Peter Parker", 17},
		Header: map[string]string{"ETag": "mock-etag1"},
	}
	srv := mock.Start(client)
	defer srv.Close()

	cnt := 0
	var fn UpdateFn = func(t TransactionNode) (interface{}, error) {
		if cnt == 0 {
			mock.Status = http.StatusPreconditionFailed
			mock.Header = map[string]string{"ETag": "mock-etag1"}
		}
		cnt++
		var p person
		if err := t.Unmarshal(&p); err != nil {
			return nil, err
		}
		p.Age++
		return &p, nil
	}
	err := testref.Transaction(context.Background(), fn)
	if err == nil {
		t.Errorf("Transaction() = nil; want error")
	}
	wanted := []*testReq{
		{
			Method: "GET",
			Path:   "/peter.json",
			Header: http.Header{"X-Firebase-ETag": []string{"true"}},
		},
	}
	for i := 0; i < txnRetries; i++ {
		wanted = append(wanted, &testReq{
			Method: "PUT",
			Path:   "/peter.json",
			Body: serialize(map[string]interface{}{
				"name": "Peter Parker",
				"age":  18,
			}),
			Header: http.Header{"If-Match": []string{"mock-etag1"}},
		})
	}
	checkAllRequests(t, mock.Reqs, wanted)
}

func TestDelete(t *testing.T) {
	mock := &mockServer{Resp: "null"}
	srv := mock.Start(client)
	defer srv.Close()

	if err := testref.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "DELETE",
		Path:   "/peter.json",
	})
}
