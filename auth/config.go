package auth

import (
	"fmt"
	"strings"
)

const (
	constMfa                      = "mfa"
	constMultiFactorConfig        = "multiFactorConfig"
	constProviderConfigs          = "providerConfigs"
	constTotpProviderConfig       = "totpProviderConfig"
	constAdjacentIntervals        = "adjacentIntervals"
	constMfaObject                = "MultiFactorConfig"
	constProviderConfigObject     = "ProviderConfig"
	constTotpProviderConfigObject = "TotpProviderConfig"
	constState                    = "state"
	constEnabledProviders         = "enabledProviders"
	constFactorIds                = "factorIds"
)

var validAuthFactors = map[string]bool{
	"PHONE_SMS": true,
}

var validStates = map[string]bool{
	"ENABLED":  true,
	"DISABLED": true,
}

// ProviderConfig represents Multi Factor Provider configuration.
// This config is used to set second factor auth except for SMS.
// Currently, only TOTP is supported.
type ProviderConfig struct {
	State              string                 `json:"state"`
	TotpProviderConfig *TotpMfaProviderConfig `json:"totpProviderConfig,omitEmpty"`
}

// TotpMfaProviderConfig represents configuration settings for TOTP second factor auth.
type TotpMfaProviderConfig struct {
	AdjacentIntervals int `json:"adjacentIntervals,omitEmpty"`
}

// MultiFactorConfig represents a multi-factor configuration for Tenant/Project .
// This can be used to define whether multi-factor authentication is enabled
// or disabled and the list of second factor challenges that are supported.
type MultiFactorConfig struct {
	State            string            `json:"state"`
	EnabledProviders []string          `json:"enabledProviders,omitEmpty"`
	ProviderConfigs  []*ProviderConfig `json:"providerConfigs,omitEmpty"`
}

func validateConfigKeys(inputReq *map[string]interface{}, validKeys map[string]bool, configName string) error {
	for key := range *inputReq {
		if !validKeys[key] {
			return fmt.Errorf(`"%s" is not a valid %s parameter`, key, configName)
		}
	}
	return nil
}

func stringKeys(m *map[string]bool) string {
	var keys []string
	for key := range *m {
		keys = append(keys, key)
	}
	return fmt.Sprintf(`{%s}`, strings.Join(keys, ","))
}

func validateAndConvertMultiFactorConfig(multiFactorConfig interface{}) (nestedMap, error) {
	if multiFactorConfig == nil {
		return nil, fmt.Errorf(`%s must be defined`, constMultiFactorConfig)
	}
	req := make(map[string]interface{})
	//validate mfa config keys
	mfaMap, ok := multiFactorConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a valid %s type`, constMultiFactorConfig, constMfaObject)
	}
	if err := validateConfigKeys(&mfaMap, map[string]bool{constState: true, constFactorIds: true, constProviderConfigs: true}, constMfaObject); err != nil {
		return nil, err
	}

	//validate mfa.state
	state, ok := mfaMap[constState]
	if !ok {
		return nil, fmt.Errorf(`%s.%s should be defined`, constMultiFactorConfig, constState)
	}
	s, ok := state.(string)
	if !ok || !validStates[s] {
		return nil, fmt.Errorf(`%s.%s must be in %s`, constMultiFactorConfig, constState, stringKeys(&validStates))
	}
	req[constState] = s

	//validate mfa.factorIds
	factorIds, ok := mfaMap[constFactorIds]
	if ok {
		var authFactorIds []string
		fi, ok := factorIds.([]string)
		if !ok {
			return nil, fmt.Errorf(`%s.%s must be a list of strings in %s`, constMultiFactorConfig, constFactorIds, stringKeys(&validAuthFactors))
		}
		for _, f := range fi {
			if !validAuthFactors[f] {
				return nil, fmt.Errorf(`%s.%s must be a list of strings in %s`, constMultiFactorConfig, constFactorIds, stringKeys(&validAuthFactors))
			}
			authFactorIds = append(authFactorIds, f)
		}
		req[constEnabledProviders] = make([]string, len(authFactorIds))
		copy(req[constEnabledProviders].([]string), authFactorIds)
	}

	//validate provider configs
	providerConfigs, ok := mfaMap[constProviderConfigs]
	if ok {
		pc, ok := providerConfigs.([]map[string]interface{})
		if !ok {
			return nil, fmt.Errorf(`%s.%s must be a list of %ss`, constMultiFactorConfig, constProviderConfigs, constProviderConfigObject)
		}
		var reqProviderConfigsList []map[string]interface{}
		for _, providerConfig := range pc {
			reqProviderConfig := make(map[string]interface{})

			//validate providerConfig struct keys
			if err := validateConfigKeys(&providerConfig, map[string]bool{constState: true, constTotpProviderConfig: true}, constProviderConfigObject); err != nil {
				return nil, err
			}

			//validate providerConfig.state
			state, ok := providerConfig[constState]
			if !ok {
				return nil, fmt.Errorf(`%s.%s should be defined`, constProviderConfigObject, constState)
			}
			s, ok := state.(string)
			if !ok || !validStates[s] {
				return nil, fmt.Errorf(`%s.%s must be in %s`, constProviderConfigObject, constState, stringKeys(&validStates))
			}
			reqProviderConfig[constState] = s

			//validate providerConfig.totpProviderConfig
			totpProviderConfig, ok := providerConfig[constTotpProviderConfig]
			if !ok {
				return nil, fmt.Errorf(`%s.%s should be present`, constProviderConfigObject, constTotpProviderConfig)
			}
			tpc, ok := totpProviderConfig.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf(`%s must be of type %s`, constTotpProviderConfig, constTotpProviderConfigObject)
			}

			//validate totpProviderConfig keys
			if err := validateConfigKeys(&tpc, map[string]bool{constAdjacentIntervals: true}, constTotpProviderConfigObject); err != nil {
				return nil, err
			}
			reqTotpProviderConfig := make(map[string]interface{})

			//validate adjacentIntervals if present
			ai, ok := tpc[constAdjacentIntervals].(int)
			if !ok || !(0 <= ai && ai <= 10) {
				return nil, fmt.Errorf(`%s must be a valid number between 0 and 10 (both inclusive)`, constAdjacentIntervals)
			}
			reqTotpProviderConfig[constAdjacentIntervals] = ai
			reqProviderConfig[constTotpProviderConfig] = reqTotpProviderConfig
			reqProviderConfigsList = append(reqProviderConfigsList, reqProviderConfig)
		}
		req[constProviderConfigs] = reqProviderConfigsList
	}

	//return validated multi factor config auth request
	return req, nil
}
