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
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"firebase.google.com/go/internal"

	"golang.org/x/net/context"
)

// Query represents a complex query that can be executed on a Ref.
//
// Complex queries can consist of up to 2 components: a required ordering constraint, and an
// optional filtering constraint. At the server, data is first sorted according to the given
// ordering constraint (e.g. order by child). Then the filtering constraint (e.g. limit, range) is
// applied on the sorted data to produce the final result. Despite the ordering constraint, the
// final result is returned by the server as an unordered collection. Therefore the values read
// from a Query instance are not ordered.
type Query struct {
	client              *Client
	path                string
	ob                  orderBy
	limFirst, limLast   int
	start, end, equalTo interface{}
}

// WithStartAt returns a shallow copy of the Query with v set as a lower bound of a range query.
//
// The resulting Query will only return child nodes with a value greater than or equal to v.
func (q *Query) WithStartAt(v interface{}) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.start = v
	return q2
}

// WithEndAt returns a shallow copy of the Query with v set as a upper bound of a range query.
//
// The resulting Query will only return child nodes with a value less than or equal to v.
func (q *Query) WithEndAt(v interface{}) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.end = v
	return q2
}

// WithEqualTo returns a shallow copy of the Query with v set as an equals constraint.
//
// The resulting Query will only return child nodes whose values equal to v.
func (q *Query) WithEqualTo(v interface{}) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.equalTo = v
	return q2
}

// WithLimitToFirst returns a shallow copy of the Query, which is anchored to the first n
// elements of the window.
func (q *Query) WithLimitToFirst(n int) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.limFirst = n
	return q2
}

// WithLimitToLast returns a shallow copy of the Query, which is anchored to the last n
// elements of the window.
func (q *Query) WithLimitToLast(n int) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.limLast = n
	return q2
}

// Get executes the Query and populates v with the results.
//
// Results will not be stored in any particular order in v.
func (q *Query) Get(ctx context.Context, v interface{}) error {
	qp := make(map[string]string)
	if err := initQueryParams(q, qp); err != nil {
		return err
	}
	resp, err := q.client.send(ctx, "GET", q.path, nil, internal.WithQueryParams(qp))
	if err != nil {
		return err
	}
	return resp.Unmarshal(http.StatusOK, v)
}

// GetOrdered executes the Query and provides the results as an ordered list.
//
// v must be a pointer to an array or a slice.
func (q *Query) GetOrdered(ctx context.Context, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("value must be a pointer")
	}
	if rv.Elem().Kind() != reflect.Slice && rv.Elem().Kind() != reflect.Array {
		return fmt.Errorf("value must be a pointer to an array or a slice")
	}

	var temp interface{}
	if err := q.Get(ctx, &temp); err != nil {
		return err
	}

	sr, err := newSortableResult(temp, q.ob)
	if err != nil {
		return err
	}
	sort.Sort(sr)

	var values []interface{}
	for _, val := range sr {
		values = append(values, val.Value)
	}
	b, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// OrderByChild returns a Query that orders data by child values before applying filters.
//
// Returned Query can be used to set additional parameters, and execute complex database queries
// (e.g. limit queries, range queries). If r has a context associated with it, the resulting Query
// will inherit it.
func (r *Ref) OrderByChild(child string) *Query {
	return newQuery(r, orderByChild(child))
}

// OrderByKey returns a Query that orders data by key before applying filters.
//
// Returned Query can be used to set additional parameters, and execute complex database queries
// (e.g. limit queries, range queries). If r has a context associated with it, the resulting Query
// will inherit it.
func (r *Ref) OrderByKey() *Query {
	return newQuery(r, orderByProperty("$key"))
}

// OrderByValue returns a Query that orders data by value before applying filters.
//
// Returned Query can be used to set additional parameters, and execute complex database queries
// (e.g. limit queries, range queries). If r has a context associated with it, the resulting Query
// will inherit it.
func (r *Ref) OrderByValue() *Query {
	return newQuery(r, orderByProperty("$value"))
}

func newQuery(r *Ref, ob orderBy) *Query {
	return &Query{
		client: r.client,
		path:   r.Path,
		ob:     ob,
	}
}

func initQueryParams(q *Query, qp map[string]string) error {
	ob, err := q.ob.encode()
	if err != nil {
		return err
	}
	qp["orderBy"] = ob

	if q.limFirst > 0 && q.limLast > 0 {
		return fmt.Errorf("cannot set both limit parameter: first = %d, last = %d", q.limFirst, q.limLast)
	} else if q.limFirst < 0 {
		return fmt.Errorf("limit first cannot be negative: %d", q.limFirst)
	} else if q.limLast < 0 {
		return fmt.Errorf("limit last cannot be negative: %d", q.limLast)
	}

	if q.limFirst > 0 {
		qp["limitToFirst"] = strconv.Itoa(q.limFirst)
	} else if q.limLast > 0 {
		qp["limitToLast"] = strconv.Itoa(q.limLast)
	}

	if err := encodeFilter("startAt", q.start, qp); err != nil {
		return err
	}
	if err := encodeFilter("endAt", q.end, qp); err != nil {
		return err
	}
	return encodeFilter("equalTo", q.equalTo, qp)
}

