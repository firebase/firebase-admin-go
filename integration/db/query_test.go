package db

import "testing"
import "firebase.google.com/go/db"

var heightSorted = []string{
	"linhenykus", "pterodactyl", "lambeosaurus",
	"triceratops", "stegosaurus", "bruhathkayosaurus",
}

func TestLimitToFirst(t *testing.T) {
	for _, tc := range []int{2, 10} {
		q := dinos.OrderByChild("height", db.WithLimitToFirst(tc))
		var m map[string]Dinosaur
		if err := q.Get(&m); err != nil {
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
		q := dinos.OrderByChild("height", db.WithLimitToLast(tc))
		var m map[string]Dinosaur
		if err := q.Get(&m); err != nil {
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
	q := dinos.OrderByChild("height", db.WithStartAt(3.5))
	var m map[string]Dinosaur
	if err := q.Get(&m); err != nil {
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
	q := dinos.OrderByChild("height", db.WithEndAt(3.5))
	var m map[string]Dinosaur
	if err := q.Get(&m); err != nil {
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
	q := dinos.OrderByChild("height", db.WithStartAt(2.5), db.WithEndAt(5))
	var m map[string]Dinosaur
	if err := q.Get(&m); err != nil {
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
	q := dinos.OrderByChild("height", db.WithEqualTo(0.6))
	var m map[string]Dinosaur
	if err := q.Get(&m); err != nil {
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
	q := dinos.OrderByChild("ratings/pos", db.WithStartAt(4))
	var m map[string]Dinosaur
	if err := q.Get(&m); err != nil {
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
	q := dinos.OrderByKey(db.WithLimitToFirst(2))
	var m map[string]Dinosaur
	if err := q.Get(&m); err != nil {
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
	q := scores.OrderByValue(db.WithLimitToLast(2))
	var m map[string]int
	if err := q.Get(&m); err != nil {
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
