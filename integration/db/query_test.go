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
	"testing"

	"golang.org/x/net/context"
)

var heightSorted = []string{
	"linhenykus", "pterodactyl", "lambeosaurus",
	"triceratops", "stegosaurus", "bruhathkayosaurus",
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func TestLimitToFirst(t *testing.T) {
	for _, tc := range []int{2, 10} {
		var d []Dinosaur
		if err := dinos.OrderByChild("height").
			WithLimitToFirst(tc).
			GetOrdered(context.Background(), &d); err != nil {
			t.Fatal(err)
		}

		wl := min(tc, len(heightSorted))
		want := heightSorted[:wl]
		if len(d) != wl {
			t.Errorf("WithLimitToFirst() = %v; want = %v", d, want)
		}
		for i, w := range want {
			if d[i] != parsedTestData[w] {
				t.Errorf("[%d] WithLimitToFirst() = %v; want = %v", i, d[i], parsedTestData[w])
			}
		}
	}
}

func TestLimitToLast(t *testing.T) {
	for _, tc := range []int{2, 10} {
		var d []Dinosaur
		if err := dinos.OrderByChild("height").
			WithLimitToLast(tc).
			GetOrdered(context.Background(), &d); err != nil {
			t.Fatal(err)
		}

		wl := min(tc, len(heightSorted))
		want := heightSorted[len(heightSorted)-wl:]
		if len(d) != wl {
			t.Errorf("WithLimitToLast() = %v; want = %v", d, want)
		}
		for i, w := range want {
			if d[i] != parsedTestData[w] {
				t.Errorf("[%d] WithLimitToLast() = %v; want = %v", i, d[i], parsedTestData[w])
			}
		}
	}
}

func TestStartAt(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("height").
		WithStartAt(3.5).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[len(heightSorted)-2:]
	if len(d) != len(want) {
		t.Errorf("WithStartAt() = %v; want = %v", d, want)
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] WithStartAt() = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestEndAt(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("height").
		WithEndAt(3.5).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[:4]
	if len(d) != len(want) {
		t.Errorf("WithStartAt() = %v; want = %v", d, want)
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] WithEndAt() = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestStartAndEndAt(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("height").
		WithStartAt(2.5).
		WithEndAt(5).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[len(heightSorted)-3 : len(heightSorted)-1]
	if len(d) != len(want) {
		t.Errorf("WithStartAt(), WithEndAt() = %v; want = %v", d, want)
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] WithStartAt(), WithEndAt() = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestEqualTo(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("height").
		WithEqualTo(0.6).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[:2]
	if len(d) != len(want) {
		t.Errorf("WithEqualTo() = %v; want = %v", d, want)
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] WithEqualTo() = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestOrderByNestedChild(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("ratings/pos").
		WithStartAt(4).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := []string{"pterodactyl", "stegosaurus", "triceratops"}
	if len(d) != len(want) {
		t.Errorf("OrderByChild(ratings/pos) = %v; want = %v", d, want)
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] OrderByChild(ratings/pos) = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestOrderByKey(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByKey().
		WithLimitToFirst(2).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := []string{"bruhathkayosaurus", "lambeosaurus"}
	if len(d) != len(want) {
		t.Errorf("OrderByKey() = %v; want = %v", d, want)
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] OrderByKey() = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestOrderByValue(t *testing.T) {
	scores := ref.Child("scores")
	var s []int
	if err := scores.OrderByValue().
		WithLimitToLast(2).
		GetOrdered(context.Background(), &s); err != nil {
		t.Fatal(err)
	}

	want := []string{"linhenykus", "pterodactyl"}
	if len(s) != len(want) {
		t.Errorf("OrderByValue() = %v; want = %v", s, want)
	}
	scoresData := testData["scores"].(map[string]interface{})
	for i, w := range want {
		ws := int(scoresData[w].(float64))
		if s[i] != ws {
			t.Errorf("[%d] OrderByValue() = %v; want = %v", i, s[i], ws)
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

func TestUnorderedQuery(t *testing.T) {
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
	for _, w := range want {
		if _, ok := m[w]; !ok {
			t.Errorf("WithStartAt(), WithEndAt() = %v; want key = %v", m, w)
		}
	}
}
