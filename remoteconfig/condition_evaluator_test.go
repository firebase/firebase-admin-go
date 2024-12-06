package remoteconfig

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const (
	IsEnabled                     = "is_enabled"
	UserProperty                  = "user_prop"
	LeadingWhiteSpaceCountActual  = 3
	TrailingWhiteSpaceCountActual = 5
	LeadingWhiteSpaceCountTarget  = 4
	TrailingWhiteSpaceCountTarget = 2
)

type customSignalTestCase struct {
	targets []string
	actual  any
	outcome bool
}

type SampleObject struct {
	index    int
	category string
}

func createNamedCondition(name string, condition OneOfCondition) NamedCondition {
	nc := NamedCondition{
		Name: name,
		Condition: OneOfCondition{
			OrCondition: &OrCondition{
				Conditions: []OneOfCondition{
					{
						AndCondition: &AndCondition{
							Conditions: []OneOfCondition{condition},
						},
					},
				},
			},
		},
	}
	return nc
}

func evaluateConditionsAndReportResult(t *testing.T, nc NamedCondition, context map[string]any, outcome bool) {
	ce := ConditionEvaluator{
		conditions:        []NamedCondition{nc},
		evaluationContext: context,
	}
	ec := ce.evaluateConditions()
	value, ok := ec[IsEnabled]
	if !ok {
		t.Fatalf("condition 'is_enabled' isn't evaluated")
	}
	if value != outcome {
		t.Fatalf("condition evaluation is incorrect")
	}
}

// Returns the number of assignments which evaluate to true for the specified percent condition.
// This method randomly generates the ids for each assignment for this purpose.
func evaluateRandomAssignments(numOfAssignments int, condition NamedCondition) int {
	evalTrueCount := 0
	for i := 0; i < numOfAssignments; i++ {
		context := map[string]any{"randomizationId": uuid.New()}
		ce := ConditionEvaluator{
			conditions:        []NamedCondition{condition},
			evaluationContext: context,
		}
		ec := ce.evaluateConditions()
		if value, ok := ec[IsEnabled]; ok && value {
			evalTrueCount += 1
		}
	}
	return evalTrueCount
}

func runCustomSignalTestCase(operator string, t *testing.T) func(customSignalTestCase) {
	return func(tc customSignalTestCase) {
		description := fmt.Sprintf("Evaluates custom signal operator %v with targets %v and actual %v to outcome %v", operator, tc.targets, tc.actual, tc.outcome)
		t.Run(description, func(t *testing.T) {
			condition := createNamedCondition(IsEnabled, OneOfCondition{
				CustomSignal: &CustomSignalCondition{
					CustomSignalOperator:     operator,
					CustomSignalKey:          UserProperty,
					TargetCustomSignalValues: tc.targets,
				},
			})
			evaluateConditionsAndReportResult(t, condition, map[string]any{UserProperty: tc.actual}, tc.outcome)
		})
	}
}

func addLeadingAndTrailingWhiteSpaces(v any, leadingSpacesCount int, trailingSpacesCount int) string {
	var strVal string
	switch value := v.(type) {
	case string:
		strVal = value
	case int:
		strVal = strconv.Itoa(value)
	case float64:
		strVal = strconv.FormatFloat(value, DecimalFormat, MinBitsPossible, DoublePrecisionWidth)
	}
	return strings.Repeat(WhiteSpace, leadingSpacesCount) + strVal + strings.Repeat(WhiteSpace, trailingSpacesCount)
}

