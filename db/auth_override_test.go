package db

import (
	"testing"
)

func TestAuthOverrideGet(t *testing.T) {
	mock := &mockServer{Resp: "data"}
	srv := mock.Start(aoClient)
	defer srv.Close()

	ref := aoClient.NewRef("peter")
	var got string
	if err := ref.Get(&got); err != nil {
		t.Fatal(err)
	}
	if got != "data" {
		t.Errorf("Get() = %q; want = %q", got, "data")
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
	if err := ref.Set(want); err != nil {
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
	if err := ref.OrderByChild("foo").Get(&got); err != nil {
		t.Fatal(err)
	}
	if got != "data" {
		t.Errorf("OrderByChild() = %q; want = %q", got, "data")
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
	if err := ref.OrderByChild("foo").WithStartAt(1).WithEndAt(10).Get(&got); err != nil {
		t.Fatal(err)
	}
	if got != "data" {
		t.Errorf("OrderByChild() = %q; want = %q", got, "data")
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