func encodeFilter(key string, val interface{}, m map[string]string) error {
	if val == nil {
		return nil
	}
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	m[key] = string(b)
	return nil
}

type orderBy interface {
	encode() (string, error)
}

type orderByChild string

func (p orderByChild) encode() (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty child path")
	} else if strings.ContainsAny(string(p), invalidChars) {
		return "", fmt.Errorf("invalid child path with illegal characters: %q", p)
	}
	segs := parsePath(string(p))
	if len(segs) == 0 {
		return "", fmt.Errorf("invalid child path: %q", p)
	}
	b, err := json.Marshal(strings.Join(segs, "/"))
	if err != nil {
		return "", nil
	}
	return string(b), nil
}

type orderByProperty string

func (p orderByProperty) encode() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

const (
	typeNull      = 0
	typeBoolFalse = 1
	typeBoolTrue  = 2
	typeNumeric   = 3
	typeString    = 4
	typeObject    = 5
)

type comparableKey struct {
	Num *float64
	Str *string
}

func (k *comparableKey) Val() interface{} {
	if k.Str != nil {
		return *k.Str
	}
	return *k.Num
}

func (k *comparableKey) Compare(o *comparableKey) int {
	if k.Str != nil && o.Str != nil {
		return strings.Compare(*k.Str, *o.Str)
	} else if k.Num != nil && o.Num != nil {
		if *k.Num < *o.Num {
			return -1
		} else if *k.Num == *o.Num {
			return 0
		}
		return 1
	} else if k.Num != nil {
		return -1
	}
	return 1
}

func newComparableKey(v interface{}) *comparableKey {
	if s, ok := v.(string); ok {
		return &comparableKey{Str: &s}
	}
	if i, ok := v.(int); ok {
		f := float64(i)
		return &comparableKey{Num: &f}
	}

	f := v.(float64)
	return &comparableKey{Num: &f}
}

type sortableResult []*sortEntry

func (s sortableResult) Len() int {
	return len(s)
}

func (s sortableResult) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortableResult) Less(i, j int) bool {
	a, b := s[i], s[j]
	var aKey, bKey *comparableKey
	if a.IndexType == b.IndexType {
		if (a.IndexType == typeNumeric || a.IndexType == typeString) && a.Index != b.Index {
			aKey, bKey = newComparableKey(a.Index), newComparableKey(b.Index)
		} else {
			aKey, bKey = a.Key, b.Key
		}
	} else {
		aKey, bKey = newComparableKey(a.IndexType), newComparableKey(b.IndexType)
	}

	return aKey.Compare(bKey) < 0
}

func newSortableResult(values interface{}, order orderBy) (sortableResult, error) {
	var entries sortableResult
	if m, ok := values.(map[string]interface{}); ok {
		for key, val := range m {
			entries = append(entries, newSortEntry(key, val, order))
		}
	} else if l, ok := values.([]interface{}); ok {
		for key, val := range l {
			entries = append(entries, newSortEntry(key, val, order))
		}
	} else {
		return nil, fmt.Errorf("sorting not supported for the result")
	}
	return entries, nil
}

type sortEntry struct {
	Key       *comparableKey
	Value     interface{}
	Index     interface{}
	IndexType int
}

func newSortEntry(key, val interface{}, order orderBy) *sortEntry {
	var index interface{}
	if prop, ok := order.(orderByProperty); ok {
		if prop == "$value" {
			index = val
		} else {
			index = key
		}
	} else {
		path := order.(orderByChild)
		index = extractChildValue(val, string(path))
	}
	return &sortEntry{
		Key:       newComparableKey(key),
		Value:     val,
		Index:     index,
		IndexType: getIndexType(index),
	}
}

func extractChildValue(val interface{}, path string) interface{} {
	segments := parsePath(path)
	curr := val
	for _, s := range segments {
		if curr == nil {
			return nil
		}

		currMap, ok := curr.(map[string]interface{})
		if !ok {
			return nil
		}
		if curr, ok = currMap[s]; !ok {
			return nil
		}
	}
	return curr
}

func getIndexType(index interface{}) int {
	if index == nil {
		return typeNull
	} else if b, ok := index.(bool); ok {
		if b {
			return typeBoolTrue
		}
		return typeBoolFalse
	} else if _, ok := index.(float64); ok {
		return typeNumeric
	} else if _, ok := index.(string); ok {
		return typeString
	}
	return typeObject
}