func runCustomSignalTestCaseWithWhiteSpaces(operator string, t *testing.T) func(customSignalTestCase) {
	return func(tc customSignalTestCase) {
		runCustomSignalTestCase(operator, t)(tc)
		targetsWithWhiteSpaces := []string{}
		for _, target := range tc.targets {
			targetsWithWhiteSpaces = append(targetsWithWhiteSpaces, addLeadingAndTrailingWhiteSpaces(target, LeadingWhiteSpaceCountTarget, TrailingWhiteSpaceCountTarget))
		}
		runCustomSignalTestCase(operator, t)(customSignalTestCase{
			outcome: tc.outcome,
			actual:  addLeadingAndTrailingWhiteSpaces(tc.actual, LeadingWhiteSpaceCountActual, TrailingWhiteSpaceCountActual),
			targets: targetsWithWhiteSpaces,
		})
	}
}

func compareSlices(slice1, slice2 []int) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	for idx := range slice1 {
		if slice1[idx] != slice2[idx] {
			return false
		}
	}
	return true
}

func TestPercentCondition_KnownValues(t *testing.T) {
	percentTestCases := []struct {
		seed            string
		randomizationId string
		outcome         bool
	}{
		{seed: "1", randomizationId: "one", outcome: false},
		{seed: "2", randomizationId: "two", outcome: false},
		{seed: "3", randomizationId: "three", outcome: true},
		{seed: "4", randomizationId: "four", outcome: false},
		{seed: "5", randomizationId: "five", outcome: true},
		{seed: "", randomizationId: "😊", outcome: true},
		{seed: "", randomizationId: "😀", outcome: false},
		{seed: "hêl£o", randomizationId: "wørlÐ", outcome: false},
		{seed: "řemøťe", randomizationId: "çōnfįġ", outcome: true},
		{seed: "long", randomizationId: strings.Repeat(".", 100), outcome: true},
		{seed: "very-long", randomizationId: strings.Repeat(".", 1000), outcome: false},
	}

	for _, tc := range percentTestCases {
		description := fmt.Sprintf("Percent condition evaluation for seed %s & randomization_id %s should produce the outcome %v", tc.seed, tc.randomizationId, tc.outcome)
		t.Run(description, func(t *testing.T) {
			percentCondition := createNamedCondition(IsEnabled, OneOfCondition{
				Percent: &PercentCondition{
					Seed:            tc.seed,
					PercentOperator: "BETWEEN",
					MicroPercentRange: MicroPercentRange{
						MicroPercentUpperBound: 50_000_000,
					},
				},
			})
			evaluateConditionsAndReportResult(t, percentCondition, map[string]any{"randomizationId": tc.randomizationId}, tc.outcome)
		})
	}
}

func TestPercentCondition_UnknownOperatorEvaluatesToFalse(t *testing.T) {
	percentCondition := createNamedCondition(IsEnabled, OneOfCondition{
		Percent: &PercentCondition{
			PercentOperator: "UNKNOWN",
		},
	})
	evaluateConditionsAndReportResult(t, percentCondition, map[string]any{"randomizationId": "1234"}, false)
}

func TestPercentCondition_MicroPercent(t *testing.T) {
	microPercentTestCases := []struct {
		description  string
		operator     string
		microPercent uint32
		outcome      bool
	}{
		{
			description:  "Evaluate less or equal to true when microPercent is max",
			operator:     "LESS_OR_EQUAL",
			microPercent: 100_000_000,
			outcome:      true,
		},
		{
			description:  "Evaluate less or equal to false when microPercent is min",
			operator:     "LESS_OR_EQUAL",
			microPercent: 0,
			outcome:      false,
		},
		{
			description: "Evaluate less or equal to false when microPercent is not set (microPercent should use zero)",
			operator:    "LESS_OR_EQUAL",
			outcome:     false,
		},
		{
			description: "Evaluate greater than to true when microPercent is not set (microPercent should use zero)",
			operator:    "GREATER_THAN",
			outcome:     true,
		},
		{
			description:  "Evaluate greater than max to false",
			operator:     "GREATER_THAN",
			outcome:      false,
			microPercent: 100_000_000,
		},
		{
			description:  "Evaluate less than equal to 9571542 to true",
			operator:     "LESS_OR_EQUAL",
			microPercent: 9_571_542, // instanceMicroPercentile of abcdef.123 is 9_571_542
			outcome:      true,
		},
		{
			description:  "Evaluate greater than 9571542 to true",
			operator:     "GREATER_THAN",
			microPercent: 9_571_541, // instanceMicroPercentile of abcdef.123 is 9_571_542
			outcome:      true,
		},
	}
	for _, tc := range microPercentTestCases {
		t.Run(tc.description, func(t *testing.T) {
			percentCondition := createNamedCondition(IsEnabled, OneOfCondition{
				Percent: &PercentCondition{
					PercentOperator: tc.operator,
					MicroPercent:    tc.microPercent,
					Seed:            "abcdef",
				},
			})
			evaluateConditionsAndReportResult(t, percentCondition, map[string]any{"randomizationId": "123"}, tc.outcome)
		})
	}
}

