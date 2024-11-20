package remoteconfig

import (
	"crypto/sha256"
	"fmt"
	"math/big"
)

// Represents a Remote Config condition in the dataplane.
// A condition targets a specific group of users. A list of these conditions
// comprise part of a Remote Config template.
type NamedCondition struct {
	// A non-empty and unique name of this condition.
	Name string

	// The logic of this condition.
	// See the documentation on https://firebase.google.com/docs/remote-config/condition-reference
	// for the expected syntax of this field.
	Condition OneOfCondition
}

// Represents a condition that may be one of several types.
// Only the first defined field will be processed.
type OneOfCondition struct {
	// Makes this condition an OR condition.
	OrCondition *OrCondition

	//  Makes this condition an AND condition.
	AndCondition *AndCondition

	// Makes this condition a percent condition.
	Percent *PercentCondition

	// Makes this condition a custom signal condition.
	CustomSignalCondition *CustomSignalCondition
}

// Represents a collection of conditions that evaluate to true if all are true.
type AndCondition struct {
	Conditions []OneOfCondition
}

// Represents a collection of conditions that evaluate to true if any are true.
type OrCondition struct {
	Conditions []OneOfCondition
}

// Represents a condition that compares the instance pseudo-random percentile to a given limit.
type PercentCondition struct {
	//  The choice of percent operator to determine how to compare targets to percent(s).
	PercentOperator string

	// The seed used when evaluating the hash function to map an instance to
	// a value in the hash space. This is a string which can have 0 - 32
	// characters and can contain ASCII characters [-_.0-9a-zA-Z].The string is case-sensitive.
	Seed string

	// The limit of percentiles to target in micro-percents when
	// using the LESS_OR_EQUAL and GREATER_THAN operators. The value must
	// be in the range [0 and 100000000].
	MicroPercent uint32

	// The micro-percent interval to be used with the BETWEEN operator.
	MicroPercentRange MicroPercentRange
}

// Represents the limit of percentiles to target in micro-percents.
// The value must be in the range [0 and 100000000]
type MicroPercentRange struct {
	// The lower limit of percentiles to target in micro-percents.
	// The value must be in the range [0 and 100000000].
	MicroPercentLowerBound uint32

	// The upper limit of percentiles to target in micro-percents.
	// The value must be in the range [0 and 100000000].
	MicroPercentUpperBound uint32
}

// Represents a condition that compares provided signals against a target value.
type CustomSignalCondition struct {
	// The choice of custom signal operator to determine how to compare targets
	// to value(s).
	CustomSignalOperator string

	// The key of the signal set in the EvaluationContext
	CustomSignalKey string

	// A list of at most 100 target custom signal values. For numeric operators,
	// this will have exactly ONE target value.
	TargetCustomSignalValues []string
}

// Type representing a Remote Config parameter value data type.
type ParameterValueType string

const (
	String  ParameterValueType = "STRING"
	Boolean ParameterValueType = "BOOLEAN"
	Number  ParameterValueType = "NUMBER"
	Json    ParameterValueType = "JSON"
)

// Structure representing a Remote Config template version.
// Output only, except for the version description. Contains metadata about a particular
// version of the Remote Config template. All fields are set at the time the specified Remote Config template is published.
type Version struct {
	// The version number of a Remote Config template.
	VersionNumber string

	// The timestamp of when this version of the Remote Config template was written to the
	// Remote Config backend.
	UpdateTime string

	// The origin of the template update action.
	UpdateOrigin string

	// The type of the template update action.
	UpdateType string

	// Aggregation of all metadata fields about the account that performed the update.
	UpdateUser RemoteConfigUser

	// The user-provided description of the corresponding Remote Config template.
	Description string

	// The version number of the Remote Config template that has become the current version
	// due to a rollback. Only present if this version is the result of a rollback.
	RollbackSource string

	// Indicates whether this Remote Config template was published before version history was supported.
	IsLegacy bool
}

// Represents a Remote Config user.
type RemoteConfigUser struct {
	// Email address. Output only.
	Email string

	// Display name. Output only.
	Name string

	// Image URL. Output only.
	ImageUrl string
}

type ConditionEvaluator struct {
	evaluationContext map[string]interface{}
	conditions        []NamedCondition
}

func (ce *ConditionEvaluator) hashSeededRandomizationId(seedRid string) *big.Int {
	hash := sha256.New()
	hash.Write([]byte(seedRid))
	// Get the resulting hash as a byte slice
	hashBytes := hash.Sum(nil)
	// Convert the hash bytes to a big.Int. The "0x" prefix is implicit in the conversion from hex to big.Int.
	return new(big.Int).SetBytes(hashBytes)
}

func (ce *ConditionEvaluator) evaluateConditions() ([]string, map[string]bool) {
	// go does not maintain the order of insertion in a map - https://go.dev/blog/maps#iteration-order
	orderedConditions := make([]string, 0, len(ce.conditions))
	evaluatedConditions := make(map[string]bool)
	for _, condition := range ce.conditions {
		orderedConditions = append(orderedConditions, condition.Name)
		evaluatedConditions[condition.Name] = ce.evaluateCondition(&condition.Condition, 0)
	}
	return orderedConditions, evaluatedConditions
}

func (ce *ConditionEvaluator) evaluateCondition(condition *OneOfCondition, level int) bool {
	if condition.OrCondition != nil {
		return ce.evaluateOrCondition(condition.OrCondition, level+1)
	} else if condition.AndCondition != nil {
		return ce.evaluateAndCondition(condition.AndCondition, level+1)
	} else if condition.Percent != nil {
		return ce.evaluatePercentCondition(condition.Percent)
	} else if condition.CustomSignalCondition != nil {
		return ce.evaluateCustomSignalCondition(condition.CustomSignalCondition)
	}
	return false
}

func (ce *ConditionEvaluator) evaluateOrCondition(orCondition *OrCondition, level int) bool {
	for _, condition := range orCondition.Conditions {
		result := ce.evaluateCondition(&condition, level+1)
		if result {
			return true
		}
	}
	return false
}

func (ce *ConditionEvaluator) evaluateAndCondition(andCondition *AndCondition, level int) bool {
	for _, condition := range andCondition.Conditions {
		result := ce.evaluateCondition(&condition, level+1)
		if result {
			return false
		}
	}
	return true
}

func (ce *ConditionEvaluator) evaluatePercentCondition(percentCondition *PercentCondition) bool {
	if rid, ok := ce.evaluationContext["randomizationId"]; ok {
		if percentCondition.PercentOperator == "" {
			return false
		}
		stringToHash := fmt.Sprintf("%s.%s", percentCondition.Seed, rid)
		hash := ce.hashSeededRandomizationId(stringToHash)
		instanceMicroPercentileBigInt := new(big.Int).Mod(hash, big.NewInt(100000000))
		// can safely convert to uint32 since the range is 0 to 100,000,000
		var instanceMicroPercentile uint32 = uint32(instanceMicroPercentileBigInt.Int64())
		switch percentCondition.PercentOperator {
		case "LESS_OR_EQUAL":
			return instanceMicroPercentile <= percentCondition.MicroPercent
		case "GREATER_THAN":
			return instanceMicroPercentile > percentCondition.MicroPercent
		case "BETWEEN":
			return instanceMicroPercentile > percentCondition.MicroPercentRange.MicroPercentLowerBound && instanceMicroPercentile <= percentCondition.MicroPercentRange.MicroPercentUpperBound
		case "UNKNOWN":
		default:
		}
	}
	return false
}

func (ce *ConditionEvaluator) evaluateCustomSignalCondition(customSignalCondition *CustomSignalCondition) bool {
	return true
}
