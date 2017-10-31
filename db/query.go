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

	req := &dbReq{
		Method: "GET",
		Path:   q.path,
		Opts:   []internal.HTTPOption{internal.WithQueryParams(qp)},
	}
	resp, err := q.client.send(ctx, req)
	if err != nil {
		return err
	}
	return resp.Unmarshal(http.StatusOK, v)
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
	if err := encodeFilter("equalTo", q.equalTo, qp); err != nil {
		return err
	}
	return nil
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
