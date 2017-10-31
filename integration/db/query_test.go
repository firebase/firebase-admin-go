package db

import (
	"testing"

	"golang.org/x/net/context"
)

var heightSorted = []string{
	"linhenykus", "pterodactyl", "lambeosaurus",
	"triceratops", "stegosaurus", "bruhathkayosaurus",
}

func TestLimitToFirst(t *testing.T) {
	for _, tc := range []int{2, 10} {
		var m map[string]Dinosaur
		if err := dinos.OrderByChild("height").
			WithLimitToFirst(tc).
			Get(context.Background(), &m); err != nil {
			t.Fatal(err)
		}

		wl := tc
		if len(heightSorted) < wl {
			wl = len(heightSorted)
		}
		want := heightSorted[:wl]
		if len(m) != len(want) {
			t.Errorf("WithLimitToFirst() = %v; want = %v", m, want)
		}
		for _, d := range want {
			if _, ok := m[d]; !ok {
				t.Errorf("WithLimitToFirst() = %v; want key %q", m, d)
			}
		}
	}
}

func TestLimitToLast(t *testing.T) {
	for _, tc := range []int{2, 10} {
		var m map[string]Dinosaur
		if err := dinos.OrderByChild("height").
			WithLimitToLast(tc).
			Get(context.Background(), &m); err != nil {
			t.Fatal(err)
		}

		wl := tc
		if len(heightSorted) < wl {
			wl = len(heightSorted)
		}
		want := heightSorted[len(heightSorted)-wl:]
		if len(m) != len(want) {
			t.Errorf("WithLimitToLast() = %v; want = %v", m, want)
		}
		for _, d := range want {
			if _, ok := m[d]; !ok {
				t.Errorf("WithLimitToLast() = %v; want key %q", m, d)
			}
		}
	}
}

func TestStartAt(t *testing.T) {
	var m map[string]Dinosaur
	if err := dinos.OrderByChild("height").
		WithStartAt(3.5).
		Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[len(heightSorted)-2:]
	if len(m) != len(want) {
		t.Errorf("WithStartAt() = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("WithStartAt() = %v; want key %q", m, d)
		}
	}
}

func TestEndAt(t *testing.T) {
	var m map[string]Dinosaur
	if err := dinos.OrderByChild("height").
		WithEndAt(3.5).
		Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[:4]
	if len(m) != len(want) {
		t.Errorf("WithStartAt() = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("WithStartAt() = %v; want key %q", m, d)
		}
	}
}

func TestStartAndEndAt(t *testing.T) {
	var m map[string]Dinosaur
	if err := dinos.OrderByChild("height").
		WithStartAt(2.5).
		WithEndAt(5).
		Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[len(heightSorted)-3 : len(heightSorted)-1]
	if len(m) != len(want) {
		t.Errorf("WithStartAt(), WithEndAt() = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("WithStartAt(), WithEndAt() = %v; want key %q", m, d)
		}
	}
}

func TestEqualTo(t *testing.T) {
	var m map[string]Dinosaur
	if err := dinos.OrderByChild("height").
		WithEqualTo(0.6).
		Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[:2]
	if len(m) != len(want) {
		t.Errorf("WithEqualTo() = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("WithEqualTo() = %v; want key %q", m, d)
		}
	}
}

func TestOrderByNestedChild(t *testing.T) {
	var m map[string]Dinosaur
	if err := dinos.OrderByChild("ratings/pos").
		WithStartAt(4).
		Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}

	want := []string{"pterodactyl", "stegosaurus", "triceratops"}
	if len(m) != len(want) {
		t.Errorf("OrderByChild(ratings/pos) = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("OrderByChild(ratings/pos) = %v; want key %q", m, d)
		}
	}
}

func TestOrderByKey(t *testing.T) {
	var m map[string]Dinosaur
	if err := dinos.OrderByKey().WithLimitToFirst(2).Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}

	want := []string{"bruhathkayosaurus", "lambeosaurus"}
	if len(m) != len(want) {
		t.Errorf("OrderByKey() = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("OrderByKey() = %v; want key %q", m, d)
		}
	}
}

func TestOrderByValue(t *testing.T) {
	scores := ref.Child("scores")
	var m map[string]int
	if err := scores.OrderByValue().WithLimitToLast(2).Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}

	want := []string{"pterodactyl", "linhenykus"}
	if len(m) != len(want) {
		t.Errorf("OrderByValue() = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("OrderByValue() = %v; want key %q", m, d)
		}
	}
}

func TestQueryWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q := dinos.OrderByKey().WithLimitToFirst(2)
	var m map[string]Dinosaur
	if err := q.Get(ctx, &m); err != nil {
		t.Fatal(err)
	}

	want := []string{"bruhathkayosaurus", "lambeosaurus"}
	if len(m) != len(want) {
		t.Errorf("OrderByKey() = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("OrderByKey() = %v; want key %q", m, d)
		}
	}

	cancel()
	m = nil
	if err := q.Get(ctx, &m); len(m) != 0 || err == nil {
		t.Errorf("Get() = (%v, %v); want = (empty, error)", m, err)
	}
}
