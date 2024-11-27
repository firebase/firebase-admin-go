package remoteconfig

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

const (
	MaxConditionRecursionDepth = 10
	RandomizationId            = "randomizationId"
	MaxPossibleSegments        = 5
	SegmentSeparator           = "."
	MinBitsPossible            = -1
	WhiteSpace = " "
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
	CustomSignal *CustomSignalCondition
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

func (ce *ConditionEvaluator) evaluateCondition(condition *OneOfCondition, nestingLevel int) bool {
	if nestingLevel >= MaxConditionRecursionDepth {
		return false
	}
	if condition.OrCondition != nil {
		return ce.evaluateOrCondition(condition.OrCondition, nestingLevel+1)
	} else if condition.AndCondition != nil {
		return ce.evaluateAndCondition(condition.AndCondition, nestingLevel+1)
	} else if condition.Percent != nil {
		return ce.evaluatePercentCondition(condition.Percent)
	} else if condition.CustomSignal != nil {
		return ce.evaluateCustomSignalCondition(condition.CustomSignal)
	}
	return false
}

func (ce *ConditionEvaluator) evaluateOrCondition(orCondition *OrCondition, nestingLevel int) bool {
	for _, condition := range orCondition.Conditions {
		result := ce.evaluateCondition(&condition, nestingLevel+1)
		// short-circuit evaluation, return true if any of the conditions return true
		if result {
			return true
		}
	}
	return false
}

func (ce *ConditionEvaluator) evaluateAndCondition(andCondition *AndCondition, nestingLevel int) bool {
	for _, condition := range andCondition.Conditions {
		result := ce.evaluateCondition(&condition, nestingLevel+1)
		// short-circuit evaluation, return false if any of the conditions return false
		if !result {
			return false
		}
	}
	return true
}

func (ce *ConditionEvaluator) evaluatePercentCondition(percentCondition *PercentCondition) bool {
	if rid, ok := ce.evaluationContext[RandomizationId]; ok {
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
	if customSignalCondition.CustomSignalOperator == "" || customSignalCondition.CustomSignalKey == "" || len(customSignalCondition.TargetCustomSignalValues) == 0 {
		return false
	}
	if actualCustomSignalValue, ok := ce.evaluationContext[customSignalCondition.CustomSignalKey]; ok {
		switch customSignalCondition.CustomSignalOperator {

		case "STRING_CONTAINS":
			// return true if atleast one target value is contained in the actual custom signal
			return compareStrings(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(actual, target string) bool { return strings.Contains(actual, target) })
		case "STRING_DOES_NOT_CONTAIN":
			// return true if none of the target values are contained in the actual custom signal
			return !compareStrings(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(actual, target string) bool { return strings.Contains(actual, target) })
		case "STRING_EXACTLY_MATCHES":
			return compareStrings(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(actual, target string) bool { return strings.Trim(actual, WhiteSpace) == strings.Trim(target, WhiteSpace) })
		case "STRING_CONTAINS_REGEX":
			return compareStrings(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(actual, targetPattern string) bool {
				result, err := regexp.MatchString(targetPattern, actual)
				if err != nil {
					return false
				}
				return result
			})

		// For numeric operators only one target value is allowed
		case "NUMERIC_LESS_THAN":
			return compareNumbers(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result < 0 })
		case "NUMERIC_LESS_EQUAL":
			return compareNumbers(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result <= 0 })
		case "NUMERIC_EQUAL":
			return compareNumbers(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result == 0 })
		case "NUMERIC_NOT_EQUAL":
			return compareNumbers(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result != 0 })
		case "NUMERIC_GREATER_THAN":
			return compareNumbers(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result > 0 })
		case "NUMERIC_GREATER_EQUAL":
			return compareNumbers(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result >= 0 })

		// For semantic operators only one target value is allowed.
		case "SEMANTIC_VERSION_LESS_THAN":
			return compareSemanticVersions(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result < 0 })
		case "SEMANTIC_VERSION_LESS_EQUAL":
			return compareSemanticVersions(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result <= 0 })
		case "SEMANTIC_VERSION_EQUAL":
			return compareSemanticVersions(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result == 0 })
		case "SEMANTIC_VERSION_NOT_EQUAL":
			return compareSemanticVersions(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result != 0 })
		case "SEMANTIC_VERSION_GREATER_THAN":
			return compareSemanticVersions(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result > 0 })
		case "SEMANTIC_VERSION_GREATER_EQUAL":
			return compareSemanticVersions(customSignalCondition.TargetCustomSignalValues, actualCustomSignalValue, func(result int) bool { return result >= 0 })
		}
	}
	return false
}

