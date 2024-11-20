package remoteconfig

import (
  "strconv"
  "strings"
)

// Value defines the interface for configuration values.
type Value struct {
	source string
	value  string
}

const (
	// Static represents a statically defined value.
	Static string = "static"

	// Default represents a default value.
	Default string = "default"

	// Remote represents a value fetched from a remote source.
	Remote string = "remote"

	defaultValueForBoolean = false
	defaultValueForString  = ""
	defaultValueForNumber = 0
)

var booleanTruthyValues = []string{"1", "true", "t", "yes", "y", "on"}

// ServerConfig is the implementation of the ServerConfig interface.
type ServerConfig struct {
	ConfigValues map[string]Value
}

// NewServerConfig creates a new ServerConfig instance.
func NewServerConfig(configValues map[string]Value) *ServerConfig {
	return &ServerConfig{ConfigValues: configValues}
}

// GetBoolean returns the boolean value associated with the given key.
func (s *ServerConfig) GetBoolean(key string) bool {
	return s.GetValue(key).asBoolean()
}

// GetNumber returns the integer value associated with the given key.
func (s *ServerConfig) GetNumber(key string) int {
	return s.GetValue(key).asNumber()
}

// GetString returns the string value associated with the given key.
func (s *ServerConfig) GetString(key string) string {
	return s.GetValue(key).asString()
}

// GetValue returns the Value associated with the given key.
func (s *ServerConfig) GetValue(key string) *Value {
	if val, ok := s.ConfigValues[key]; ok {
		return &val
	}
	return NewValue(Static, "")
}

// NewValue creates a new Value instance.
func NewValue(source string, value string) *Value {
	if value == "" {
		value = defaultValueForString
	}
	return &Value{source: source, value: value}
}

// asString returns the value as a string.
func (v *Value) asString() string {
	return v.value
}

// asBoolean returns the value as a boolean.
func (v *Value) asBoolean() bool {
	if v.source == Static {
		return defaultValueForBoolean
	}

	for _, truthyValue := range booleanTruthyValues {
		if strings.ToLower(v.value) == truthyValue {
			return true
		}
	}

	return false
}

// asNumber returns the value as an integer.
func (v *Value) asNumber() int {
	if v.source == Static {
		return defaultValueForNumber
	}
	num, err := strconv.Atoi(v.value)
	
	if err != nil {
		return defaultValueForNumber
	}

	return num
}

// GetSource returns the source of the value.
func (v *Value) GetSource() string {
	return v.source
}