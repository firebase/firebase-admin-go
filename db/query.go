package db

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type Query interface {
	Get(v interface{}) error
	WithContext(ctx context.Context) Query
}

type queryImpl struct {
	ctx  context.Context
	ref  *Ref
	opts []QueryOption
}

func (q *queryImpl) Get(v interface{}) error {
	qp := make(queryParams)
	for _, o := range q.opts {
		if err := o.apply(qp); err != nil {
			return err
		}
	}

	opts := []httpOption{withQueryParams(qp)}
	if q.ctx != nil {
		opts = append(opts, withContext(q.ctx))
	}
	resp, err := q.ref.send("GET", nil, opts...)
	if err != nil {
		return err
	}
	return resp.CheckAndParse(http.StatusOK, v)
}

func (q *queryImpl) WithContext(ctx context.Context) Query {
	q2 := new(queryImpl)
	*q2 = *q
	q2.ctx = ctx
	return q2
}

type QueryOption interface {
	apply(qp queryParams) error
}

func WithLimitToFirst(lim int) QueryOption {
	return &limitParam{"limitToFirst", lim}
}

func WithLimitToLast(lim int) QueryOption {
	return &limitParam{"limitToLast", lim}
}

func WithStartAt(v interface{}) QueryOption {
	return &filterParam{"startAt", v}
}

func WithEndAt(v interface{}) QueryOption {
	return &filterParam{"endAt", v}
}

func WithEqualTo(v interface{}) QueryOption {
	return &filterParam{"equalTo", v}
}

func (r *Ref) OrderByChild(child string, opts ...QueryOption) Query {
	opts = append(opts, orderByChild(child))
	return newQuery(r, opts)
}

func (r *Ref) OrderByKey(opts ...QueryOption) Query {
	opts = append(opts, orderByParam("$key"))
	return newQuery(r, opts)
}

func (r *Ref) OrderByValue(opts ...QueryOption) Query {
	opts = append(opts, orderByParam("$value"))
	return newQuery(r, opts)
}

func newQuery(r *Ref, opts []QueryOption) Query {
	return &queryImpl{ref: r, opts: opts}
}

type queryParams map[string]string

type orderByChild string

func (p orderByChild) apply(qp queryParams) error {
	if p == "" {
		return fmt.Errorf("empty child path")
	} else if strings.ContainsAny(string(p), invalidChars) {
		return fmt.Errorf("invalid child path with illegal characters: %q", p)
	}
	segs := parsePath(string(p))
	b, err := json.Marshal(strings.Join(segs, "/"))
	if err != nil {
		return nil
	}
	qp["orderBy"] = string(b)
	return nil
}

type orderByParam string

func (p orderByParam) apply(qp queryParams) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	qp["orderBy"] = string(b)
	return nil
}

type limitParam struct {
	key string
	val int
}

func (p *limitParam) apply(qp queryParams) error {
	if p.val < 0 {
		return fmt.Errorf("limit parameters must not be negative: %d", p.val)
	} else if p.val == 0 {
		return nil
	}

	qp[p.key] = strconv.Itoa(p.val)
	cnt := 0
	for _, k := range []string{"limitToFirst", "limitToLast"} {
		if _, ok := qp[k]; ok {
			cnt++
		}
	}
	if cnt == 2 {
		return fmt.Errorf("cannot set both limit parameters")
	}
	return nil
}

type filterParam struct {
	key string
	val interface{}
}

func (p *filterParam) apply(qp queryParams) error {
	if p.val == nil {
		return nil
	}
	b, err := json.Marshal(p.val)
	if err != nil {
		return err
	}
	qp[p.key] = string(b)
	return nil
}
