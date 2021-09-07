package remoteconfig

import (
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/api/iterator"
)

// TagColor represents a tag color
type TagColor int

// Tag colors
const (
	colorUnspecified TagColor = iota
	Blue
	Brown
	Cyan
	DeepOrange
	Green
	Indigo
	Lime
	Orange
	Pink
	Purple
	Teal
)

// Version a Remote Config template version.
// Output only, except for the version description.
// Contains metadata about a particular version of the Remote Config template.
// All fields are set at the time the specified Remote Config template is published.
// A version's description field may be specified in PublishTemplate calls
type Version struct {
	Description    string    `json:"description"`
	IsLegacy       bool      `json:"isLegacy"`
	RollbackSource string    `json:"rollbackSource"`
	UpdateOrigin   string    `json:"updateOrigin"`
	UpdateTime     time.Time `json:"updateTime"`
	UpdateType     string    `json:"updateType"`
	UpdateUser     *User     `json:"updateUser"`
	VersionNumber  string    `json:"versionNumber"`
}

// VersionIterator represents the iterator for looping over versions
type VersionIterator struct{}

// PageInfo represents the information about a Page
func (it *VersionIterator) PageInfo() *iterator.PageInfo {
	// TODO
	return nil
}

// Next will return the next version item in the loop
func (it *VersionIterator) Next() (*Version, error) {
	return nil, nil
}

// ListVersionsResponse is a list of Remote Config template versions
type ListVersionsResponse struct {
	NextPageToken string    `json:"nextPageToken"`
	Versions      []Version `json:"versions"`
}

// ListVersionsOptions to be used as query params in the request to list versions
type ListVersionsOptions struct {
	StartTime        time.Time
	EndTime          time.Time
	EndVersionNumber string
	PageSize         int
	PageToken        string
}

// Condition targets a specific group of users
// A list of these conditions make up part of a Remote Config template
type Condition struct {
	Expression string   `json:"expression"`
	Name       string   `json:"name"`
	TagColor   TagColor `json:"tagColor"`
}

// UnmarshalJSON unmarshals a JSON string into a Condition (for internal use only).
func (c *Condition) UnmarshalJSON(b []byte) error {
	type conditionInternal Condition
	temp := struct {
		TagColor string `json:"tagColor"`
		*conditionInternal
	}{
		conditionInternal: (*conditionInternal)(c),
	}
	if err := json.Unmarshal(b, &temp); err != nil {
		return err
	}

	if temp.TagColor != "" {
		colours := map[string]TagColor{
			"BLUE":        Blue,
			"BROWN":       Brown,
			"CYAN":        Cyan,
			"DEEP_ORANGE": DeepOrange,
			"GREEN":       Green,
			"INDIGO":      Indigo,
			"LIME":        Lime,
			"ORANGE":      Orange,
			"PINK":        Pink,
			"PURPLE":      Purple,
			"TEAL":        Teal,
		}
		if tag, ok := colours[temp.TagColor]; ok {
			c.TagColor = tag
		} else {
			return fmt.Errorf("unknown tag colour value: %q", temp.TagColor)
		}
	}
	return nil
}

// RemoteConfig represents a Remote Config
type RemoteConfig struct {
	Conditions      []Condition               `json:"conditions"`
	Parameters      map[string]Parameter      `json:"parameters"`
	Version         Version                   `json:"version"`
	ParameterGroups map[string]ParameterGroup `json:"parameterGroups"`
}

// Response to save the API response including ETag
type Response struct {
	RemoteConfig
	Etag string `json:"etag"`
}

// Parameter .
type Parameter struct {
	ConditionalValues map[string]*ParameterValue `json:"conditionalValues"`
	DefaultValue      *ParameterValue            `json:"defaultValue"`
	Description       string                     `json:"description"`
}

// ParameterValue .
type ParameterValue struct {
	Value           string
	UseInAppDefault bool
}

// UnmarshalJSON unmarshals a JSON string into an ParameterValue (for internal use only).
func (pv *ParameterValue) UnmarshalJSON(b []byte) error {
	temp := struct {
		Value           string `json:"value"`
		UseInAppDefault bool   `json:"useInAppDefault"`
	}{}
	if err := json.Unmarshal(b, &temp); err != nil {
		return err
	}

	if temp.UseInAppDefault {
		pv.UseInAppDefault = true
		return nil
	}

	pv.Value = temp.Value
	return nil
}

// UseInAppDefaultValue returns a parameter value with the in app default as true
func UseInAppDefaultValue() *ParameterValue {
	return &ParameterValue{
		UseInAppDefault: true,
	}
}

// NewExplicitParameterValue will add a new explicit parameter value
func NewExplicitParameterValue(value string) *ParameterValue {
	return &ParameterValue{
		UseInAppDefault: false,
		Value:           value,
	}
}

// ParameterGroup representing a Remote Config parameter group
// Grouping parameters is only for management purposes and does not affect client-side fetching of parameter values
type ParameterGroup struct {
	Description string                `json:"description"`
	Parameters  map[string]*Parameter `json:"parameters"`
}

// Template .
type Template struct {
	Conditions      []*Condition
	ETag            string
	Parameters      map[string]*Parameter
	ParameterGroups map[string]*ParameterGroup
	Version         *Version
}

// User represents a remote config user
type User struct {
	Email    string `json:"email"`
	ImageURL string `json:"imageUrl"`
	Name     string `json:"name"`
}
