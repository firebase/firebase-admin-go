package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockServer struct {
	Resp   interface{}
	Header map[string]string
	Status int
	srv    *httptest.Server
}

func (s *mockServer) Start(c *Client) *httptest.Server {
	if s.srv != nil {
		return s.srv
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range s.Header {
			w.Header().Set(k, v)
		}

		print := r.URL.Query().Get("print")
		if s.Status != 0 {
			w.WriteHeader(s.Status)
		} else if print == "silent" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		b, _ := json.Marshal(s.Resp)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})
	s.srv = httptest.NewServer(handler)
	//c.url = s.srv.URL
	return s.srv
}

func TestExportPayload(t *testing.T) {
	uf := NewUserFields()
	_ = uf
}

func TestGetUser(t *testing.T) {

}
