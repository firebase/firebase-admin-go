package db

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var reservedFilters = map[string]bool{
	"$key":      true,
	"$value":    true,
	"$priority": true,
}

type Query struct {
	ref *Ref
	qp  queryParams
}

func (q *Query) Get(v interface{}) error {
	resp, err := q.ref.send("GET", nil, withQueryParams(q.qp))
	if err != nil {
		return err
	}
	return resp.CheckAndParse(http.StatusOK, v)
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

func (r *Ref) OrderByChild(child string, opts ...QueryOption) (*Query, error) {
	if child == "" {
		return nil, fmt.Errorf("child path must be a non-empty string")
	}
	if _, ok := reservedFilters[child]; ok {
		return nil, fmt.Errorf("invalid child path: %s", child)
	}
	segs, err := parsePath(child)
	if err != nil {
		return nil, err
	}
	opts = append(opts, orderByParam(strings.Join(segs, "/")))
	return newQuery(r, opts)
}

func (r *Ref) OrderByKey(opts ...QueryOption) (*Query, error) {
	opts = append(opts, orderByParam("$key"))
	return newQuery(r, opts)
}

func (r *Ref) OrderByValue(opts ...QueryOption) (*Query, error) {
	opts = append(opts, orderByParam("$value"))
	return newQuery(r, opts)
}

func newQuery(r *Ref, opts []QueryOption) (*Query, error) {
	qp := make(queryParams)
	for _, o := range opts {
		if err := o.apply(qp); err != nil {
			return nil, err
		}
	}
	return &Query{ref: r, qp: qp}, nil
}

type queryParams map[string]string

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
