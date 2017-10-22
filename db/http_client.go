package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

func (r *Ref) send(method string, body interface{}, opts ...httpOption) (*response, error) {
	url := fmt.Sprintf("%s%s%s", r.client.baseURL, r.Path, ".json")
	var data io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		data = bytes.NewBuffer(b)
		opts = append(opts, withHeader("Content-Type", "application/json"))
	}

	req, err := http.NewRequest(method, url, data)
	if err != nil {
		return nil, err
	}
	for _, o := range opts {
		o(req)
	}

	resp, err := r.client.hc.Do(req)
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
