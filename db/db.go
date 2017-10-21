package db

import (
	"fmt"
	"net/http"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/internal"

	"net/url"

	"io/ioutil"

	"encoding/json"

	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const invalidChars = "[].#$"

var userAgent = fmt.Sprintf("Firebase/HTTP/%s/AdminGo", firebase.Version)

type Client struct {
	hc      *http.Client
	baseURL string
}

func NewClient(ctx context.Context, c *internal.DatabaseConfig) (*Client, error) {
	o := []option.ClientOption{option.WithUserAgent(userAgent)}
	o = append(o, c.Opts...)

	hc, _, err := transport.NewHTTPClient(ctx, o...)
	if err != nil {
		return nil, err
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("database url not specified")
	}
	url, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	} else if url.Scheme != "https" {
		return nil, fmt.Errorf("invalid database URL (incorrect scheme): %q", c.BaseURL)
	} else if !strings.HasSuffix(url.Host, ".firebaseio.com") {
		return nil, fmt.Errorf("invalid database URL (incorrest host): %q", c.BaseURL)
	}
	return &Client{
		hc:      hc,
		baseURL: fmt.Sprintf("https://%s", url.Host),
	}, nil
}

func (c *Client) NewRef(path string) (*Ref, error) {
	if strings.ContainsAny(path, invalidChars) {
		return nil, fmt.Errorf("path %q contains one or more invalid characters", path)
	}
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}

	key := ""
	if len(segs) > 0 {
		key = segs[len(segs)-1]
	}

	return &Ref{
		client: c,
		segs:   segs,
		Key:    key,
		Path:   "/" + strings.Join(segs, "/"),
	}, nil
}

func (c *Client) sendRequest(method string, path string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s%s", c.baseURL, path, ".json")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.hc.Do(req)
}

type Ref struct {
	client *Client
	segs   []string
	Key    string
	Path   string
}

func (r *Ref) Parent() *Ref {
	l := len(r.segs)
	if l > 0 {
		path := strings.Join(r.segs[:l-1], "/")
		parent, _ := r.client.NewRef(path)
		return parent
	}
	return nil
}

func (r *Ref) Get(v interface{}) error {
	resp, err := r.client.sendRequest("GET", r.Path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	return nil
}
