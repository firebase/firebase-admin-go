package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

const invalidChars = "[].#$"
const authVarOverride = "auth_variable_override"

type request struct {
	Method string
	Path   string
	Body   interface{}
	Opts   []httpOption
}

func (c *Client) send(ctx context.Context, r *request) (*response, error) {
	if strings.ContainsAny(r.Path, invalidChars) {
		return nil, fmt.Errorf("invalid path with illegal characters: %q", r.Path)
	}

	var opts []httpOption
	var data io.Reader
	if r.Body != nil {
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, err
		}
		data = bytes.NewBuffer(b)
		opts = append(opts, withHeader("Content-Type", "application/json"))
	}

	url := fmt.Sprintf("%s%s.json", c.url, r.Path)
	req, err := http.NewRequest(r.Method, url, data)
	if err != nil {
		return nil, err
	}

	if ctx != nil {
		req = req.WithContext(ctx)
	}

	if c.ao != "" {
		opts = append(opts, withQueryParam(authVarOverride, c.ao))
	}
	opts = append(opts, r.Opts...)

	return doSend(c.hc, req, opts...)
}

func doSend(hc *http.Client, req *http.Request, opts ...httpOption) (*response, error) {
	for _, o := range opts {
		o(req)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &response{
		Status: resp.StatusCode,
		Body:   b,
		Header: resp.Header,
	}, nil
}

type response struct {
	Status int
	Header http.Header
	Body   []byte
}

func (r *response) CheckStatus(want int) error {
	if r.Status == want {
		return nil
	}
	var b struct {
		Error string `json:"error"`
	}
	json.Unmarshal(r.Body, &b)
	var msg string
	if b.Error != "" {
		msg = fmt.Sprintf("http error status: %d; reason: %s", r.Status, b.Error)
	} else {
		msg = fmt.Sprintf("http error status: %d; message: %s", r.Status, string(r.Body))
	}
	return fmt.Errorf(msg)
}

func (r *response) CheckAndParse(want int, v interface{}) error {
	if err := r.CheckStatus(want); err != nil {
		return err
	} else if err := json.Unmarshal(r.Body, v); err != nil {
		return err
	}
	return nil
}

type httpOption func(*http.Request)

func withHeader(key, value string) httpOption {
	return func(r *http.Request) {
		r.Header.Set(key, value)
	}
}

func withQueryParam(key, value string) httpOption {
	return func(r *http.Request) {
		q := r.URL.Query()
		q.Add(key, value)
		r.URL.RawQuery = q.Encode()
	}
}

func withQueryParams(qp queryParams) httpOption {
	return func(r *http.Request) {
		q := r.URL.Query()
		for k, v := range qp {
			q.Add(k, v)
		}
		r.URL.RawQuery = q.Encode()
	}
}
