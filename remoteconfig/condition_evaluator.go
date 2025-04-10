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
	"fmt"
	"log"
	"math/big"
)

type conditionEvaluator struct {
	evaluationContext map[string]any
	conditions        []namedCondition
}

const (
	maxConditionRecursionDepth = 10
	randomizationId            = "randomizationId"
	rootNestingLevel           = 0
	totalMicroPercentiles      = 100_000_000
)

const (
	lessThanOrEqual = "LESS_OR_EQUAL"
	greaterThan     = "GREATER_THAN"
	between         = "BETWEEN"
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
	if rid, ok := ce.evaluationContext[randomizationId].(string); ok {
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
	log.Println("Missing or invalid randomizationId (requires a string value) for percent condition.")
	return false
}

func computeInstanceMicroPercentile(seed string, randomizationId string) uint32 {
	seedPrefix := ""
	if len(seed) > 0 {
		seedPrefix = fmt.Sprintf("%s.", seed)
	}
	stringToHash := fmt.Sprintf("%s%s", seedPrefix, randomizationId)

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
