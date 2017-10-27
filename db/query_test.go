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

	var got map[string]interface{}
	if err := ref.OrderByChild("messages").Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"orderBy": "\"messages\""},
	})
}

func TestNestedChildQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := ref.OrderByChild("messages/ratings").Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
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

	opts := []QueryOption{
		WithStartAt("m4"),
		WithEndAt("m50"),
		WithLimitToFirst(10),
	}
	var got map[string]interface{}
	if err := ref.OrderByChild("messages", opts...).Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
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

func TestInvalidChildPath(t *testing.T) {
	mock := &mockServer{Resp: "test"}
	srv := mock.Start(client)
	defer srv.Close()

	r := client.NewRef("/")
	cases := []string{
		"foo$", "foo.", "foo#", "foo]", "foo[",
	}
	for _, tc := range cases {
		var got string
		if err := r.OrderByChild(tc).Get(&got); got != "" || err == nil {
			t.Errorf("Get() = (%q, %v); want = (%q, error)", got, err, "")
		}
	}
	if len(mock.Reqs) != 0 {
		t.Errorf("Requests: %v; want: empty", mock.Reqs)
	}
}

func TestKeyQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := ref.OrderByKey().Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
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
	if err := ref.OrderByValue().Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
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
	if err := ref.OrderByChild("messages", WithLimitToFirst(10)).Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
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
	if err := ref.OrderByChild("messages", WithLimitToLast(10)).Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
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

	var got map[string]interface{}
	q := ref.OrderByChild("messages", WithLimitToFirst(10), WithLimitToLast(10))
	if err := q.Get(&got); got != nil || err == nil {
		t.Errorf("Get() = (%v, %v); want = (nil, error)", got, err)
	}
	if len(mock.Reqs) != 0 {
		t.Errorf("Requests: %v; want: empty", mock.Reqs)
	}
}

func TestStartAtQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := ref.OrderByChild("messages", WithStartAt(10)).Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
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
	if err := ref.OrderByChild("messages", WithEndAt(10)).Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"endAt": "10", "orderBy": "\"messages\""},
	})
}

func TestAllParamsQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	q := ref.OrderByChild("messages", WithLimitToFirst(100), WithStartAt("bar"), WithEndAt("foo"))
	var got map[string]interface{}
	if err := q.Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
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

func TestEqualToQuery(t *testing.T) {
	want := map[string]interface{}{"m1": "Hello", "m2": "Bye"}
	mock := &mockServer{Resp: want}
	srv := mock.Start(client)
	defer srv.Close()

	var got map[string]interface{}
	if err := ref.OrderByChild("messages", WithEqualTo(10)).Get(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Get() = %v; want = %v", got, want)
	}
	checkOnlyRequest(t, mock.Reqs, &testReq{
		Method: "GET",
		Path:   "/peter.json",
		Query:  map[string]string{"equalTo": "10", "orderBy": "\"messages\""},
	})
}
