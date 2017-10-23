package db

import "testing"
import "firebase.google.com/go/db"

var heightSorted = []string{
	"linhenykus", "pterodactyl", "lambeosaurus",
	"triceratops", "stegosaurus", "bruhathkayosaurus",
}

func TestLimitToFirst(t *testing.T) {
	q, err := dinos.OrderByChild("height", db.WithLimitToFirst(2))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]Dinosaur
	if err := q.Get(&m); err != nil {
		t.Fatal(err)
	}
	want := heightSorted[:2]
	if len(m) != 2 {
		t.Errorf("WithLimitToFirst() = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("WithLimitToFirst() = %v; want key %q", m, d)
		}
	}
}

func TestLimitToLast(t *testing.T) {
	q, err := dinos.OrderByChild("height", db.WithLimitToLast(2))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]Dinosaur
	if err := q.Get(&m); err != nil {
		t.Fatal(err)
	}
	want := heightSorted[len(heightSorted)-2:]
	if len(m) != 2 {
		t.Errorf("WithLimitToLast() = %v; want = %v", m, want)
	}
	for _, d := range want {
		if _, ok := m[d]; !ok {
			t.Errorf("WithLimitToLast() = %v; want key %q", m, d)
		}
	}
}
