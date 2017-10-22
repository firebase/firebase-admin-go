package db

import (
	"reflect"
	"testing"
)

func TestChildQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q, err := ref.OrderByChild("messages")
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	p := "/peter.json?orderBy=\"messages\""
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: p})
}

func TestNestedChildQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q, err := ref.OrderByChild("messages/ratings")
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	p := "/peter.json?orderBy=\"messages/ratings\""
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: p})
}

func TestChildQueryWithParams(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	opts := []QueryOption{
		WithStartAt("m4"),
		WithEndAt("m50"),
		WithLimitToFirst(10),
	}
	q, err := ref.OrderByChild("messages", opts...)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	p := "/peter.json?startAt=\"m4\"&endAt=\"m50\"&limitToFirst=10&orderBy=\"messages\""
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: p})
}

func TestKeyQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q, err := ref.OrderByKey()
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json?orderBy=\"$key\""})
}

func TestValueQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q, err := ref.OrderByValue()
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: "/peter.json?orderBy=\"$value\""})
}

func TestLimitFirstQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q, err := ref.OrderByChild("messages", WithLimitToFirst(10))
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	p := "/peter.json?limitToFirst=10&orderBy=\"messages\""
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: p})
}

func TestLimitLastQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q, err := ref.OrderByChild("messages", WithLimitToLast(10))
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	p := "/peter.json?limitToLast=10&orderBy=\"messages\""
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: p})
}

func TestInvalidLimitQuery(t *testing.T) {
	q, err := ref.OrderByChild("messages", WithLimitToFirst(10), WithLimitToLast(10))
	if q != nil || err == nil {
		t.Errorf("Query(first=10, last=10) = (%v, %v); want (nil, error)", q, err)
	}

	q, err = ref.OrderByChild("messages", WithLimitToFirst(-10))
	if q != nil || err == nil {
		t.Errorf("Query(first=-10) = (%v, %v); want (nil, error)", q, err)
	}

	q, err = ref.OrderByChild("messages", WithLimitToLast(-10))
	if q != nil || err == nil {
		t.Errorf("Query(last=-10) = (%v, %v); want (nil, error)", q, err)
	}
}

func TestStartAtQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q, err := ref.OrderByChild("messages", WithStartAt(10))
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	p := "/peter.json?startAt=10&orderBy=\"messages\""
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: p})
}

func TestEndAtQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q, err := ref.OrderByChild("messages", WithEndAt(10))
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	p := "/peter.json?endAt=10&orderBy=\"messages\""
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: p})
}

func TestEqualToQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q, err := ref.OrderByChild("messages", WithEqualTo(10))
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	p := "/peter.json?equalTo=10&orderBy=\"messages\""
	checkOnlyRequest(t, mock.Reqs, &testReq{Method: "GET", Path: p})
}
