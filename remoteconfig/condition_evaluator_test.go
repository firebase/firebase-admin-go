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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const (
	isEnabled           = "is_enabled"
	customSignalKey     = "customSignalKey"
	premium             = "premium"
	testRandomizationID = "123"
	testSeed            = "abcdef"

	leadingWhiteSpaceCountTarget  = 3
	trailingWhiteSpaceCountTarget = 5
	leadingWhiteSpaceCountActual  = 4
	trailingWhiteSpaceCountActual = 2
)

type customSignalTestCase struct {
	targets string
	actual  any
	outcome bool
}

func createNamedCondition(name string, condition oneOfCondition) namedCondition {
	nc := namedCondition{
		Name:      name,
		Condition: &condition,
	}
	return nc
}

func evaluateConditionsAndReportResult(t *testing.T, nc namedCondition, conditionName string, context map[string]any, outcome bool) {
	ce := conditionEvaluator{
		conditions:        []namedCondition{nc},
		evaluationContext: context,
	}
	ec := ce.evaluateConditions()
	value, ok := ec[conditionName]
	if !ok {
		t.Fatalf("condition %q was not found in evaluated conditions", conditionName)
	}
	if value != outcome {
		t.Errorf("condition evaluation for %q = %v, want = %v", conditionName, value, outcome)
	}
}

// Returns the number of assignments which evaluate to true for the specified percent condition.
// This method randomly generates the ids for each assignment for this purpose.
func evaluateRandomAssignments(numOfAssignments int, condition namedCondition) int {
	evalTrueCount := 0
	for i := 0; i < numOfAssignments; i++ {
		context := map[string]any{randomizationID: fmt.Sprintf("random-%d", i)}
		ce := conditionEvaluator{
			conditions:        []namedCondition{condition},
			evaluationContext: context,
		}
		ec := ce.evaluateConditions()
		if value, ok := ec[isEnabled]; ok && value {
			evalTrueCount++
		}
	}
	return evalTrueCount
}

func runCustomSignalTestCase(operator string, t *testing.T) func(customSignalTestCase) {
	return func(tc customSignalTestCase) {
		description := fmt.Sprintf("Evaluates operator %v with targets %v and actual %v to outcome %v", operator, tc.targets, tc.actual, tc.outcome)
		t.Run(description, func(t *testing.T) {
			condition := createNamedCondition(isEnabled, oneOfCondition{
				CustomSignal: &customSignalCondition{
					CustomSignalOperator:     operator,
					CustomSignalKey:          customSignalKey,
					TargetCustomSignalValues: strings.Split(tc.targets, ","),
				},
			})
			evaluateConditionsAndReportResult(t, condition, isEnabled, map[string]any{customSignalKey: tc.actual}, tc.outcome)
		})
	}
}

func runCustomSignalTestCaseWithWhiteSpaces(operator string, t *testing.T) func(customSignalTestCase) {
	return func(tc customSignalTestCase) {
		targetsWithWhiteSpaces := []string{}
		for _, target := range strings.Split(tc.targets, ",") {
			targetsWithWhiteSpaces = append(targetsWithWhiteSpaces, addLeadingAndTrailingWhiteSpaces(target, leadingWhiteSpaceCountTarget, trailingWhiteSpaceCountTarget))
		}
		runCustomSignalTestCase(operator, t)(customSignalTestCase{
			outcome: tc.outcome,
			actual:  addLeadingAndTrailingWhiteSpaces(tc.actual, leadingWhiteSpaceCountActual, trailingWhiteSpaceCountActual),
			targets: strings.Join(targetsWithWhiteSpaces, ","),
		})
	}
}

func addLeadingAndTrailingWhiteSpaces(v any, leadingSpacesCount int, trailingSpacesCount int) string {
	vStr, ok := v.(string)
	if !ok {
		if jsonBytes, err := json.Marshal(v); err == nil {
			vStr = string(jsonBytes)
		}
	}
	return strings.Repeat(whiteSpace, leadingSpacesCount) + vStr + strings.Repeat(whiteSpace, trailingSpacesCount)
}

func TestEvaluateEmptyOrCondition(t *testing.T) {
	condition := createNamedCondition(isEnabled, oneOfCondition{
		OrCondition: &orCondition{},
	})
	evaluateConditionsAndReportResult(t, condition, isEnabled, map[string]any{}, false)
}

