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
	"reflect"
	"testing"
)

var sortableKeysResp = map[string]interface{}{
	"bob":     person{Name: "bob", Age: 20},
	"alice":   person{Name: "alice", Age: 30},
	"charlie": person{Name: "charlie", Age: 15},
	"dave":    person{Name: "dave", Age: 25},
	"ernie":   person{Name: "ernie"},
}

var sortableValuesResp = []struct {
	resp     map[string]interface{}
	want     []interface{}
	wantKeys []string
}{
	{
		resp:     map[string]interface{}{"k1": 1, "k2": 2, "k3": 3},
		want:     []interface{}{1.0, 2.0, 3.0},
		wantKeys: []string{"k1", "k2", "k3"},
	},
	{
		resp:     map[string]interface{}{"k1": 3, "k2": 2, "k3": 1},
		want:     []interface{}{1.0, 2.0, 3.0},
		wantKeys: []string{"k3", "k2", "k1"},
	},
	{
		resp:     map[string]interface{}{"k1": 3, "k2": 1, "k3": 2},
		want:     []interface{}{1.0, 2.0, 3.0},
		wantKeys: []string{"k2", "k3", "k1"},
	},
	{
		resp:     map[string]interface{}{"k1": 1, "k2": 2, "k3": 1},
		want:     []interface{}{1.0, 1.0, 2.0},
		wantKeys: []string{"k1", "k3", "k2"},
	},
	{
		resp:     map[string]interface{}{"k1": 1, "k2": 1, "k3": 2},
		want:     []interface{}{1.0, 1.0, 2.0},
		wantKeys: []string{"k1", "k2", "k3"},
	},
	{
		resp:     map[string]interface{}{"k1": 2, "k2": 1, "k3": 1},
		want:     []interface{}{1.0, 1.0, 2.0},
		wantKeys: []string{"k2", "k3", "k1"},
	},
	{
		resp:     map[string]interface{}{"k1": "foo", "k2": "bar", "k3": "baz"},
		want:     []interface{}{"bar", "baz", "foo"},
		wantKeys: []string{"k2", "k3", "k1"},
	},
	{
		resp:     map[string]interface{}{"k1": "foo", "k2": "bar", "k3": 10},
		want:     []interface{}{10.0, "bar", "foo"},
		wantKeys: []string{"k3", "k2", "k1"},
	},
	{
		resp:     map[string]interface{}{"k1": "foo", "k2": "bar", "k3": nil},
		want:     []interface{}{nil, "bar", "foo"},
		wantKeys: []string{"k3", "k2", "k1"},
	},
	{
		resp:     map[string]interface{}{"k1": 5, "k2": "bar", "k3": nil},
		want:     []interface{}{nil, 5.0, "bar"},
		wantKeys: []string{"k3", "k1", "k2"},
	},
	{
		resp: map[string]interface{}{
			"k1": true, "k2": 0, "k3": "foo", "k4": "foo", "k5": false,
			"k6": map[string]interface{}{"k1": true},
		},
		want:     []interface{}{false, true, 0.0, "foo", "foo", map[string]interface{}{"k1": true}},
		wantKeys: []string{"k5", "k1", "k2", "k3", "k4", "k6"},
	},
	{
		resp: map[string]interface{}{
			"k1": true, "k2": 0, "k3": "foo", "k4": "foo", "k5": false,
			"k6": map[string]interface{}{"k1": true}, "k7": nil,
			"k8": map[string]interface{}{"k0": true},
		},
		want: []interface{}{
			nil, false, true, 0.0, "foo", "foo",
			map[string]interface{}{"k1": true}, map[string]interface{}{"k0": true},
		},
		wantKeys: []string{"k7", "k5", "k1", "k2", "k3", "k4", "k6", "k8"},
	},
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

	q := testref.OrderByChild("messages").StartAt("m4").EndAt("m50").LimitToFirst(10)
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
	if err := testref.OrderByChild("messages").LimitToFirst(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("LimitToFirst() = %v; want = %v", got, want)
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
	if err := testref.OrderByChild("messages").LimitToLast(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("LimitToLast() = %v; want = %v", got, want)
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
		{"BothLimits", q.LimitToFirst(10).LimitToLast(10)},
		{"NegativeFirst", q.LimitToFirst(-10)},
		{"NegativeLast", q.LimitToLast(-10)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]interface{}
			if err := tc.q.Get(context.Background(), &got); got != nil || err == nil {
				t.Errorf("OrderByChild(%q) = (%v, %v); want = (nil, error)", tc.name, got, err)
			}
			if len(mock.Reqs) != 0 {
				t.Errorf("OrderByChild(%q): %v; want: empty", tc.name, mock.Reqs)
			}
		})
	}
}

func TestStartAtQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := testref.OrderByChild("messages").StartAt(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("StartAt() = %v; want = %v", got, want)
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
	if err := testref.OrderByChild("messages").EndAt(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("EndAt() = %v; want = %v", got, want)
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
	if err := testref.OrderByChild("messages").EqualTo(10).Get(context.Background(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("EqualTo() = %v; want = %v", got, want)
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
		{"InvalidStartAt", q.StartAt(func() {})},
		{"InvalidEndAt", q.EndAt(func() {})},
		{"InvalidEqualTo", q.EqualTo(func() {})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]interface{}
			if err := tc.q.Get(context.Background(), &got); got != nil || err == nil {
				t.Errorf("OrderByChild(%q) = (%v, %v); want = (nil, error)", tc.name, got, err)
			}
			if len(mock.Reqs) != 0 {
				t.Errorf("OrdderByChild(%q) = %v; want = empty", tc.name, mock.Reqs)
			}
		})
	}
}

func TestAllParamsQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q := testref.OrderByChild("messages").LimitToFirst(100).StartAt("bar").EndAt("foo")
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

func TestChildQueryGetOrdered(t *testing.T) {
	mock := &mockServer{Resp: sortableKeysResp}
	srv := mock.Start(client)
	defer srv.Close()

	cases := []struct {
		child string
		want  []string
	}{
		{"name", []string{"alice", "bob", "charlie", "dave", "ernie"}},
		{"age", []string{"ernie", "charlie", "bob", "dave", "alice"}},
		{"nonexisting", []string{"alice", "bob", "charlie", "dave", "ernie"}},
	}

	var reqs []*testReq
	for idx, tc := range cases {
		result, err := testref.OrderByChild(tc.child).GetOrdered(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		reqs = append(reqs, &testReq{
			Method: "GET",
			Path:   "/peter.json",
			Query:  map[string]string{"orderBy": fmt.Sprintf("%q", tc.child)},
		})

		var gotKeys, gotVals []string
		for _, r := range result {
			var p person
			if err := r.Unmarshal(&p); err != nil {
				t.Fatal(err)
			}
			gotKeys = append(gotKeys, r.Key())
			gotVals = append(gotVals, p.Name)
		}
		if !reflect.DeepEqual(tc.want, gotKeys) {
			t.Errorf("[%d] GetOrdered(child: %q) = %v; want = %v", idx, tc.child, gotKeys, tc.want)
		}
		if !reflect.DeepEqual(tc.want, gotVals) {
			t.Errorf("[%d] GetOrdered(child: %q) = %v; want = %v", idx, tc.child, gotVals, tc.want)
		}
	}
	checkAllRequests(t, mock.Reqs, reqs)
}

func TestImmediateChildQueryGetOrdered(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	type parsedMap struct {
		Child interface{} `json:"child"`
	}

	var reqs []*testReq
	for idx, tc := range sortableValuesResp {
		resp := map[string]interface{}{}
		for k, v := range tc.resp {
			resp[k] = map[string]interface{}{"child": v}
		}
		mock.Resp = resp

		result, err := testref.OrderByChild("child").GetOrdered(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		reqs = append(reqs, &testReq{
			Method: "GET",
			Path:   "/peter.json",
			Query:  map[string]string{"orderBy": "\"child\""},
		})

		var gotKeys []string
		var gotVals []interface{}
		for _, r := range result {
			var p parsedMap
			if err := r.Unmarshal(&p); err != nil {
				t.Fatal(err)
			}
			gotKeys = append(gotKeys, r.Key())
			gotVals = append(gotVals, p.Child)
		}
		if !reflect.DeepEqual(tc.wantKeys, gotKeys) {
			t.Errorf("[%d] GetOrdered(child: %q) = %v; want = %v", idx, "child", gotKeys, tc.wantKeys)
		}
		if !reflect.DeepEqual(tc.want, gotVals) {
			t.Errorf("[%d] GetOrdered(child: %q) = %v; want = %v", idx, "child", gotVals, tc.want)
		}
	}
	checkAllRequests(t, mock.Reqs, reqs)
}

func TestNestedChildQueryGetOrdered(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	type grandChild struct {
		GrandChild interface{} `json:"grandchild"`
	}
	type parsedMap struct {
		Child grandChild `json:"child"`
	}

	var reqs []*testReq
	for idx, tc := range sortableValuesResp {
		resp := map[string]interface{}{}
		for k, v := range tc.resp {
			resp[k] = map[string]interface{}{"child": map[string]interface{}{"grandchild": v}}
		}
		mock.Resp = resp

		q := testref.OrderByChild("child/grandchild")
		result, err := q.GetOrdered(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		reqs = append(reqs, &testReq{
			Method: "GET",
			Path:   "/peter.json",
			Query:  map[string]string{"orderBy": "\"child/grandchild\""},
		})

		var gotKeys []string
		var gotVals []interface{}
		for _, r := range result {
			var p parsedMap
			if err := r.Unmarshal(&p); err != nil {
				t.Fatal(err)
			}
			gotKeys = append(gotKeys, r.Key())
			gotVals = append(gotVals, p.Child.GrandChild)
		}
		if !reflect.DeepEqual(tc.wantKeys, gotKeys) {
			t.Errorf("[%d] GetOrdered(child: %q) = %v; want = %v", idx, "child/grandchild", gotKeys, tc.wantKeys)
		}
		if !reflect.DeepEqual(tc.want, gotVals) {
			t.Errorf("[%d] GetOrdered(child: %q) = %v; want = %v", idx, "child/grandchild", gotVals, tc.want)
		}
	}
	checkAllRequests(t, mock.Reqs, reqs)
}

func TestKeyQueryGetOrdered(t *testing.T) {
	mock := &mockServer{Resp: sortableKeysResp}
	srv := mock.Start(client)
	defer srv.Close()

	result, err := testref.OrderByKey().GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	req := &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"orderBy": "\"$key\""},
	}

	var gotKeys, gotVals []string
	for _, r := range result {
		var p person
		if err := r.Unmarshal(&p); err != nil {
			t.Fatal(err)
		}
		gotKeys = append(gotKeys, r.Key())
		gotVals = append(gotVals, p.Name)
	}

	want := []string{"alice", "bob", "charlie", "dave", "ernie"}
	if !reflect.DeepEqual(want, gotKeys) {
		t.Errorf("GetOrdered(key) = %v; want = %v", gotKeys, want)
	}
	if !reflect.DeepEqual(want, gotVals) {
		t.Errorf("GetOrdered(key) = %v; want = %v", gotVals, want)
	}
	checkOnlyRequest(t, mock.Reqs, req)
}

func TestValueQueryGetOrdered(t *testing.T) {
	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	var reqs []*testReq
	for idx, tc := range sortableValuesResp {
		mock.Resp = tc.resp

		result, err := testref.OrderByValue().GetOrdered(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		reqs = append(reqs, &testReq{
			Method: "GET",
			Path:   "/peter.json",
			Query:  map[string]string{"orderBy": "\"$value\""},
		})

		var gotKeys []string
		var gotVals []interface{}
		for _, r := range result {
			var v interface{}
			if err := r.Unmarshal(&v); err != nil {
				t.Fatal(err)
			}
			gotKeys = append(gotKeys, r.Key())
			gotVals = append(gotVals, v)
		}

		if !reflect.DeepEqual(tc.wantKeys, gotKeys) {
			t.Errorf("[%d] GetOrdered(value) = %v; want = %v", idx, gotKeys, tc.wantKeys)
		}
		if !reflect.DeepEqual(tc.want, gotVals) {
			t.Errorf("[%d] GetOrdered(value) = %v; want = %v", idx, gotVals, tc.want)
		}
	}
	checkAllRequests(t, mock.Reqs, reqs)
}

func TestValueQueryGetOrderedWithList(t *testing.T) {
	cases := []struct {
		resp     []interface{}
		want     []interface{}
		wantKeys []string
	}{
		{
			resp:     []interface{}{1, 2, 3},
			want:     []interface{}{1.0, 2.0, 3.0},
			wantKeys: []string{"0", "1", "2"},
		},
		{
			resp:     []interface{}{3, 2, 1},
			want:     []interface{}{1.0, 2.0, 3.0},
			wantKeys: []string{"2", "1", "0"},
		},
		{
			resp:     []interface{}{1, 3, 2},
			want:     []interface{}{1.0, 2.0, 3.0},
			wantKeys: []string{"0", "2", "1"},
		},
		{
			resp:     []interface{}{1, 3, 3},
			want:     []interface{}{1.0, 3.0, 3.0},
			wantKeys: []string{"0", "1", "2"},
		},
		{
			resp:     []interface{}{1, 2, 1},
			want:     []interface{}{1.0, 1.0, 2.0},
			wantKeys: []string{"0", "2", "1"},
		},
		{
			resp:     []interface{}{"foo", "bar", "baz"},
			want:     []interface{}{"bar", "baz", "foo"},
			wantKeys: []string{"1", "2", "0"},
		},
		{
			resp:     []interface{}{"foo", 1, false, nil, 0, true},
			want:     []interface{}{nil, false, true, 0.0, 1.0, "foo"},
			wantKeys: []string{"3", "2", "5", "4", "1", "0"},
		},
	}

	mock := &mockServer{}
	srv := mock.Start(client)
	defer srv.Close()

	var reqs []*testReq
	for _, tc := range cases {
		mock.Resp = tc.resp

		result, err := testref.OrderByValue().GetOrdered(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		reqs = append(reqs, &testReq{
			Method: "GET",
			Path:   "/peter.json",
			Query:  map[string]string{"orderBy": "\"$value\""},
		})

		var gotKeys []string
		var gotVals []interface{}
		for _, r := range result {
			var v interface{}
			if err := r.Unmarshal(&v); err != nil {
				t.Fatal(err)
			}
			gotKeys = append(gotKeys, r.Key())
			gotVals = append(gotVals, v)
		}

		if !reflect.DeepEqual(tc.wantKeys, gotKeys) {
			t.Errorf("GetOrdered(value) = %v; want = %v", gotKeys, tc.wantKeys)
		}
		if !reflect.DeepEqual(tc.want, gotVals) {
			t.Errorf("GetOrdered(value) = %v; want = %v", gotVals, tc.want)
		}
	}
	checkAllRequests(t, mock.Reqs, reqs)
}

func TestGetOrderedWithNilResult(t *testing.T) {
	mock := &mockServer{Resp: nil}
	srv := mock.Start(client)
	defer srv.Close()

	result, err := testref.OrderByChild("child").GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("GetOrdered(value) = %v; want = nil", result)
	}
}

func TestGetOrderedWithLeafNode(t *testing.T) {
	mock := &mockServer{Resp: "foo"}
	srv := mock.Start(client)
	defer srv.Close()

	result, err := testref.OrderByChild("child").GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("GetOrdered(chid) = %d; want = 1", len(result))
	}
	if result[0].Key() != "0" {
		t.Errorf("GetOrdered(value).Key() = %v; want = %q", result[0].Key(), 0)
	}

	var v interface{}
	if err := result[0].Unmarshal(&v); err != nil {
		t.Fatal(err)
	}
	if v != "foo" {
		t.Errorf("GetOrdered(value) = %v; want = %v", v, "foo")
	}
}

func TestQueryHttpError(t *testing.T) {
	mock := &mockServer{Resp: map[string]string{"error": "test error"}, Status: 500}
	srv := mock.Start(client)
	defer srv.Close()

	want := "http error status: 500; reason: test error"
	result, err := testref.OrderByChild("child").GetOrdered(context.Background())
	if err == nil || err.Error() != want {
		t.Errorf("GetOrdered() = %v; want = %v", err, want)
	}
	if result != nil {
		t.Errorf("GetOrdered() = %v; want = nil", result)
	}
}
