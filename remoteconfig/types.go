package remoteconfig

// Version a Remote Config template version.
// Output only, except for the version description.
// Contains metadata about a particular version of the Remote Config template.
// All fields are set at the time the specified Remote Config template is published.
// A version's description field may be specified in PublishTemplate calls
type Version struct {
	Description    string `json:"description"`
	IsLegacy       bool   `json:"isLegacy"`
	RollbackSource string `json:"rollbackSource"`
	UpdateOrigin   string `json:"updateOrigin"`
	UpdateTime     string `json:"updateTime"`
	UpdateType     string `json:"updateType"`
	UpdateUser     User   `json:"updateUser"`
	VersionNumber  string `json:"versionNumber"`
}

// ListVersionsResponse is a list of Remote Config template versions
type ListVersionsResponse struct {
	NextPageToken string    `json:"nextPageToken"`
	Versions      []Version `json:"versions"`
}

// ListVersionsOptions to be used as query params in the request to list versions
type ListVersionsOptions struct {
	StartTime        string
	EndTime          string
	EndVersionNumber string
	PageSize         int64
	PageToken        string
}

// Condition targets a specific group of users
// A list of these conditions make up part of a Remote Config template
type Condition struct {
	Expression string `json:"expression"`
	Name       string `json:"name"`
	TagColor   string `json:"tagColor"`
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
	ConditionalValues map[string]ParameterValue `json:"conditionalValues"`
	DefaultValue      ParameterValue            `json:"defaultValue"`
	Description       string                    `json:"description"`
}

// ParameterValue .
type ParameterValue struct {
	Value           string `json:"value"`
	UseInAppDefault bool   `json:"useInAppDefault"`
}

// ParameterGroup representing a Remote Config parameter group
// Grouping parameters is only for management purposes and does not affect client-side fetching of parameter values
type ParameterGroup struct {
	Description string               `json:"description"`
	Parameters  map[string]Parameter `json:"parameters"`
}

// Template .
type Template struct {
	Conditions      []Condition
	ETag            string
	ParameterGroups map[string]ParameterGroup
	Parameters      map[string]Parameter
	Version         Version
}

// User represents a remote config user
type User struct {
	Email    string `json:"email"`
	ImageURL string `json:"imageUrl"`
	Name     string `json:"name"`
}