func TestEvaluateEmptyOrAndCondition(t *testing.T) {
	condition := createNamedCondition(isEnabled, oneOfCondition{
		OrCondition: &orCondition{
			Conditions: []oneOfCondition{
				{
					AndCondition: &andCondition{},
				},
			},
		},
	})
	evaluateConditionsAndReportResult(t, condition, isEnabled, map[string]any{}, true)
}

func TestEvaluateOrConditionShortCircuit(t *testing.T) {
	boolFalse := false
	boolTrue := true
	condition := createNamedCondition(isEnabled, oneOfCondition{
		OrCondition: &orCondition{
			Conditions: []oneOfCondition{
				{
					Boolean: &boolFalse,
				},
				{
					Boolean: &boolTrue,
				},
				{
					Boolean: &boolFalse,
				},
			},
		},
	})
	evaluateConditionsAndReportResult(t, condition, isEnabled, map[string]any{}, true)
}

func TestEvaluateAndConditionShortCircuit(t *testing.T) {
	boolFalse := false
	boolTrue := true
	condition := createNamedCondition(isEnabled, oneOfCondition{
		AndCondition: &andCondition{
			Conditions: []oneOfCondition{
				{
					Boolean: &boolTrue,
				},
				{
					Boolean: &boolFalse,
				},
				{
					Boolean: &boolTrue,
				},
			},
		},
	})
	evaluateConditionsAndReportResult(t, condition, isEnabled, map[string]any{}, false)
}

func TestPercentConditionWithoutRandomizationId(t *testing.T) {
	condition := createNamedCondition(isEnabled, oneOfCondition{
		Percent: &percentCondition{
			PercentOperator: between,
			Seed:            testSeed,
			MicroPercentRange: microPercentRange{
				MicroPercentLowerBound: 0,
				MicroPercentUpperBound: 1_000_000,
			},
		},
	})
	evaluateConditionsAndReportResult(t, condition, isEnabled, map[string]any{}, false)
}

func TestUnknownPercentOperator(t *testing.T) {
	condition := createNamedCondition(isEnabled, oneOfCondition{
		Percent: &percentCondition{
			PercentOperator: "UNKNOWN",
			Seed:            testSeed,
			MicroPercentRange: microPercentRange{
				MicroPercentLowerBound: 0,
				MicroPercentUpperBound: 1_000_000,
			},
		},
	})
	evaluateConditionsAndReportResult(t, condition, isEnabled, map[string]any{}, false)
}

func TestEmptyPercentOperator(t *testing.T) {
	condition := createNamedCondition(isEnabled, oneOfCondition{
		Percent: &percentCondition{
			Seed: testSeed,
			MicroPercentRange: microPercentRange{
				MicroPercentLowerBound: 0,
				MicroPercentUpperBound: 1_000_000,
			},
		},
	})
	evaluateConditionsAndReportResult(t, condition, isEnabled, map[string]any{}, false)
}

func TestInvalidRandomizationIdType(t *testing.T) {
	// randomizationID is expected to be a string
	condition := createNamedCondition(isEnabled, oneOfCondition{
		Percent: &percentCondition{
			Seed: testSeed,
			MicroPercentRange: microPercentRange{
				MicroPercentLowerBound: 0,
				MicroPercentUpperBound: 1_000_000,
			},
		},
	})

	invalidRandomizationIDTestCases := []struct {
		randomizationID any
	}{
		{randomizationID: 123},
		{randomizationID: true},
		{randomizationID: 123.4},
		{randomizationID: "{\"hello\": \"world\"}"},
	}
	for _, tc := range invalidRandomizationIDTestCases {
		description := fmt.Sprintf("RandomizationId %v of type %s", tc.randomizationID, reflect.TypeOf(tc.randomizationID))
		t.Run(description, func(t *testing.T) {
			evaluateConditionsAndReportResult(t, condition, isEnabled, map[string]any{randomizationID: tc.randomizationID}, false)
		})
	}

}

