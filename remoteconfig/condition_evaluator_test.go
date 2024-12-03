package remoteconfig

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
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

func runCustomSignalTestCase(operator string, t *testing.T) func(customSignalTestCase) {
	return func(tc customSignalTestCase) {
		description := fmt.Sprintf("Evaluates operator %v with targets %v and actual %v to outcome %v", operator, tc.targets, tc.actual, tc.outcome)
		t.Run(description, func(t *testing.T) {
			condition := createNamedCondition(IsEnabled, OneOfCondition{
				CustomSignal: &CustomSignalCondition{
					CustomSignalOperator:     operator,
					CustomSignalKey:          UserProperty,
					TargetCustomSignalValues: tc.targets,
				},
			})
			ce := ConditionEvaluator{
				conditions:        []NamedCondition{condition},
				evaluationContext: map[string]any{UserProperty: tc.actual},
			}

			_, ec := ce.evaluateConditions()
			value, ok := ec[IsEnabled]

			if !ok {
				t.Fatalf("condition 'is_enabled' isn't evaluated")
			}

			if value != tc.outcome {
				t.Fatalf("condition evaluation is incorrect")
			}
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
		{actual: true, targets: []string{"true"}, outcome: false},
		{actual: SampleObject{index: 1, category: "sample"}, targets: []string{"{index: 1, category: \"sample\"}"}, outcome: false},
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
		{actual: 25.5, targets: []string{"25.6"}, outcome: false},
		{actual: -25.5, targets: []string{"-25.6"}, outcome: false},
		{actual: "-25.5", targets: []string{"-25.5"}, outcome: true},
		{actual: "0", targets: []string{"0"}, outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("NUMERIC_EQUAL", t)(tc)
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
		{actual: ".3", targets: []string{"0.1"}, outcome: false},
		{actual: "1.2.3.4.5.6", targets: []string{"1.2.3"}, outcome: false},
		{actual: 2.3, targets: []string{"2.3.0"}, outcome: true},
		{actual: "2.3.4.5.6", targets: []string{"2.3.4.5.6"}, outcome: true},
	}
	for _, tc := range testCases {
		runCustomSignalTestCaseWithWhiteSpaces("SEMANTIC_VERSION_EQUAL", t)(tc)
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