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
	"reflect"
	"testing"

	"firebase.google.com/go/db"
)

var heightSorted = []string{
	"linhenykus", "pterodactyl", "lambeosaurus",
	"triceratops", "stegosaurus", "bruhathkayosaurus",
}

func TestLimitToFirst(t *testing.T) {
	for _, tc := range []int{2, 10} {
		results, err := dinos.OrderByChild("height").LimitToFirst(tc).GetOrdered(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		wl := min(tc, len(heightSorted))
		want := heightSorted[:wl]
		if len(results) != wl {
			t.Errorf("LimitToFirst() = %d; want = %d", len(results), wl)
		}
		got := getNames(results)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LimitToLast() = %v; want = %v", got, want)
		}
		compareValues(t, results)
	}
}

func TestLimitToLast(t *testing.T) {
	for _, tc := range []int{2, 10} {
		results, err := dinos.OrderByChild("height").LimitToLast(tc).GetOrdered(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		wl := min(tc, len(heightSorted))
		want := heightSorted[len(heightSorted)-wl:]
		if len(results) != wl {
			t.Errorf("LimitToLast() = %d; want = %d", len(results), wl)
		}
		got := getNames(results)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LimitToLast() = %v; want = %v", got, want)
		}
		compareValues(t, results)
	}
}

func TestStartAt(t *testing.T) {
	results, err := dinos.OrderByChild("height").StartAt(3.5).GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := heightSorted[len(heightSorted)-2:]
	if len(results) != len(want) {
		t.Errorf("StartAt() = %d; want = %d", len(results), len(want))
	}
	got := getNames(results)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LimitToLast() = %v; want = %v", got, want)
	}
	compareValues(t, results)
}

func TestEndAt(t *testing.T) {
	results, err := dinos.OrderByChild("height").EndAt(3.5).GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := heightSorted[:4]
	if len(results) != len(want) {
		t.Errorf("StartAt() = %d; want = %d", len(results), len(want))
	}
	got := getNames(results)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LimitToLast() = %v; want = %v", got, want)
	}
	compareValues(t, results)
}

func TestStartAndEndAt(t *testing.T) {
	results, err := dinos.OrderByChild("height").StartAt(2.5).EndAt(5).GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := heightSorted[len(heightSorted)-3 : len(heightSorted)-1]
	if len(results) != len(want) {
		t.Errorf("StartAt(), EndAt() = %d; want = %d", len(results), len(want))
	}
	got := getNames(results)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LimitToLast() = %v; want = %v", got, want)
	}
	compareValues(t, results)
}

func TestEqualTo(t *testing.T) {
	results, err := dinos.OrderByChild("height").EqualTo(0.6).GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := heightSorted[:2]
	if len(results) != len(want) {
		t.Errorf("EqualTo() = %d; want = %d", len(results), len(want))
	}
	got := getNames(results)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LimitToLast() = %v; want = %v", got, want)
	}
	compareValues(t, results)
}

func TestOrderByNestedChild(t *testing.T) {
	results, err := dinos.OrderByChild("ratings/pos").StartAt(4).GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"pterodactyl", "stegosaurus", "triceratops"}
	if len(results) != len(want) {
		t.Errorf("OrderByChild(ratings/pos) = %d; want = %d", len(results), len(want))
	}
	got := getNames(results)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LimitToLast() = %v; want = %v", got, want)
	}
	compareValues(t, results)
}

func TestOrderByKey(t *testing.T) {
	results, err := dinos.OrderByKey().LimitToFirst(2).GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"bruhathkayosaurus", "lambeosaurus"}
	if len(results) != len(want) {
		t.Errorf("OrderByKey() = %d; want = %d", len(results), len(want))
	}
	got := getNames(results)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LimitToLast() = %v; want = %v", got, want)
	}
	compareValues(t, results)
}

func TestOrderByValue(t *testing.T) {
	scores := ref.Child("scores")
	results, err := scores.OrderByValue().LimitToLast(2).GetOrdered(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"linhenykus", "pterodactyl"}
	if len(results) != len(want) {
		t.Errorf("OrderByValue() = %d; want = %d", len(results), len(want))
	}
	got := getNames(results)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LimitToLast() = %v; want = %v", got, want)
	}
	wantScores := []int{80, 93}
	for i, r := range results {
		var val int
		if err := r.Unmarshal(&val); err != nil {
			t.Fatalf("queryNode.Unmarshal() = %v", err)
		}
		if val != wantScores[i] {
			t.Errorf("queryNode.Unmarshal() = %d; want = %d", val, wantScores[i])
		}
	}
}

func TestQueryWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q := dinos.OrderByKey().LimitToFirst(2)
	var m map[string]Dinosaur
	if err := q.Get(ctx, &m); err != nil {
		t.Fatal(err)
	}

	want := []string{"bruhathkayosaurus", "lambeosaurus"}
	if len(m) != len(want) {
		t.Errorf("OrderByKey() = %d; want = %d", len(m), len(want))
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
		StartAt(2.5).
		EndAt(5).
		Get(context.Background(), &m); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[len(heightSorted)-3 : len(heightSorted)-1]
	if len(m) != len(want) {
		t.Errorf("Get() = %d; want = %d", len(m), len(want))
	}
	for i, w := range want {
		if _, ok := m[w]; !ok {
			t.Errorf("[%d] result[%q] not present", i, w)
		}
	}
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func getNames(results []db.QueryNode) []string {
	s := make([]string, len(results))
	for i, v := range results {
		s[i] = v.Key()
	}
	return s
}

func compareValues(t *testing.T, results []db.QueryNode) {
	for _, r := range results {
		var d Dinosaur
		if err := r.Unmarshal(&d); err != nil {
			t.Fatalf("queryNode.Unmarshal(%q) = %v", r.Key(), err)
		}
		if !reflect.DeepEqual(d, parsedTestData[r.Key()]) {
			t.Errorf("queryNode.Unmarshal(%q) = %v; want = %v", r.Key(), d, parsedTestData[r.Key()])
		}
	}
}