func TestInstanceMicroPercentileComputation(t *testing.T) {
	percentTestCases := []struct {
		seed                    string
		randomizationID         string
		expectedMicroPercentile uint32
	}{
		{seed: "1", randomizationID: "one", expectedMicroPercentile: 64146488},
		{seed: "2", randomizationID: "two", expectedMicroPercentile: 76516209},
		{seed: "3", randomizationID: "three", expectedMicroPercentile: 6701947},
		{seed: "4", randomizationID: "four", expectedMicroPercentile: 85000289},
		{seed: "5", randomizationID: "five", expectedMicroPercentile: 2514745},
		{seed: "", randomizationID: "😊", expectedMicroPercentile: 9911325},
		{seed: "", randomizationID: "😀", expectedMicroPercentile: 62040281},
		{seed: "hêl£o", randomizationID: "wørlÐ", expectedMicroPercentile: 67411682},
		{seed: "řemøťe", randomizationID: "çōnfįġ", expectedMicroPercentile: 19728496},
		{seed: "long", randomizationID: strings.Repeat(".", 100), expectedMicroPercentile: 39278120},
		{seed: "very-long", randomizationID: strings.Repeat(".", 1000), expectedMicroPercentile: 71699042},
	}

	for _, tc := range percentTestCases {
		description := fmt.Sprintf("Instance micro-percentile for seed %s & randomization_id %s", tc.seed, tc.randomizationID)
		t.Run(description, func(t *testing.T) {
			actualMicroPercentile := computeInstanceMicroPercentile(tc.seed, tc.randomizationID)
			if tc.expectedMicroPercentile != actualMicroPercentile {
				t.Errorf("instanceMicroPercentile = %d, want %d", actualMicroPercentile, tc.expectedMicroPercentile)

			}
		})
	}
}

func TestPercentConditionMicroPercent(t *testing.T) {
	microPercentTestCases := []struct {
		description  string
		operator     string
		microPercent uint32
		outcome      bool
	}{
		{
			description:  "Evaluate LESS_OR_EQUAL to true when MicroPercent is max",
			operator:     "LESS_OR_EQUAL",
			microPercent: 100_000_000,
			outcome:      true,
		},
		{
			description:  "Evaluate LESS_OR_EQUAL to false when MicroPercent is min",
			operator:     "LESS_OR_EQUAL",
			microPercent: 0,
			outcome:      false,
		},
		{
			description: "Evaluate LESS_OR_EQUAL to false when MicroPercent is not set (MicroPercent should use zero)",
			operator:    "LESS_OR_EQUAL",
			outcome:     false,
		},
		{
			description: "Evaluate GREATER_THAN to true when MicroPercent is not set (MicroPercent should use zero)",
			operator:    "GREATER_THAN",
			outcome:     true,
		},
		{
			description:  "Evaluate GREATER_THAN max to false",
			operator:     "GREATER_THAN",
			outcome:      false,
			microPercent: 100_000_000,
		},
		{
			description:  "Evaluate LESS_OR_EQUAL to 9571542 to true",
			operator:     "LESS_OR_EQUAL",
			microPercent: 9_571_542, // instanceMicroPercentile of abcdef.123 (testSeed.testRandomizationID) is 9_571_542
			outcome:      true,
		},
		{
			description:  "Evaluate greater than 9571542 to true",
			operator:     "GREATER_THAN",
			microPercent: 9_571_541, // instanceMicroPercentile of abcdef.123 (testSeed.testRandomizationID) is 9_571_542
			outcome:      true,
		},
	}
	for _, tc := range microPercentTestCases {
		t.Run(tc.description, func(t *testing.T) {
			percentCondition := createNamedCondition(isEnabled, oneOfCondition{
				Percent: &percentCondition{
					PercentOperator: tc.operator,
					MicroPercent:    tc.microPercent,
					Seed:            testSeed,
				},
			})
			evaluateConditionsAndReportResult(t, percentCondition, isEnabled, map[string]any{"randomizationID": testRandomizationID}, tc.outcome)
		})
	}
}