func TestPercentCondition_MicroPercentRange(t *testing.T) {
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
			description:    "Evaluate to true when between lower and upper bound", // instanceMicroPercentile of abcdef.123 is 9_571_542
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
			description:    "Evaluate to false when not between 9_400_000 and 9_500_000", // instanceMicroPercentile of abcdef.123 is 9_571_542
			microPercentLb: 9_400_000,
			microPercentUb: 9_500_000,
			operator:       "BETWEEN",
			outcome:        false,
		},
	}
	for _, tc := range microPercentTestCases {
		t.Run(tc.description, func(t *testing.T) {
			percentCondition := createNamedCondition(IsEnabled, OneOfCondition{
				Percent: &PercentCondition{
					PercentOperator: tc.operator,
					MicroPercentRange: MicroPercentRange{
						MicroPercentLowerBound: tc.microPercentLb,
						MicroPercentUpperBound: tc.microPercentUb,
					},
					Seed: "abcdef",
				},
			})
			evaluateConditionsAndReportResult(t, percentCondition, map[string]any{"randomizationId": "123"}, tc.outcome)
		})
	}
}

func TestPercentCondition_ProbabilisticEvaluation(t *testing.T) {
	probabilisticEvalTestCases := []struct {
		description string
		condition   NamedCondition
		assignments int
		baseline    int
		tolerance   int
	}{
		{
			description: "Evaluate less or equal to 10% to approx 10%",
			condition: createNamedCondition(IsEnabled, OneOfCondition{
				Percent: &PercentCondition{
					PercentOperator: "LESS_OR_EQUAL",
					MicroPercent:    10_000_000,
				},
			}),
			assignments: 100_000,
			baseline:    10000,
			tolerance:   284, // 284 is 3 standard deviations for 100k trials with 10% probability.
		},
		{
			description: "Evaluate between 0 to 10% to approx 10%",
			condition: createNamedCondition(IsEnabled, OneOfCondition{
				Percent: &PercentCondition{
					PercentOperator: "BETWEEN",
					MicroPercentRange: MicroPercentRange{
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
			condition: createNamedCondition(IsEnabled, OneOfCondition{
				Percent: &PercentCondition{
					PercentOperator: "GREATER_THAN",
					MicroPercent:    10_000_000,
				},
			}),
			assignments: 100_000,
			baseline:    90000,
			tolerance:   284, // 284 is 3 standard deviations for 100k trials with 90% probability.
		},
		{
			description: "Evaluate between 40% to 60% to approx 20%",
			condition: createNamedCondition(IsEnabled, OneOfCondition{
				Percent: &PercentCondition{
					PercentOperator: "BETWEEN",
					MicroPercentRange: MicroPercentRange{
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
			condition: createNamedCondition(IsEnabled, OneOfCondition{
				Percent: &PercentCondition{
					PercentOperator: "BETWEEN",
					MicroPercentRange: MicroPercentRange{
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
				t.Fatalf("Incorrect probablistic evaluation")
			}
		})
	}
}

func TestCustomSignal_InvalidCustomSignalConditionEvaluatesToFalse(t *testing.T) {
	invalidCustomSignalCondition := []struct {
		operator    string
		key         string
		target      []string
		description string
	}{
		{
			description: "Custom signal operator not set",
			key:         "csOne",
			target:      []string{"123"},
		},
		{
			description: "Unknown custom signal operator",
			operator:    "UNKNOWN",
			target:      []string{"123"},
		},
		{
			description: "Custom signal key is not passed",
			operator:    "STRING_CONTAINS",
			target:      []string{"123"},
		},
		{
			description: "Target values not set",
			operator:    "STRING_CONTAINS",
			key:         "csOne",
		},
	}
	for _, tc := range invalidCustomSignalCondition {
		t.Run(tc.description, func(t *testing.T) {
			condition := createNamedCondition(IsEnabled, OneOfCondition{
				CustomSignal: &CustomSignalCondition{
					CustomSignalOperator:     tc.operator,
					CustomSignalKey:          tc.key,
					TargetCustomSignalValues: tc.target,
				},
			})
			evaluateConditionsAndReportResult(t, condition, map[string]any{}, false)
		})
	}
}

func TestCustomSignal_EmptyContextEvaluatesToFalse(t *testing.T) {
	condition := createNamedCondition(IsEnabled, OneOfCondition{
		CustomSignal: &CustomSignalCondition{
			CustomSignalOperator:     "STRING_EXACTLY_MATCHES",
			CustomSignalKey:          "csOne",
			TargetCustomSignalValues: []string{"hello"},
		},
	})
	evaluateConditionsAndReportResult(t, condition, map[string]any{}, false)
}

func TestEvaluateConditions_EmptyOrConditionToFalse(t *testing.T) {
	condition := createNamedCondition(IsEnabled, OneOfCondition{
		OrCondition: &OrCondition{},
	})
	evaluateConditionsAndReportResult(t, condition, map[string]any{}, false)
}

func TestEvaluateConditions_EmptyOrAndConditionToTrue(t *testing.T) {
	condition := createNamedCondition(IsEnabled, OneOfCondition{
		OrCondition: &OrCondition{
			Conditions: []OneOfCondition{
				{
					AndCondition: &AndCondition{},
				},
			},
		},
	})
	evaluateConditionsAndReportResult(t, condition, map[string]any{}, true)
}

func TestEvaluateConditions_MultipleCustomSignals(t *testing.T) {
	csOneKey := "version"
	csTwoKey := "tier"
	condition := createNamedCondition(IsEnabled, OneOfCondition{
		OrCondition: &OrCondition{
			Conditions: []OneOfCondition{
				{
					AndCondition: &AndCondition{
						Conditions: []OneOfCondition{
							{
								CustomSignal: &CustomSignalCondition{
									CustomSignalOperator:     "SEMANTIC_VERSION_GREATER_EQUAL",
									CustomSignalKey:          csOneKey,
									TargetCustomSignalValues: []string{"1.2.3"},
								},
							},
							{
								CustomSignal: &CustomSignalCondition{
									CustomSignalOperator:     "STRING_EXACTLY_MATCHES",
									CustomSignalKey:          csTwoKey,
									TargetCustomSignalValues: []string{"paid"},
								},
							},
						},
					},
				},
			},
		},
	})

	customSignalTestCases := []struct {
		description string
		csOneVal    any
		csTwoVal    any
		outcome     bool
	}{
		{
			description: "Values for both the keys satisfy the condition",
			csOneVal:    1.4,
			csTwoVal:    "paid",
			outcome:     true,
		},
		{
			description: "Only one of the custom signal values satisfy the condition",
			csOneVal:    "1.2.3",
			csTwoVal:    "free",
			outcome:     false,
		},
		{
			description: "None of the custom signal values satisfy the conditions",
			csOneVal:    .5,
			csTwoVal:    "mid-tier",
			outcome:     false,
		},
	}
	for _, tc := range customSignalTestCases {
		t.Run(tc.description, func(t *testing.T) {
			evaluateConditionsAndReportResult(t, condition, map[string]any{"version": tc.csOneVal, "tier": tc.csTwoVal}, tc.outcome)
		})
	}
}

func TestCustomSignals_StringContains(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: "presenting", targets: []string{"present", "absent"}, outcome: true},
		{actual: "check for spaces", targets: []string{"for ", "test"}, outcome: true},
		{actual: "no word is present", targets: []string{"not", "absent", "words"}, outcome: false},
		{actual: "case Sensitive", targets: []string{"Case", "sensitive"}, outcome: false},
		{actual: "match 'single quote'", targets: []string{"'single quote'", "Match"}, outcome: true},
		{actual: "match \"double quotes\"", targets: []string{"\"double quotes\""}, outcome: true},
		{actual: "'single quote' present", targets: []string{"single quote", "Match"}, outcome: true},
		{actual: "no quote present", targets: []string{"'no quote'", "\"present\""}, outcome: false},
		{actual: 123, targets: []string{"23", "string"}, outcome: true},
		{actual: 134.627, targets: []string{"34.62", "98621"}, outcome: true},
		{actual: 462.233, targets: []string{"2.a", "56"}, outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCase("STRING_CONTAINS", t)(tc)
	}
}

func TestCustomSignals_StringDoesNotContain(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: "foobar", targets: []string{"foo", "biz"}, outcome: false},
		{actual: "foobar", targets: []string{"biz", "cat", "car"}, outcome: true},
		{actual: 387.42, targets: []string{"6.4", "54"}, outcome: true},
		{actual: "single quote present", targets: []string{"'single quote'", "Present "}, outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCase("STRING_DOES_NOT_CONTAIN", t)(tc)
	}
}

func TestCustomSignals_StringExactlyMatches(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: "foobar", targets: []string{"foo", "biz"}, outcome: false},
		{actual: "Foobar", targets: []string{"Foobar", "cat", "car"}, outcome: true},
		{actual: "matches if there are leading and trailing whitespaces", targets: []string{"   matches if there are leading and trailing whitespaces    "}, outcome: true},
		{actual: "does not match internal whitespaces", targets: []string{"   does    not match internal    whitespaces    "}, outcome: false},
		{actual: 123.456, targets: []string{"123.45", "456"}, outcome: false},
		{actual: 987654321.1234567, targets: []string{"987654321.1234567", "12"}, outcome: true},
		{actual: "single quote present", targets: []string{"'single quote'", "Present "}, outcome: false},
		{actual: true, targets: []string{"true"}, outcome: false},
		{actual: SampleObject{index: 1, category: "sample"}, targets: []string{"{index: 1, category: \"sample\"}"}, outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("STRING_EXACTLY_MATCHES", t)(tc)
	}
}

func TestCustomSignals_StringContainsRegex(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: "foobar", targets: []string{"^foo", "biz"}, outcome: true},
		{actual: " hello world ", targets: []string{"     hello ", "    world    "}, outcome: false},
		{actual: "endswithhello", targets: []string{".*hello$"}, outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCase("STRING_CONTAINS_REGEX", t)(tc)
	}
}

func TestCustomSignals_NumericLessThan(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: 2, targets: []string{"4"}, outcome: true},
		{actual: "-2", targets: []string{"-2"}, outcome: false},
		{actual: 25.5, targets: []string{"25.6"}, outcome: true},
		{actual: -25.5, targets: []string{"-25.6"}, outcome: false},
		{actual: "-25.5", targets: []string{"-25.1"}, outcome: true},
		{actual: "0", targets: []string{"0"}, outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("NUMERIC_LESS_THAN", t)(tc)
	}
}

func TestCustomSignals_NumericLessEqual(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: 2, targets: []string{"4"}, outcome: true},
		{actual: "-2", targets: []string{"-2"}, outcome: true},
		{actual: 25.5, targets: []string{"25.6"}, outcome: true},
		{actual: -25.5, targets: []string{"-25.6"}, outcome: false},
		{actual: "-25.5", targets: []string{"-25.1"}, outcome: true},
		{actual: "0", targets: []string{"0"}, outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("NUMERIC_LESS_EQUAL", t)(tc)
	}
}

func TestCustomSignals_NumericEqual(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: 2, targets: []string{"4"}, outcome: false},
		{actual: "-2", targets: []string{"-2"}, outcome: true},
		{actual: -25.5, targets: []string{"-25.6"}, outcome: false},
		{actual: "-25.5", targets: []string{"123a"}, outcome: false},
		{actual: 0, targets: []string{"0"}, outcome: true},
		{actual: SampleObject{index: 2}, targets: []string{"0"}, outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("NUMERIC_EQUAL", t)(tc)
	}
}

func TestCustomSignals_NumericNotEqual(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: 2, targets: []string{"4"}, outcome: true},
		{actual: "-2", targets: []string{"-2"}, outcome: false},
		{actual: -25.5, targets: []string{"-25.6"}, outcome: true},
		{actual: "123a", targets: []string{"-25.5"}, outcome: false},
		{actual: "0", targets: []string{"0"}, outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("NUMERIC_NOT_EQUAL", t)(tc)
	}
}

func TestCustomSignals_NumericGreaterThan(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: 2, targets: []string{"4"}, outcome: false},
		{actual: "-2", targets: []string{"-2"}, outcome: false},
		{actual: 25.5, targets: []string{"25.6"}, outcome: false},
		{actual: -25.5, targets: []string{"-25.6"}, outcome: true},
		{actual: "-25.5", targets: []string{"-25.5"}, outcome: false},
		{actual: "0", targets: []string{"0"}, outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("NUMERIC_GREATER_THAN", t)(tc)
	}
}

func TestCustomSignals_NumericGreaterEqual(t *testing.T) {
	testCases := []customSignalTestCase{
		{actual: 2, targets: []string{"4"}, outcome: false},
		{actual: "-2", targets: []string{"-2"}, outcome: true},
		{actual: 25.5, targets: []string{"25.6"}, outcome: false},
		{actual: -25.5, targets: []string{"-25.6"}, outcome: true},
		{actual: "-25.5", targets: []string{"-25.5"}, outcome: true},
		{actual: "0", targets: []string{"0"}, outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("NUMERIC_GREATER_EQUAL", t)(tc)
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
			description:     "Valid semantic version is provided",
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
			description:     "Semantic version exceeds maximum segment length",
			outcome: struct {
				err      error
				segments []int
			}{
				err:      errors.New(ErrTooManySegments),
				segments: []int{},
			},
		},
		{
			semanticVersion: "1.2.3.4.-5",
			description:     "Semantic version exceeds maximum segment length",
			outcome: struct {
				err      error
				segments []int
			}{
				err:      errors.New(ErrNegativeSegment),
				segments: []int{},
			},
		},
	}

	for _, tc := range versionToSegmentTestCases {
		t.Run(tc.description, func(t *testing.T) {
			segments, err := transformVersionToSegments(tc.semanticVersion)

			if tc.outcome.err == nil {
				if err != nil {
					t.Fatalf("Expected error to be nil")
				}
			} else {
				if tc.outcome.err.Error() != err.Error() {
					t.Fatalf("Incorrect error returned")
				}
			}

			if !compareSlices(tc.outcome.segments, segments) {
				t.Fatalf("Generated segments are incorrect")
			}
		})
	}
}

func TestCustomSignals_SemanticVersionLessThan(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2, targets: []string{"4"}, outcome: true},
		{actual: 2., targets: []string{"4.0"}, outcome: true},
		{actual: .9, targets: []string{"0.4"}, outcome: false},
		{actual: ".3", targets: []string{"0.1"}, outcome: false},
		{actual: 2.3, targets: []string{"2.3.2"}, outcome: true},
		{actual: "2.3.4.1", targets: []string{"2.3.4"}, outcome: false},
		{actual: 2.3, targets: []string{"2.3.0"}, outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("SEMANTIC_VERSION_LESS_THAN", t)(tc)
	}
}

