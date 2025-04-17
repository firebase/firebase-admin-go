// Copyright 2025 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package remoteconfig

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

type conditionEvaluator struct {
	evaluationContext map[string]any
	conditions        []namedCondition
}

const (
	maxConditionRecursionDepth = 10
	rootNestingLevel           = 0
	doublePrecision            = 64
)

const (
	randomizationID       = "randomizationID"
	totalMicroPercentiles = 100_000_000
	lessThanOrEqual       = "LESS_OR_EQUAL"
	greaterThan           = "GREATER_THAN"
	between               = "BETWEEN"
)

const (
	whiteSpace = " "

	stringContains       = "STRING_CONTAINS"
	stringDoesNotContain = "STRING_DOES_NOT_CONTAIN"
	stringExactlyMatches = "STRING_EXACTLY_MATCHES"
	stringContainsRegex  = "STRING_CONTAINS_REGEX"

	numericLessThan      = "NUMERIC_LESS_THAN"
	numericLessThanEqual = "NUMERIC_LESS_EQUAL"
	numericEqual         = "NUMERIC_EQUAL"
	numericNotEqual      = "NUMERIC_NOT_EQUAL"
	numericGreaterThan   = "NUMERIC_GREATER_THAN"
	numericGreaterEqual  = "NUMERIC_GREATER_EQUAL"
)

func (ce *conditionEvaluator) evaluateConditions() map[string]bool {
	evaluatedConditions := make(map[string]bool)
	for _, condition := range ce.conditions {
		evaluatedConditions[condition.Name] = ce.evaluateCondition(condition.Condition, rootNestingLevel)
	}
	return evaluatedConditions
}

func (ce *conditionEvaluator) evaluateCondition(condition *oneOfCondition, nestingLevel int) bool {
	if nestingLevel >= maxConditionRecursionDepth {
		log.Println("Maximum recursion depth is exceeded.")
		return false
	}

	if condition.Boolean != nil {
		return *condition.Boolean
	} else if condition.OrCondition != nil {
		return ce.evaluateOrCondition(condition.OrCondition, nestingLevel+1)
	} else if condition.AndCondition != nil {
		return ce.evaluateAndCondition(condition.AndCondition, nestingLevel+1)
	} else if condition.Percent != nil {
		return ce.evaluatePercentCondition(condition.Percent)
	} else if condition.CustomSignal != nil {
		return ce.evaluateCustomSignalCondition(condition.CustomSignal)
	}
	log.Println("Unknown condition type encountered.")
	return false
}

func (ce *conditionEvaluator) evaluateOrCondition(orCondition *orCondition, nestingLevel int) bool {
	for _, condition := range orCondition.Conditions {
		result := ce.evaluateCondition(&condition, nestingLevel+1)
		// short-circuit evaluation, return true if any of the conditions return true
		if result {
			return true
		}
	}
	return false
}

func (ce *conditionEvaluator) evaluateAndCondition(andCondition *andCondition, nestingLevel int) bool {
	for _, condition := range andCondition.Conditions {
		result := ce.evaluateCondition(&condition, nestingLevel+1)
		// short-circuit evaluation, return false if any of the conditions return false
		if !result {
			return false
		}
	}
	return true
}

func (ce *conditionEvaluator) evaluatePercentCondition(percentCondition *percentCondition) bool {
	if rid, ok := ce.evaluationContext[randomizationID].(string); ok {
		if percentCondition.PercentOperator == "" {
			log.Println("Missing percent operator for percent condition.")
			return false
		}
		instanceMicroPercentile := computeInstanceMicroPercentile(percentCondition.Seed, rid)
		switch percentCondition.PercentOperator {
		case lessThanOrEqual:
			return instanceMicroPercentile <= percentCondition.MicroPercent
		case greaterThan:
			return instanceMicroPercentile > percentCondition.MicroPercent
		case between:
			return instanceMicroPercentile > percentCondition.MicroPercentRange.MicroPercentLowerBound && instanceMicroPercentile <= percentCondition.MicroPercentRange.MicroPercentUpperBound
		default:
			log.Printf("Unknown percent operator: %s\n", percentCondition.PercentOperator)
			return false
		}
	}
	log.Println("Missing or invalid randomizationID (requires a string value) for percent condition.")
	return false
}

func computeInstanceMicroPercentile(seed string, randomizationID string) uint32 {
	seedPrefix := ""
	if len(seed) > 0 {
		seedPrefix = fmt.Sprintf("%s.", seed)
	}
	stringToHash := fmt.Sprintf("%s%s", seedPrefix, randomizationID)

	hash := sha256.New()
	hash.Write([]byte(stringToHash))
	// Calculate the final SHA-256 hash as a byte slice (32 bytes).
	hashBytes := hash.Sum(nil)

	hashBigInt := new(big.Int).SetBytes(hashBytes)
	// Convert the hash bytes to a big.Int. The "0x" prefix is implicit in the conversion from hex to big.Int.
	instanceMicroPercentileBigInt := new(big.Int).Mod(hashBigInt, big.NewInt(totalMicroPercentiles))
	// Can safely convert to uint32 since the range of instanceMicroPercentile is 0 to 100_000_000; range of uint32 is 0 to 4_294_967_295.
	return uint32(instanceMicroPercentileBigInt.Int64())
}