func TestPercentConditionMicroPercentRange(t *testing.T) {
	// These tests verify that the percentage-based conditions correctly target the intended proportion of users over many random evaluations.
	// The results are checked against expected statistical distributions to ensure accuracy within a defined tolerance (3 standard deviations).
	microPercentTestCases := []struct {
		description    string
		operator       string
		microPercentLb uint32
		microPercentUb uint32
		outcome        bool
	}{
		{
			description: "Evaluate to false when microPercentRange is not set",
			operator:    "BETWEEN",
			outcome:     false,
		},
		{
			description:    "Evaluate to false when upper bound is not set",
			microPercentLb: 0,
			operator:       "BETWEEN",
			outcome:        false,
		},
		{
			description:    "Evaluate to true when lower bound is not set and upper bound is max",
			microPercentUb: 100_000_000,
			operator:       "BETWEEN",
			outcome:        true,
		},
		{
			description:    "Evaluate to true when between lower and upper bound", // instanceMicroPercentile of abcdef.123 (testSeed.testRandomizationID) is 9_571_542
			microPercentLb: 9_000_000,
			microPercentUb: 9_571_542, // interval is (9_000_000, 9_571_542]
			operator:       "BETWEEN",
			outcome:        true,
		},
		{
			description:    "Evaluate to false when lower and upper bounds are equal",
			microPercentLb: 98_000_000,
			microPercentUb: 98_000_000,
			operator:       "BETWEEN",
			outcome:        false,
		},
		{
			description:    "Evaluate to false when not between 9_400_000 and 9_500_000", // instanceMicroPercentile of abcdef.123 (testSeed.testRandomizationID) is 9_571_542
			microPercentLb: 9_400_000,
			microPercentUb: 9_500_000,
			operator:       "BETWEEN",
			outcome:        false,
		},
	}
	for _, tc := range microPercentTestCases {
		t.Run(tc.description, func(t *testing.T) {
			percentCondition := createNamedCondition(isEnabled, oneOfCondition{
				Percent: &percentCondition{
					PercentOperator: tc.operator,
					MicroPercentRange: microPercentRange{
						MicroPercentLowerBound: tc.microPercentLb,
						MicroPercentUpperBound: tc.microPercentUb,
					},
					Seed: testSeed,
				},
			})
			evaluateConditionsAndReportResult(t, percentCondition, isEnabled, map[string]any{randomizationID: testRandomizationID}, tc.outcome)
		})
	}
}

// Statistically validates that percentage conditions accurately target the intended proportion of users over many random evaluations.
func TestPercentConditionProbabilisticEvaluation(t *testing.T) {
	probabilisticEvalTestCases := []struct {
		description string
		condition   namedCondition
		assignments int
		baseline    int
		tolerance   int
	}{
		{
			description: "Evaluate less or equal to 10% to approx 10%",
			condition: createNamedCondition(isEnabled, oneOfCondition{
				Percent: &percentCondition{
					PercentOperator: lessThanOrEqual,
					MicroPercent:    10_000_000,
				},
			}),
			assignments: 100_000,
			baseline:    10000,
			tolerance:   284, // 284 is 3 standard deviations for 100k trials with 10% probability.
		},
		{
			description: "Evaluate between 0 to 10% to approx 10%",
			condition: createNamedCondition(isEnabled, oneOfCondition{
				Percent: &percentCondition{
					PercentOperator: between,
					MicroPercentRange: microPercentRange{
						MicroPercentUpperBound: 10_000_000,
					},
				},
			}),
			assignments: 100_000,
			baseline:    10000,
			tolerance:   284, // 284 is 3 standard deviations for 100k trials with 10% probability.
		},
		{
			description: "Evaluate greater than 10% to approx 90%",
			condition: createNamedCondition(isEnabled, oneOfCondition{
				Percent: &percentCondition{
					PercentOperator: greaterThan,
					MicroPercent:    10_000_000,
				},
			}),
			assignments: 100_000,
			baseline:    90000,
			tolerance:   284, // 284 is 3 standard deviations for 100k trials with 90% probability.
		},
		{
			description: "Evaluate between 40% to 60% to approx 20%",
			condition: createNamedCondition(isEnabled, oneOfCondition{
				Percent: &percentCondition{
					PercentOperator: between,
					MicroPercentRange: microPercentRange{
						MicroPercentLowerBound: 40_000_000,
						MicroPercentUpperBound: 60_000_000,
					},
				},
			}),
			assignments: 100_000,
			baseline:    20000,
			tolerance:   379, // 379 is 3 standard deviations for 100k trials with 20% probability.
		},
		{
			description: "Evaluate between interquartile range to approx 50%",
			condition: createNamedCondition(isEnabled, oneOfCondition{
				Percent: &percentCondition{
					PercentOperator: between,
					MicroPercentRange: microPercentRange{
						MicroPercentLowerBound: 25_000_000,
						MicroPercentUpperBound: 75_000_000,
					},
				},
			}),
			assignments: 100_000,
			baseline:    50000,
			tolerance:   474, // 474 is 3 standard deviations for 100k trials with 50% probability.
		},
	}
	for _, tc := range probabilisticEvalTestCases {
		t.Run(tc.description, func(t *testing.T) {
			truthyAssignments := evaluateRandomAssignments(tc.assignments, tc.condition)
			lessThan := truthyAssignments <= tc.baseline+tc.tolerance
			greaterThan := truthyAssignments >= tc.baseline-tc.tolerance
			outcome := lessThan && greaterThan
			if outcome != true {
				t.Errorf("Incorrect probabilistic evaluation: got %d true assignments, want between %d and %d (baseline %d, tolerance %d)",
					truthyAssignments, tc.baseline-tc.tolerance, tc.baseline+tc.tolerance, tc.baseline, tc.tolerance)
			}
		})
	}
}

