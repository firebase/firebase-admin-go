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
			LimitToFirst(tc).
			GetOrdered(context.Background(), &d); err != nil {
			t.Fatal(err)
		}

		wl := min(tc, len(heightSorted))
		want := heightSorted[:wl]
		if len(d) != wl {
			t.Errorf("LimitToFirst() = %d; want = %d", len(d), wl)
		}
		for i, w := range want {
			if d[i] != parsedTestData[w] {
				t.Errorf("[%d] LimitToFirst() = %v; want = %v", i, d[i], parsedTestData[w])
			}
		}
	}
}

func TestLimitToLast(t *testing.T) {
	for _, tc := range []int{2, 10} {
		var d []Dinosaur
		if err := dinos.OrderByChild("height").
			LimitToLast(tc).
			GetOrdered(context.Background(), &d); err != nil {
			t.Fatal(err)
		}

		wl := min(tc, len(heightSorted))
		want := heightSorted[len(heightSorted)-wl:]
		if len(d) != wl {
			t.Errorf("LimitToLast() = %d; want = %d", len(d), wl)
		}
		for i, w := range want {
			if d[i] != parsedTestData[w] {
				t.Errorf("[%d] LimitToLast() = %v; want = %v", i, d[i], parsedTestData[w])
			}
		}
	}
}

func TestStartAt(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("height").
		StartAt(3.5).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[len(heightSorted)-2:]
	if len(d) != len(want) {
		t.Errorf("StartAt() = %d; want = %d", len(d), len(want))
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] StartAt() = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestEndAt(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("height").
		EndAt(3.5).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[:4]
	if len(d) != len(want) {
		t.Errorf("StartAt() = %d; want = %d", len(d), len(want))
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] EndAt() = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestStartAndEndAt(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("height").
		StartAt(2.5).
		EndAt(5).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[len(heightSorted)-3 : len(heightSorted)-1]
	if len(d) != len(want) {
		t.Errorf("StartAt(), EndAt() = %d; want = %d", len(d), len(want))
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] StartAt(), EndAt() = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestEqualTo(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("height").
		EqualTo(0.6).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := heightSorted[:2]
	if len(d) != len(want) {
		t.Errorf("EqualTo() = %d; want = %d", len(d), len(want))
	}
	for i, w := range want {
		if d[i] != parsedTestData[w] {
			t.Errorf("[%d] EqualTo() = %v; want = %v", i, d[i], parsedTestData[w])
		}
	}
}

func TestOrderByNestedChild(t *testing.T) {
	var d []Dinosaur
	if err := dinos.OrderByChild("ratings/pos").
		StartAt(4).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := []string{"pterodactyl", "stegosaurus", "triceratops"}
	if len(d) != len(want) {
		t.Errorf("OrderByChild(ratings/pos) = %d; want = %d", len(d), len(want))
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
		LimitToFirst(2).
		GetOrdered(context.Background(), &d); err != nil {
		t.Fatal(err)
	}

	want := []string{"bruhathkayosaurus", "lambeosaurus"}
	if len(d) != len(want) {
		t.Errorf("OrderByKey() = %d; want = %d", len(d), len(want))
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
		LimitToLast(2).
		GetOrdered(context.Background(), &s); err != nil {
		t.Fatal(err)
	}

	want := []string{"linhenykus", "pterodactyl"}
	if len(s) != len(want) {
		t.Errorf("OrderByValue() = %d; want = %d", len(s), len(want))
	}
	scoresData := testData["scores"].(map[string]interface{})
	for i, w := range want {
		ws := int(scoresData[w].(float64))
		if s[i] != ws {
			t.Errorf("[%d] OrderByValue() = %d; want = %d", i, s[i], ws)
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
