package domain

// GroupModelsListConfig controls the optional custom public model-discovery list.
type GroupModelsListConfig struct {
	Enabled bool     `json:"enabled"`
	Models  []string `json:"models,omitempty"`
}