func TestCustomSignals_SemanticVersionLessEqual(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2., targets: []string{"2.0"}, outcome: true},
		{actual: .456, targets: []string{"0.456.13"}, outcome: true},
		{actual: ".3", targets: []string{"0.1"}, outcome: false},
		{actual: 2.3, targets: []string{"2.3.0"}, outcome: true},
		{actual: "2.3.4.5.6", targets: []string{"2.3.4.5.6"}, outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("SEMANTIC_VERSION_LESS_EQUAL", t)(tc)
	}
}

func TestCustomSignals_SemanticVersionEqual(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2., targets: []string{"2.0"}, outcome: true},
		{actual: 2.0, targets: []string{"2"}, outcome: true},
		{actual: 2, targets: []string{"2"}, outcome: true},
		{actual: ".3", targets: []string{"0.1"}, outcome: false},
		{actual: "1.2.3.4.5.6", targets: []string{"1.2.3"}, outcome: false},
		{actual: 2.3, targets: []string{"2.3.0"}, outcome: true},
		{actual: "2.3.4.5.6", targets: []string{"2.3.4.5.6"}, outcome: true},
		{actual: "5.12.-3.4", targets: []string{"5.12.3.4"}, outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("SEMANTIC_VERSION_EQUAL", t)(tc)
	}
}

