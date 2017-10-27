package db

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/net/context"
)

type Query struct {
	ctx                 context.Context
	client              *Client
	path                string
	ob                  orderBy
	limFirst, limLast   int
	start, end, equalTo interface{}
}

func (q *Query) WithStartAt(v interface{}) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.start = v
	return q2
}

func (q *Query) WithEndAt(v interface{}) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.end = v
	return q2
}

func (q *Query) WithEqualTo(v interface{}) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.equalTo = v
	return q2
}

func (q *Query) WithLimitToFirst(lim int) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.limFirst = lim
	return q2
}

func (q *Query) WithLimitToLast(lim int) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.limLast = lim
	return q2
}

func (q *Query) WithContext(ctx context.Context) *Query {
	q2 := new(Query)
	*q2 = *q
	q2.ctx = ctx
	return q2
}

func (q *Query) Get(v interface{}) error {
	qp := make(map[string]string)
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

	req := &request{
		Method: "GET",
		Path:   q.path,
		Opts:   []httpOption{withQueryParams(qp)},
	}
	resp, err := q.client.send(q.ctx, req)
	if err != nil {
		return err
	}
	return resp.CheckAndParse(http.StatusOK, v)
}

func (r *Ref) OrderByChild(child string) *Query {
	return newQuery(r, orderByChild(child))
}

func (r *Ref) OrderByKey() *Query {
	return newQuery(r, orderByProperty("$key"))
}

func (r *Ref) OrderByValue() *Query {
	return newQuery(r, orderByProperty("$value"))
}

func newQuery(r *Ref, ob orderBy) *Query {
	return &Query{
		ctx:    r.ctx,
		client: r.client,
		path:   r.Path,
		ob:     ob,
	}
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
