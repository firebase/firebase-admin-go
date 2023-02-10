package auth

import (
	"fmt"
)

const (
	enabled  = "ENABLED"
	disabled = "DISABLED"
)

var validAuthFactors = map[string]bool{
	"PHONE_SMS": true,
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

func validateAndConvertMultiFactorConfig(multiFactorConfig interface{}) (nestedMap, error) {
	if multiFactorConfig == nil {
		return nil, fmt.Errorf(`multiFactorConfig must be defined`)
	}
	req := make(map[string]interface{})
	mfaMap, ok := multiFactorConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`multiFactorConfig must be a valid MultiFactorConfig type`)
	}

	//validate mfa config keys
	validMfaKeys := make(map[string]bool)
	validMfaKeys["state"] = true
	validMfaKeys["factorIds"] = true
	validMfaKeys["providerConfigs"] = true
	for k := range mfaMap {
		if !validMfaKeys[k] {
			return nil, fmt.Errorf(`"%s" is not a valid MultiFactorConfig parameter`, k)
		}
	}

	//validate mfa.state
	state, ok := mfaMap["state"]
	if !ok {
		return nil, fmt.Errorf(`multiFactorConfig.state should be defined`)
	}
	s, ok := state.(string)
	if !ok || (s != enabled && s != disabled) {
		return nil, fmt.Errorf(`multiFactorConfig.state must be either "ENABLED" or "DISABLED"`)
	}
	req["state"] = s

	//validate mfa.factorIds
	factorIds, ok := mfaMap["factorIds"]
	if ok {
		fmt.Println(factorIds)
		var authFactorIds []string
		fi, ok := factorIds.([]string)
		if !ok {
			return nil, fmt.Errorf(`multiFactorConfig.factorIds must be a defined list of AuthFactor type strings`)
		}
		for _, f := range fi {
			if !validAuthFactors[f] {
				return nil, fmt.Errorf(`factorId must be a valid AuthFactor type string`)
			}
			authFactorIds = append(authFactorIds, f)
		}
		req["enabledProviders"] = make([]string, len(authFactorIds))
		copy(req["enabledProviders"].([]string), authFactorIds)
	}

	//validate provider configs
	providerConfigs, ok := mfaMap["providerConfigs"]
	if ok {
		pc, ok := providerConfigs.([]map[string]interface{})
		if !ok {
			return nil, fmt.Errorf(`multiFactorConfig.providerConfigs must be a list of ProviderConfigs`)
		}
		var reqProviderConfigsList []map[string]interface{}
		for _, providerConfig := range pc {
			reqProviderConfig := make(map[string]interface{})

			//validate providerConfig struct keys
			validConfigKeys := make(map[string]bool)
			validConfigKeys["state"] = true
			validConfigKeys["totpProviderConfig"] = true
			for k := range providerConfig {
				if !validConfigKeys[k] {
					return nil, fmt.Errorf(`"%s" is not a valid providerConfig parameter`, k)
				}
			}

			//validate providerConfig.state
			state, ok := providerConfig["state"]
			if !ok {
				return nil, fmt.Errorf(`providerConfig.state should be defined`)
			}
			s, ok := state.(string)
			if !ok || (s != enabled && s != disabled) {
				return nil, fmt.Errorf(`providerConfig.state must be either "ENABLED" or "DISABLED"`)
			}
			reqProviderConfig["state"] = s

			//validate providerConfig.totpProviderConfig
			totpProviderConfig, ok := providerConfig["totpProviderConfig"]
			if !ok {
				return nil, fmt.Errorf(`providerConfig.totpProviderConfig should be present`)
			}
			tpc, ok := totpProviderConfig.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf(`totpProviderConfig must be of type TotpProviderConfig`)
			}

			//validate totpProviderConfig keys
			validTotpConfigKeys := make(map[string]bool)
			validTotpConfigKeys["adjacentIntervals"] = true
			for k := range tpc {
				if !validTotpConfigKeys[k] {
					return nil, fmt.Errorf(`"%s" is not a valid totpProviderConfig parameter`, k)
				}
			}
			reqTotpProviderConfig := make(map[string]interface{})

			//validate adjacentIntervals if present
			adjacentIntervals, ok := tpc["adjacentIntervals"]
			if ok {
				ai, ok := adjacentIntervals.(int)
				if !ok || !(0 <= ai && ai <= 10) {
					return nil, fmt.Errorf(`adjacentIntervals must be a valid number between 0 and 10 (both inclusive)`)
				}
				reqTotpProviderConfig["adjacentIntervals"] = ai
			}
			reqProviderConfig["totpProviderConfig"] = reqTotpProviderConfig
			reqProviderConfigsList = append(reqProviderConfigsList, reqProviderConfig)
		}
		req["providerConfigs"] = reqProviderConfigsList
	}

	//return validated multi factor config auth request
	return req, nil
}