func TestCustomSignals_SemanticVersionNotEqual(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2.3, targets: []string{"2.0"}, outcome: true},
		{actual: 8, targets: []string{"2"}, outcome: true},
		{actual: "1.2.3.4.5.6", targets: []string{"1.2.3"}, outcome: false},
		{actual: "2.3.4.5.6", targets: []string{"2.3.4.5.6"}, outcome: false},
		{actual: "5.12.-3.4", targets: []string{"5.12.3.4"}, outcome: false},
		{actual: "1.2.3", targets: []string{"1.2.a"}, outcome: false},
		{actual: SampleObject{}, targets: []string{"1"}, outcome: false},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("SEMANTIC_VERSION_NOT_EQUAL", t)(tc)
	}
}

func TestCustomSignals_SemanticVersionGreaterThan(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2., targets: []string{"2.0"}, outcome: false},
		{actual: 2.0, targets: []string{"2"}, outcome: false},
		{actual: ".3", targets: []string{"0.1"}, outcome: true},
		{actual: "1.2.3.4.5.6", targets: []string{"1.2.3"}, outcome: false},
		{actual: 12.4, targets: []string{"12.3.0"}, outcome: true},
		{actual: "2.3.4.5.6", targets: []string{"2.3.4.5.6"}, outcome: false},
		{actual: "5.12.3.4", targets: []string{"5.11.8.9"}, outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("SEMANTIC_VERSION_GREATER_THAN", t)(tc)
	}
}

func TestCustomSignals_SemanticVersionGreaterEqual(t *testing.T) {
	// a semantic version with leading or trailing segment separators cannot be entered on the console
	testCases := []customSignalTestCase{
		{actual: 2., targets: []string{"2.0"}, outcome: true},
		{actual: 2.0, targets: []string{"2"}, outcome: true},
		{actual: ".3", targets: []string{"0.1"}, outcome: true},
		{actual: "1.2.3.4.5.6", targets: []string{"1.2.3"}, outcome: false},
		{actual: 12.4, targets: []string{"12.3.0"}, outcome: true},
		{actual: "2.3.4.5.6", targets: []string{"2.3.4.5.6"}, outcome: true},
		{actual: "5.12.3.4", targets: []string{"5.11.8.9"}, outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("SEMANTIC_VERSION_GREATER_EQUAL", t)(tc)
	}
}