func TestCustomSignalConditionIsValid(t *testing.T) {
	testCases := []struct {
		description string
		condition   customSignalCondition
		expected    error
	}{
		{
			description: "Valid condition",
			condition: customSignalCondition{
				CustomSignalOperator:     stringExactlyMatches,
				CustomSignalKey:          customSignalKey,
				TargetCustomSignalValues: []string{premium},
			},
			expected: nil,
		},
		{
			description: "Missing operator",
			condition: customSignalCondition{
				CustomSignalKey:          customSignalKey,
				TargetCustomSignalValues: []string{premium},
			},
			expected: errInvalidCustomSignal,
		},
		{
			description: "Missing key",
			condition: customSignalCondition{
				CustomSignalOperator:     stringExactlyMatches,
				TargetCustomSignalValues: []string{premium},
			},
			expected: errInvalidCustomSignal,
		},
		{
			description: "Missing target values",
			condition: customSignalCondition{
				CustomSignalOperator: stringExactlyMatches,
				CustomSignalKey:      customSignalKey,
			},
			expected: errInvalidCustomSignal,
		},
		{
			description: "Missing multiple fields (operator and key)",
			condition: customSignalCondition{
				TargetCustomSignalValues: []string{premium},
			},
			expected: errInvalidCustomSignal,
		},
		{
			description: "Missing all fields",
			condition:   customSignalCondition{},
			expected:    errInvalidCustomSignal,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			actual := tc.condition.isValid()
			if actual != tc.expected {
				t.Errorf("isValid() = %v, want %v for condition: %+v", actual, tc.expected, tc.condition)
			}
		})
	}
}

func TestEvaluateCustomSignalCondition_MissingKeyInContext(t *testing.T) {
	condition := createNamedCondition(isEnabled, oneOfCondition{
		CustomSignal: &customSignalCondition{
			CustomSignalOperator:     stringExactlyMatches,
			CustomSignalKey:          customSignalKey,
			TargetCustomSignalValues: []string{premium},
		},
	})
	// Context does NOT contain 'customSignalKey'
	context := map[string]any{
		"key": "value",
	}
	evaluateConditionsAndReportResult(t, condition, isEnabled, context, false)
}

func TestCustomSignals_StringContains(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: "testing", targets: "test,sting", outcome: true},
		{actual: "check for spaces", targets: "for ,test", outcome: true},
		{actual: "no word is present", targets: "not,absent,words", outcome: false},
		{actual: "case Sensitive", targets: "Case,sensitive", outcome: false},
		{actual: "match 'single quote'", targets: "'single quote',Match", outcome: true},
		{actual: false, targets: "true, false", outcome: false},
		{actual: false, targets: "true,false", outcome: true},
		{actual: "no quote present", targets: "'no quote',\"present\"", outcome: false},
		{actual: 123, targets: "23,string", outcome: true},
		{actual: 123.45, targets: "9862123451,23.4", outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCase(stringContains, t)(tc)
	}
}

