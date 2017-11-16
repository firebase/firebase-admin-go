package utils

// StringP returns a pointer to the string value passed in.
func StringP(v string) *string {
	return &v
}

// BoolP returns a pointer to the bool value passed in.
func BoolP(v bool) *bool {
	return &v
}

// IntP returns a pointer to the int value passed in.
func IntP(v int) *int {
	return &v
}

// Int32P returns a pointer to the int32 value passed in.
func Int32P(v int32) *int32 {
	return &v
}

// Int64P returns a pointer to the int64 value passed in.
func Int64P(v int64) *int64 {
	return &v
}

// Float64P returns a pointer to the float64 value passed in.
func Float64P(v float64) *float64 {
	return &v
}
