package dataconnect

// ConnectorConfig is the configuration for the Data Connect service.
type ConnectorConfig struct {
    Location  string `json:"location"`
    ServiceID string `json:"serviceId"`
}

// GraphqlOptions represents the options for a GraphQL query.
type GraphqlOptions struct {
    Variables     map[string]interface{} `json:"variables,omitempty"`
    OperationName string                 `json:"operationName,omitempty"`
}

// ExecuteGraphqlResponse is the response from a GraphQL query.
type ExecuteGraphqlResponse struct {
    Data map[string]interface{} `json:"data"`
}