func TestCustomSignals_StringDoesNotContain(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: "foobar", targets: "foo,biz", outcome: false},
		{actual: "foobar", targets: "biz,cat,car", outcome: true},
		{actual: 387.42, targets: "6.4,54", outcome: true},
		{actual: "single quote present", targets: "'single quote',Present ", outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCase(stringDoesNotContain, t)(tc)
	}
}

func TestCustomSignals_StringExactlyMatches(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: "foobar", targets: "foo,biz", outcome: false},
		{actual: "Foobar", targets: "   Foobar ,cat,car", outcome: true},
		{actual: "matches if there are leading and trailing whitespaces", targets: "   matches if there are leading and trailing whitespaces    ", outcome: true},
		{actual: "does not match internal whitespaces", targets: "   does    not match internal    whitespaces    ", outcome: false},
		{actual: 123.456, targets: "123.45,456", outcome: false},
		{actual: 987654321.1234567, targets: "  987654321.1234567  ,12", outcome: true},
		{actual: "single quote present", targets: "'single quote',Present ", outcome: false},
		{actual: true, targets: "true ", outcome: true},
		{actual: struct {
			index    int
			category string
		}{index: 1, category: "sample"}, targets: "{index: 1, category: \"sample\"}", outcome: false},
	}

	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(stringExactlyMatches, t)(tc)
		runCustomSignalTestCase(stringExactlyMatches, t)(tc)
	}
}

func TestCustomSignals_StringContainsRegex(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: "foobar", targets: "^foo,biz", outcome: true},                          // Matches start anchor ^foo
		{actual: " hello world ", targets: "     hello ,    world    ", outcome: false}, // Patterns are literal strings including spaces, neither matches exactly? (Outcome seems unexpected for contains)
		{actual: "endswithhello", targets: ".*hello$", outcome: true},                   // Matches end anchor hello$
		{actual: "foobar", targets: "^foo", outcome: true},                              // Starts with "foo"
		{actual: "barfoo", targets: "^foo", outcome: false},                             // Does not start with "foo"
		{actual: "foobar", targets: "bar$", outcome: true},                              // Ends with "bar"
		{actual: "barfoo", targets: "bar$", outcome: false},                             // Does not end with "bar"
		{actual: "hello world", targets: "hello.*world", outcome: true},                 // Contains "hello" and "world" with anything in between
		{actual: "hello world", targets: "hello\\s+world", outcome: true},               // Contains "hello" and "world" with one or more whitespace in between
		{actual: "helloworld", targets: "hello\\s+world", outcome: false},               // No whitespace between hello and world
		{actual: "123-456-7890", targets: "\\d{3}-\\d{3}-\\d{4}", outcome: true},        // Phone number format
		{actual: "invalid", targets: "([a-z]+", outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCase(stringContainsRegex, t)(tc)
	}
}

func TestCustomSignals_NumericLessThan(t *testing.T) {
	withWhiteSpaces := []customSignalTestCase{
		{actual: int16(2), targets: "4", outcome: true},
		{actual: " -2.0 ", targets: "  -2  ", outcome: false},
		{actual: uint8(25), targets: "25.6", outcome: true},
		{actual: float32(-25.5), targets: "-25.6", outcome: false},
		{actual: " -25.5", targets: " -25.1  ", outcome: true},
		{actual: " 3", targets: " 2,4  ", outcome: false},
		{actual: "0", targets: "0", outcome: false},
	}
	for _, tc := range withWhiteSpaces {
		runCustomSignalTestCaseWithWhiteSpaces(numericLessThan, t)(tc)
	}
	withoutWhiteSpaces := append(withWhiteSpaces, customSignalTestCase{actual: false, targets: "1", outcome: true})
	for _, tc := range withoutWhiteSpaces {
		runCustomSignalTestCase(numericLessThan, t)(tc)
	}
}

func TestCustomSignals_NumericLessEqual(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: int16(2), targets: "4", outcome: true},
		{actual: "-2", targets: "-2", outcome: true},
		{actual: float32(25.5), targets: "25.6", outcome: true},
		{actual: -25.5, targets: "-25.6", outcome: false},
		{actual: "-25.5", targets: "-25.1", outcome: true},
		{actual: "0", targets: "0", outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(numericLessThanEqual, t)(tc)
		runCustomSignalTestCase(numericLessThanEqual, t)(tc)
	}
}

