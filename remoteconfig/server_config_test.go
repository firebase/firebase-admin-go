package remoteconfig

import "testing"

type configGetterTestCase struct {
	name           string
	key            string
	expectedString string
	expectedInt    int
	expectedBool   bool
	expectedFloat  float64
	expectedSource ValueSource
}

func getTestConfig() ServerConfig {
	config := ServerConfig{
		configValues: map[string]value{
			paramOne: {
				value:  valueOne,
				source: Default,
			},
			paramTwo: {
				value:  valueTwo,
				source: Remote,
			},
			paramThree: {
				value:  valueThree,
				source: Default,
			},
			paramFour: {
				value:  valueFour,
				source: Remote,
			},
		},
	}
	return config
}

func TestServerConfigGetters(t *testing.T) {
	config := getTestConfig()
	testCases := []configGetterTestCase{
		{
			name:           "Parameter Value : String, Default Source",
			key:            paramOne,
			expectedString: valueOne,
			expectedInt:    0,
			expectedBool:   false,
			expectedFloat:  0,
			expectedSource: Default,
		},
		{
			name:           "Parameter Value : JSON, Remote Source",
			key:            paramTwo,
			expectedString: valueTwo,
			expectedInt:    0,
			expectedBool:   false,
			expectedFloat:  0,
			expectedSource: Remote,
		},
		{
			name:           "Unknown Parameter Value",
			key:            "unknown_param",
			expectedString: "",
			expectedInt:    0,
			expectedBool:   false,
			expectedFloat:  0,
			expectedSource: Static,
		},
		{
			name:           "Parameter Value - Float, Default Source",
			key:            paramThree,
			expectedString: "123456789.123",
			expectedInt:    0,
			expectedBool:   false,
			expectedFloat:  123456789.123,
			expectedSource: Default,
		},
		{
			name:           "Parameter Value - Boolean, Remote Source",
			key:            paramFour,
			expectedString: "1",
			expectedInt:    1,
			expectedBool:   true,
			expectedFloat:  1,
			expectedSource: Remote,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := config.GetString(tc.key); got != tc.expectedString {
				t.Errorf("GetString(%q): got %q, want %q", tc.key, got, tc.expectedString)
			}

			if got := config.GetInt(tc.key); got != tc.expectedInt {
				t.Errorf("GetInt(%q): got %d, want %d", tc.key, got, tc.expectedInt)
			}

			if got := config.GetBoolean(tc.key); got != tc.expectedBool {
				t.Errorf("GetBoolean(%q): got %t, want %t", tc.key, got, tc.expectedBool)
			}

			if got := config.GetFloat(tc.key); got != tc.expectedFloat {
				t.Errorf("GetFloat(%q): got %f, want %f", tc.key, got, tc.expectedFloat)
			}

			if got := config.GetValueSource(tc.key); got != tc.expectedSource {
				t.Errorf("GetValueSource(%q): got %v, want %v", tc.key, got, tc.expectedSource)
			}
		})
	}
}
