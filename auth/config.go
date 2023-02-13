package auth

import (
	"fmt"
	"strings"
)

const (
	ConstMfa                      = "mfa"
	ConstMultiFactorConfig        = "multiFactorConfig"
	ConstProviderConfigs          = "providerConfigs"
	ConstTotpProviderConfig       = "totpProviderConfig"
	ConstAdjacentIntervals        = "adjacentIntervals"
	ConstMfaObject                = "MultiFactorConfig"
	ConstProviderConfigObject     = "ProviderConfig"
	ConstTotpProviderConfigObject = "TotpProviderConfig"
	ConstState                    = "state"
	ConstEnabledProviders         = "enabledProviders"
	ConstFactorIds                = "factorIds"
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
		return nil, fmt.Errorf(`%s must be defined`, ConstMultiFactorConfig)
	}
	req := make(map[string]interface{})
	//validate mfa config keys
	mfaMap, ok := multiFactorConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a valid %s type`, ConstMultiFactorConfig, ConstMfaObject)
	}
	if err := validateConfigKeys(&mfaMap, map[string]bool{ConstState: true, ConstFactorIds: true, ConstProviderConfigs: true}, ConstMfaObject); err != nil {
		return nil, err
	}

	//validate mfa.state
	state, ok := mfaMap[ConstState]
	if !ok {
		return nil, fmt.Errorf(`%s.%s should be defined`, ConstMultiFactorConfig, ConstState)
	}
	s, ok := state.(string)
	if !ok || !validStates[s] {
		return nil, fmt.Errorf(`%s.%s must be in %s`, ConstMultiFactorConfig, ConstState, stringKeys(&validStates))
	}
	req[ConstState] = s

	//validate mfa.factorIds
	factorIds, ok := mfaMap[ConstFactorIds]
	if ok {
		var authFactorIds []string
		fi, ok := factorIds.([]string)
		if !ok {
			return nil, fmt.Errorf(`%s.%s must be a list of strings in %s`, ConstMultiFactorConfig, ConstFactorIds, stringKeys(&validAuthFactors))
		}
		for _, f := range fi {
			if !validAuthFactors[f] {
				return nil, fmt.Errorf(`%s.%s must be a list of strings in %s`, ConstMultiFactorConfig, ConstFactorIds, stringKeys(&validAuthFactors))
			}
			authFactorIds = append(authFactorIds, f)
		}
		req[ConstEnabledProviders] = make([]string, len(authFactorIds))
		copy(req[ConstEnabledProviders].([]string), authFactorIds)
	}

	//validate provider configs
	providerConfigs, ok := mfaMap[ConstProviderConfigs]
	if ok {
		pc, ok := providerConfigs.([]map[string]interface{})
		if !ok {
			return nil, fmt.Errorf(`%s.%s must be a list of %ss`, ConstMultiFactorConfig, ConstProviderConfigs, ConstProviderConfigObject)
		}
		var reqProviderConfigsList []map[string]interface{}
		for _, providerConfig := range pc {
			reqProviderConfig := make(map[string]interface{})

			//validate providerConfig struct keys
			if err := validateConfigKeys(&providerConfig, map[string]bool{ConstState: true, ConstTotpProviderConfig: true}, ConstProviderConfigObject); err != nil {
				return nil, err
			}

			//validate providerConfig.state
			state, ok := providerConfig[ConstState]
			if !ok {
				return nil, fmt.Errorf(`%s.%s should be defined`, ConstProviderConfigObject, ConstState)
			}
			s, ok := state.(string)
			if !ok || !validStates[s] {
				return nil, fmt.Errorf(`%s.%s must be in %s`, ConstProviderConfigObject, ConstState, stringKeys(&validStates))
			}
			reqProviderConfig[ConstState] = s

			//validate providerConfig.totpProviderConfig
			totpProviderConfig, ok := providerConfig[ConstTotpProviderConfig]
			if !ok {
				return nil, fmt.Errorf(`%s.%s should be present`, ConstProviderConfigObject, ConstTotpProviderConfig)
			}
			tpc, ok := totpProviderConfig.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf(`%s must be of type %s`, ConstTotpProviderConfig, ConstTotpProviderConfigObject)
			}

			//validate totpProviderConfig keys
			if err := validateConfigKeys(&tpc, map[string]bool{ConstAdjacentIntervals: true}, ConstTotpProviderConfigObject); err != nil {
				return nil, err
			}
			reqTotpProviderConfig := make(map[string]interface{})

			//validate adjacentIntervals if present
			ai, ok := tpc[ConstAdjacentIntervals].(int)
			if !ok || !(0 <= ai && ai <= 10) {
				return nil, fmt.Errorf(`%s must be a valid number between 0 and 10 (both inclusive)`, ConstAdjacentIntervals)
			}
			reqTotpProviderConfig[ConstAdjacentIntervals] = ai
			reqProviderConfig[ConstTotpProviderConfig] = reqTotpProviderConfig
			reqProviderConfigsList = append(reqProviderConfigsList, reqProviderConfig)
		}
		req[ConstProviderConfigs] = reqProviderConfigsList
	}

	//return validated multi factor config auth request
	return req, nil
}