func TestCustomSignals_NumericEqual(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: float32(2), targets: "4", outcome: false},
		{actual: "-2", targets: "-2", outcome: true},
		{actual: -25.5, targets: "-25.6", outcome: false},
		{actual: "-25.5", targets: "123a", outcome: false},
		{actual: uint16(0), targets: "0", outcome: true},
		{actual: struct {
			index int
		}{index: 2}, targets: "0", outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(numericEqual, t)(tc)
		runCustomSignalTestCase(numericEqual, t)(tc)
	}
}

func TestCustomSignals_NumericNotEqual(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: int16(-2), targets: "4", outcome: true},
		{actual: "-2", targets: "-2", outcome: false},
		{actual: float32(-25.5), targets: "-25.6", outcome: true},
		{actual: "123a", targets: "-25.5", outcome: false},
		{actual: "0", targets: "0", outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(numericNotEqual, t)(tc)
		runCustomSignalTestCase(numericNotEqual, t)(tc)
	}
}

func TestCustomSignals_NumericGreaterThan(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: float32(2), targets: "4", outcome: false},
		{actual: "-2", targets: "-2", outcome: false},
		{actual: 25.59, targets: "25.6", outcome: false},
		{actual: int32(-25), targets: "-25.6", outcome: true},
		{actual: "-25.5", targets: "-25.5", outcome: false},
		{actual: "0", targets: "0", outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(numericGreaterThan, t)(tc)
		runCustomSignalTestCase(numericGreaterThan, t)(tc)
	}
}

func TestCustomSignals_NumericGreaterEqual(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: uint32(2), targets: "4", outcome: false},
		{actual: "-2", targets: "-2", outcome: true},
		{actual: float32(25.5), targets: "25.6", outcome: false},
		{actual: -25.5, targets: "-25.6", outcome: true},
		{actual: "-25.5", targets: "-25.5", outcome: true},
		{actual: "0", targets: "0", outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(numericGreaterEqual, t)(tc)
		runCustomSignalTestCase(numericGreaterEqual, t)(tc)
	}
}

func Test_TransformVersionToSegments(t *testing.T) {
	versionToSegmentTestCases := []struct {
		description     string
		semanticVersion string
		outcome         struct {
			err      error
			segments []int
		}
	}{
		{
			semanticVersion: "1.2.3.4.5",
			description:     "Valid semantic version with maximum allowed segments",
			outcome: struct {
				err      error
				segments []int
			}{
				err:      nil,
				segments: []int{1, 2, 3, 4, 5},
			},
		},
		{
			semanticVersion: "1.2.3.4.5.6",
			description:     "Returns error when version exceeds maximum allowed segments",
			outcome: struct {
				err      error
				segments []int
			}{
				err:      errTooManySegments,
				segments: nil,
			},
		},
		{
			semanticVersion: "1.2.3.4.-5",
			description:     "Returns error when a segment is negative",
			outcome: struct {
				err      error
				segments []int
			}{
				err:      errNegativeSegment,
				segments: nil,
			},
		},
		{
			semanticVersion: ".1.2.",
			description:     "Handles leading/trailing separators and pads missing segments with zero",
			outcome: struct {
				err      error
				segments []int
			}{
				err:      nil,
				segments: []int{1, 2, 0, 0, 0},
			},
		},
		{
			semanticVersion: "abcd.123",
			description:     "Returns error for non-numeric segment value",
			outcome: struct {
				err      error
				segments []int
			}{
				err:      errors.New("failed to parse segment \"abcd\": strconv.Atoi: parsing \"abcd\": invalid syntax"),
				segments: nil,
			},
		},
	}

	for _, tc := range versionToSegmentTestCases {
		t.Run(tc.description, func(t *testing.T) {
			segments, err := transformVersionToSegments(tc.semanticVersion)
			if tc.outcome.err == nil {
				if err != nil {
					t.Fatalf("transformVersionToSegments(%q) error = %v, want nil", tc.semanticVersion, err)
				}
			} else {
				if err == nil || tc.outcome.err.Error() != err.Error() {
					t.Fatalf("transformVersionToSegments(%q) error = %v, want %v", tc.semanticVersion, err, tc.outcome.err)
				}
			}

			if !reflect.DeepEqual(tc.outcome.segments, segments) {
				t.Errorf("transformVersionToSegments(%q) segments = %v, want %v", tc.semanticVersion, segments, tc.outcome.segments)
			}
		})
	}
}

