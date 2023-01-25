package auth

import (
	"context"
	"errors"
	"net/http"

	"firebase.google.com/go/v4/internal"
)

type Project struct {
	Name              string            `json:"name"`
	MultiFactorConfig MultiFactorConfig `json:"mfa,omitEmpty"`
}

func (base *baseClient) Project(ctx context.Context) (*Project, error) {
	req := &internal.Request{
		Method: http.MethodGet,
		URL:    base.projectMgtEndpoint,
	}
	var project Project
	if _, err := base.httpClient.DoAndUnmarshal(ctx, req, &project); err != nil {
		return nil, err
	}

	return &project, nil
}

func (base *baseClient) UpdateProjectConfig(ctx context.Context, project *ProjectToUpdate) (*Project, error) {
	if project == nil {
		return nil, errors.New("project must not be nil")
	}
	request, err := project.validatedRequest()
	if err != nil {
		return nil, err
	}
	req := &internal.Request{
		Method: http.MethodPost,
		URL:    base.projectMgtEndpoint,
		Body:   internal.NewJSONEntity(request),
	}
	var result Project
	if _, err := base.httpClient.DoAndUnmarshal(ctx, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

const (
	multiFactorConfigKey = "multiFactorConfig"
)

type ProjectToUpdate struct {
	params nestedMap
}

func (p *ProjectToUpdate) set(key string, value interface{}) *ProjectToUpdate {
	if p.params == nil {
		p.params = make(map[string]interface{})
	}
	p.params[key] = value
	return p
}

func (p *ProjectToUpdate) MultiFactorConfig(mfaSettings MultiFactorConfig) *ProjectToUpdate {
	mfaConfig := make(map[string]interface{})
	mfaConfig["state"] = mfaSettings.State
	mfaConfig["factorIds"] = mfaSettings.EnabledProviders
	if mfaSettings.ProviderConfigs != nil {
		var providerConfigs [](map[string]interface{})
		for _, providerConfig := range mfaSettings.ProviderConfigs {
			providerConfigTemp := make(map[string]interface{})
			providerConfigTemp["state"] = providerConfig.State
			totpProviderConfig := make(map[string]interface{})
			totpProviderConfig["adjacentIntervals"] = providerConfig.TotpProviderConfig.AdjacentIntervals
			providerConfigTemp["totpProviderConfig"] = totpProviderConfig
			providerConfigs = append(providerConfigs, providerConfigTemp)
		}
		mfaConfig["providerConfigs"] = providerConfigs
	}
	return p.set(multiFactorConfig, mfaConfig)
}

func (p *ProjectToUpdate) validatedRequest() (nestedMap, error) {
	req := make(map[string]interface{})
	for k, v := range p.params {
		req[k] = v
	}
	if mfaConfig, ok := req[multiFactorConfigKey]; ok {
		mfaConfigAuthReq, err := validateAndConvertMultiFactorConfig(mfaConfig)
		if err != nil {
			return nil, err
		}
		//converting to auth type
		req["mfa"] = mfaConfigAuthReq
		delete(req, multiFactorConfigKey)
	}
	return req, nil
}
