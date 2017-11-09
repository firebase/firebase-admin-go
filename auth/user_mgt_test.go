package auth

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"firebase.google.com/go/internal"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

type testReq struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
	Query  map[string]string
}

func newTestReq(r *http.Request) (*testReq, error) {
	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(r.RequestURI)
	if err != nil {
		return nil, err
	}

	query := make(map[string]string)
	for k, v := range u.Query() {
		query[k] = v[0]
	}
	return &testReq{
		Method: r.Method,
		Path:   u.Path,
		Header: r.Header,
		Body:   b,
		Query:  query,
	}, nil
}

type mockAuthServer struct {
	Resp   interface{}
	Header map[string]string
	Status int
	Reqs   []*testReq
	srv    *httptest.Server
	client *Client
}

func EchoServer(resp interface{}) *mockAuthServer {
	s := mockAuthServer{Resp: resp}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr, _ := newTestReq(r)
		s.Reqs = append(s.Reqs, tr)

		for k, v := range s.Header {
			w.Header().Set(k, v)
		}
		if s.Status != 0 {
			w.WriteHeader(s.Status)
		}
		b, _ := json.Marshal(s.Resp)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	s.srv = httptest.NewServer(handler)
	authClient, err := NewClient(context.Background(),
		&internal.AuthConfig{
			Opts: []option.ClientOption{option.WithHTTPClient(s.srv.Client())},
		})
	_ = err
	authClient.url = s.srv.Client().URL
	s.client = authClient
	return &s
}
func (s *mockAuthServer) Client() *Client {
	return s.client
}

func TestExportPayload(t *testing.T) {
	uf := NewUserFields()
	_ = uf
}

func TestGetUser(t *testing.T) {
	s := EchoServer("message5")
	t.Errorf(" a %+v\n b %+v\n c %+v\n==\n1 %#v\n2 %#v\n3 %#v\n==================-------=-=-=--", s, s.srv, s.srv.URL,
		s.client.transportClient, s.client.httpClient().Client, s.srv.Client())

	respo, err := s.client.transportClient.Client.Get("fdfdfdfdfdf")
	if err != nil {
		t.Errorf("= - = - = - %s\n%#v\n\n", err, respo)

	}
	t.Fatalf("= - = - = - %s\n%#v\n\n", err, respo)
	defer s.srv.Close()

	user, err := s.Client().GetUser(context.Background(), "asdf")
	_ = user
	_ = err
	//	t.Errorf("%v\n>> > > > \n>\n>\n> > \n%+v\n >>>>> %+v\n >> %s", user, s, s.Client(), err)
}