func TestCustomSignals_SemanticVersionLessThan(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: uint16(2), targets: "4", outcome: true},
		{actual: 2., targets: "4.0", outcome: true},
		{actual: .9, targets: "0.4", outcome: false},
		{actual: ".3", targets: "0.1", outcome: false},
		{actual: float32(2.3), targets: "2.3.2", outcome: true},
		{actual: "2.3.4.1", targets: "2.3.4", outcome: false},
		{actual: 2.3, targets: "2.3.0", outcome: false},
		{actual: int16(3), targets: "1.2,4", outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(semanticVersionLessThan, t)(tc)
		runCustomSignalTestCase(semanticVersionLessThan, t)(tc)
	}
}

func TestCustomSignals_SemanticVersionLessEqual(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2., targets: "2.0", outcome: true},
		{actual: .456, targets: "0.456.13", outcome: true},
		{actual: ".3", targets: "0.1,0.4", outcome: false},
		{actual: float32(2.3), targets: "2.3.0", outcome: true},
		{actual: "2.3.4.5.6", targets: "2.3.4.5.6", outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(semanticVersionLessEqual, t)(tc)
		runCustomSignalTestCase(semanticVersionLessEqual, t)(tc)
	}
}

func TestCustomSignals_SemanticVersionEqual(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2., targets: "2.0", outcome: true},
		{actual: 2.0, targets: "2", outcome: true},
		{actual: uint16(2), targets: "2", outcome: true},
		{actual: ".3", targets: "0.1, 0.4", outcome: false},
		{actual: "1.2.3.4.5.6", targets: "1.2.3", outcome: false},
		{actual: float32(2.3), targets: "2.3.0", outcome: true},
		{actual: "2.3.4.5.6", targets: "2.3.4.5.6", outcome: true},
		{actual: "1.3.4.5.6", targets: "2.3.4.5.6", outcome: false},
		{actual: "5.12.-3.4", targets: "5.12.3.4", outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(semanticVersionEqual, t)(tc)
		runCustomSignalTestCase(semanticVersionEqual, t)(tc)
	}
}

func TestCustomSignals_SemanticVersionNotEqual(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2.3, targets: "2.0", outcome: true},
		{actual: uint32(8), targets: "2", outcome: true},
		{actual: "1.2.3.4.5.6", targets: "1.2.3", outcome: false},
		{actual: "2.3.4.5.6", targets: "2.3.4.5.6", outcome: false},
		{actual: "5.12.-3.4", targets: "5.12.3.4", outcome: false},
		{actual: "1.2.3", targets: "1.2.a", outcome: false},
		{actual: struct{}{}, targets: "1", outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(semanticVersionNotEqual, t)(tc)
		runCustomSignalTestCase(semanticVersionNotEqual, t)(tc)
	}
}

func TestCustomSignals_SemanticVersionGreaterThan(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2., targets: "2.0", outcome: false},
		{actual: 2.0, targets: "2", outcome: false},
		{actual: ".3", targets: "0.1", outcome: true},
		{actual: "1.2.3.4.5.6", targets: "1.2.3", outcome: false},
		{actual: 12.4, targets: "12.3.0", outcome: true},
		{actual: "2.3.4.5.6", targets: "2.3.4.5.6", outcome: false},
		{actual: "5.12.3.4", targets: "5.11.8.9", outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(semanticVersionGreaterThan, t)(tc)
		runCustomSignalTestCase(semanticVersionGreaterThan, t)(tc)
	}
}

func TestCustomSignals_SemanticVersionGreaterEqual(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2., targets: "2.0", outcome: true},
		{actual: int16(2), targets: "2", outcome: true},
		{actual: ".3", targets: "0.1", outcome: true},
		{actual: "1.2.3.4.5.6", targets: "1.2.3", outcome: false},
		{actual: float32(12.4), targets: "12.3.0", outcome: true},
		{actual: "2.3.4.5.6", targets: "2.3.4.5.6", outcome: true},
		{actual: "5.12.3.4", targets: "5.11.8.9", outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces(semanticVersionGreaterEqual, t)(tc)
		runCustomSignalTestCase(semanticVersionGreaterEqual, t)(tc)
	}
}