// Compares the actual string value of a signal against a list of target values.
// If any of the target values are a match, returns true.
func compareStrings(targetValues []string, actualValue any, predicateFn func(actual, target string) bool) bool {
	var actualAsString string
	switch actualValue := actualValue.(type) {
	case string:
		actualAsString = actualValue
	case int:
		actualAsString = strconv.Itoa(actualValue)
	case float64:
		actualAsString = strconv.FormatFloat(actualValue, 'f', MinBitsPossible, 64)
	default:
		// if the custom signal is passed with a value other than these data types return false -- should throw an error ?
		return false
	}
	for _, target := range targetValues {
		if predicateFn(actualAsString, target) {
			return true
		}
	}
	return false
}

// Compares two numbers against each other.
// Calls the predicate function with  -1, 0, 1 if actual is less than, equal to, or greater than target.
func compareNumbers(targetValue []string, actualValue any, predicateFn func(result int) bool) bool {
	var actualAsFloat float64
	switch actualValue := actualValue.(type) {
	case string:
		var err error
		actualAsFloat, err = strconv.ParseFloat(strings.Trim(actualValue, WhiteSpace), 64)
		if err != nil {
			return false
		}
	case int:
		actualAsFloat = float64(actualValue)
	case float64:
		actualAsFloat = actualValue
	default:
		return false
	}
	targetValueAsFloat, err := strconv.ParseFloat(targetValue[0], 64)
	if err != nil {
		return false
	}
	result := 0
	if actualAsFloat > targetValueAsFloat {
		result = 1
	} else if actualAsFloat < targetValueAsFloat {
		result = -1
	}
	return predicateFn(result)
}

func transformVersionToSegments(version string) ([]int, error) {
	segments := strings.Split(version, SegmentSeparator)
	transformedVersion := make([]int, len(segments))
	for idx := 0; idx < len(segments); idx++ {
		v, err := strconv.Atoi(segments[idx])
		transformedVersion[idx] = v
		if err != nil {
			return []int{}, err
		}
	}
	return transformedVersion, nil
}

// Compares semantic version strings against each other.
// Calls the predicate function with  -1, 0, 1 if actual is less than, equal to, or greater than target.
func compareSemanticVersions(targetValue []string, actualValue any, predicateFn func(result int) bool) bool {
	var actualAsString string
	switch actualValue := actualValue.(type) {
	case string:
		actualAsString = strings.TrimSpace(actualValue)
	case int:
		actualAsString = strconv.Itoa(actualValue)
	case float64:
		actualAsString = strconv.FormatFloat(actualValue, 'f', MinBitsPossible, 64)
	default:
		return false
	}
	version1, err := transformVersionToSegments(actualAsString)
	if err != nil {
		return false
	}
	version2, err := transformVersionToSegments(targetValue[0])
	if err != nil {
		return false
	}
	version1Segments, version2Segments := len(version1), len(version2)
	if version1Segments > MaxPossibleSegments || version2Segments > MaxPossibleSegments {
		return false
	}
	maxSegments := version2Segments
	// append 0's to simplify comparison
	if version1Segments > version2Segments {
		maxSegments = version1Segments
		for idx := version2Segments; idx < version1Segments; idx++ {
			version2 = append(version2, 0)
		}
	} else {
		for idx := version1Segments; idx < version2Segments; idx++ {
			version1 = append(version1, 0)
		}
	}
	for idx := 0; idx < maxSegments; idx++ {
		if version1[idx] > version2[idx] {
			return predicateFn(1)
		} else if version1[idx] < version2[idx] {
			return predicateFn(-1)
		}
	}
	return predicateFn(0)
}