func (ce *conditionEvaluator) evaluateCustomSignalCondition(customSignalCondition *customSignalCondition) bool {
	if !customSignalCondition.isValid() {
		return false
	}
	csVal, ok := ce.evaluationContext[customSignalCondition.CustomSignalKey]
	if !ok {
		log.Printf("Custom signal key: %s, missing from context\n", customSignalCondition.CustomSignalKey)
		return false
	}

	fmt.Println("CUSTOM SIGNALS ---- ")
	fmt.Println("signal value from context ", csVal)
	fmt.Println(customSignalCondition.TargetCustomSignalValues)

	switch customSignalCondition.CustomSignalOperator {
	case stringContains:
		return compareStrings(customSignalCondition.TargetCustomSignalValues, csVal, func(csVal, target string) bool { return strings.Contains(csVal, target) })
	case stringDoesNotContain:
		return !compareStrings(customSignalCondition.TargetCustomSignalValues, csVal, func(csVal, target string) bool { return strings.Contains(csVal, target) })
	case stringExactlyMatches:
		return compareStrings(customSignalCondition.TargetCustomSignalValues, csVal, func(csVal, target string) bool {
			return strings.Trim(csVal, whiteSpace) == strings.Trim(target, whiteSpace)
		})
	case stringContainsRegex:
		return compareStrings(customSignalCondition.TargetCustomSignalValues, csVal, func(csVal, targetPattern string) bool {
			result, err := regexp.MatchString(targetPattern, csVal)
			if err != nil {
				return false
			}
			return result
		})

	// For numeric operators only one target value is allowed
	case numericLessThan:
		return compareNumbers(customSignalCondition.TargetCustomSignalValues[0], csVal, func(result int) bool { return result < 0 })
	case numericLessThanEqual:
		return compareNumbers(customSignalCondition.TargetCustomSignalValues[0], csVal, func(result int) bool { return result <= 0 })
	case numericEqual:
		return compareNumbers(customSignalCondition.TargetCustomSignalValues[0], csVal, func(result int) bool { return result == 0 })
	case numericNotEqual:
		return compareNumbers(customSignalCondition.TargetCustomSignalValues[0], csVal, func(result int) bool { return result != 0 })
	case numericGreaterThan:
		return compareNumbers(customSignalCondition.TargetCustomSignalValues[0], csVal, func(result int) bool { return result > 0 })
	case numericGreaterEqual:
		return compareNumbers(customSignalCondition.TargetCustomSignalValues[0], csVal, func(result int) bool { return result >= 0 })
	}

	return false
}

func (cs *customSignalCondition) isValid() bool {
	if cs.CustomSignalOperator == "" || cs.CustomSignalKey == "" || len(cs.TargetCustomSignalValues) == 0 {
		log.Println("Missing operator, key, or target values for custom signal condition.")
		return false
	}
	return true
}

// Compares the actual string value of a signal against a list of target values.
// If any of the target values are a match, returns true.
func compareStrings(targetCustomSignalValues []string, csVal any, compare func(csVal, target string) bool) bool {
	csValStr, ok := csVal.(string)
	if !ok {
		if jsonBytes, err := json.Marshal(csVal); err == nil {
			csValStr = string(jsonBytes)
		} else {
			log.Printf("failed to parse custom signal value '%v' as a string\n", csVal)
			return false
		}
	}

	for _, target := range targetCustomSignalValues {
		if compare(csValStr, target) {
			return true
		}
	}
	return false
}

// Compares two numbers against each other.
// Calls the predicate function with  -1, 0, 1 if actual is less than, equal to, or greater than target.
func compareNumbers(targetCustomSignalValue string, csVal any, compare func(result int) bool) bool {
	targetFloat, err := strconv.ParseFloat(strings.Trim(targetCustomSignalValue, whiteSpace), doublePrecision)
	if err != nil {
		log.Printf("Failed to convert target custom signal value '%v' from string to number: %v", targetCustomSignalValue, err)
		return false
	}
	var csValFloat float64
	switch csVal := csVal.(type) {
	case float32:
		csValFloat = float64(csVal)
	case float64:
		csValFloat = csVal
	case int8:
		csValFloat = float64(csVal)
	case int:
		csValFloat = float64(csVal)
	case int16:
		csValFloat = float64(csVal)
	case int32:
		csValFloat = float64(csVal)
	case int64:
		csValFloat = float64(csVal)
	case uint8:
		csValFloat = float64(csVal)
	case uint:
		csValFloat = float64(csVal)
	case uint16:
		csValFloat = float64(csVal)
	case uint32:
		csValFloat = float64(csVal)
	case uint64:
		csValFloat = float64(csVal)
	case bool:
		if csVal {
			csValFloat = 1
		} else {
			csValFloat = 0
		}
	case string:
		csValFloat, err = strconv.ParseFloat(strings.Trim(csVal, whiteSpace), doublePrecision)
		if err != nil {
			log.Printf("Failed to convert custom signal value '%v' from string to number: %v", csVal, err)
			return false
		}
	default:
		log.Printf("Cannot parse custom signal value '%v' of type %T as a number", csVal, csVal)
		return false
	}
	result := 0
	if csValFloat > targetFloat {
		result = 1
	} else if csValFloat < targetFloat {
		result = -1
	}
	r := compare(result)
	return r
}
