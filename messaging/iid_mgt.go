package messaging

import (
	"context"
	"fmt"
	"net/http"

	"firebase.google.com/go/v4/internal"
)

type iidBatchImportRequest struct {
	Application string   `json:"application"`
	Sandbox     bool     `json:"sandbox"`
	ApnsTokens  []string `json:"apns_tokens"`
}

type RegistrationToken struct {
	ApnsToken         string `json:"apns_token"`
	Status            string `json:"status"`
	RegistrationToken string `json:"registration_token,omitempty"`
}

type iidRegistrationTokens struct {
	Results []RegistrationToken `json:"results"`
}

func (c *iidClient) GetRegistrationFromAPNs(
	ctx context.Context,
	application string,
	tokens []string,
) ([]RegistrationToken, error) {
	return c.getRegistrationFromAPNs(ctx, application, tokens, false)
}

func (c *iidClient) GetRegistrationFromAPNsDryRun(
	ctx context.Context,
	application string,
	tokens []string,
) ([]RegistrationToken, error) {
	return c.getRegistrationFromAPNs(ctx, application, tokens, true)
}

func (c *iidClient) getRegistrationFromAPNs(
	ctx context.Context,
	application string,
	tokens []string,
	sandbox bool,
) ([]RegistrationToken, error) {
	if application == "" {
		return nil, fmt.Errorf("empty application id")
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty APNs tokens")
	}

	request := &internal.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s:batchImport", c.iidEndpoint),
		Body: internal.NewJSONEntity(&iidBatchImportRequest{
			Application: application,
			Sandbox:     sandbox,
			ApnsTokens:  tokens,
		}),
	}

	var result iidRegistrationTokens
	if _, err := c.httpClient.DoAndUnmarshal(ctx, request, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

type Topics map[string]struct {
	AddDate string `json:"addDate"`
}

type TokenDetails struct {
	ApplicationVersion string `json:"applicationVersion"`
	Application        string `json:"application"`
	AuthorizedEntity   string `json:"authorizedEntity"`
	Rel                struct {
		Topics Topics `json:"topics"`
	} `json:"rel"`
	Platform string `json:"platform"`
}

func (c *iidClient) GetSubscriptions(ctx context.Context, token string) (*Topics, error) {
	res, err := c.GetTokenDetails(ctx, token)
	if err != nil {
		return nil, err
	}
	return &res.Rel.Topics, nil
}

func (c *iidClient) GetTokenDetails(ctx context.Context, token string) (*TokenDetails, error) {
	request := &internal.Request{
		Method: http.MethodGet,
		URL:    fmt.Sprintf("%s:/info/%s?details=true", c.iidEndpoint, token),
	}
	var result TokenDetails
	if _, err := c.httpClient.DoAndUnmarshal(ctx, request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
