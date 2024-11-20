package remoteconfig

import (
	"context"
	"fmt"
	"net/http"
	
	"firebase.google.com/go/v4/internal"
)

type ServerTemplateData struct {
	Conditions []struct {
		Name      string `json:"name"`
		Condition interface{} `json:"condition"`
	} `json:"conditions"`
	Parameters	map[string]RemoteConfigParameter `json:"parameters"`

	Version struct {
		VersionNumber	string	`json:"versionNumber"`
		IsLegacy	bool	`json:"isLegacy"`
	}	`json:"version"`

	ETag	string
}

type RemoteConfigParameter struct {
	DefaultValue struct {
		Value string `json:"value"`
	} `json:"defaultValue"`
	ConditionalValues   map[string]RemoteConfigParameterValue	`json:"conditionalValues"`
}

type RemoteConfigParameterValue interface{}

// ServerTemplate represents a template with configuration data, cache, and service information.
type ServerTemplate struct {
	RcClient	*rcClient
	Cache       *ServerTemplateData
}

// NewServerTemplate initializes a new ServerTemplate with optional default configuration.
func NewServerTemplate(rcClient *rcClient) *ServerTemplate {
	return &ServerTemplate{
		RcClient: rcClient,
		Cache:	  nil,
	}
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Load(ctx context.Context) error {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s/v1/projects/%s/namespaces/firebase-server/serverRemoteConfig", s.RcClient.RcBaseUrl , s.RcClient.Project),
	}

	var templateData ServerTemplateData
	response, err := s.RcClient.HttpClient.DoAndUnmarshal(ctx, request, &templateData)

	if err != nil {
		return err
	}

	templateData.ETag = response.Header.Get("etag")
	s.Cache = &templateData
	fmt.Println("Etag", s.Cache.ETag) // TODO: Remove ETag 
	return nil
}

// Load fetches the server template data from the remote config service and caches it.
func (s *ServerTemplate) Set(templateData *ServerTemplateData) {
	s.Cache = templateData 
}

// Evaluate processes the cached template data with a condition evaluator 
// based on the provided context.
func (s *ServerTemplate) Evaluate(context map[string]interface{}) *ServerConfig {
	// TODO: Write ConditionalEvaluator for evaluating

    configMap := make(map[string]Value)
    for key, value := range s.Cache.Parameters{
        configMap[key] = *NewValue(Remote, value.DefaultValue.Value)
    }

	return &ServerConfig{ConfigValues: configMap}
}