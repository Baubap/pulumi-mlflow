package registry

import "github.com/Baubap/pulumi-mlflow/provider/client"

// registeredModelDTO mirrors MLflow's RegisteredModel entity.
type registeredModelDTO struct {
	Name                 string            `json:"name"`
	Description          string            `json:"description,omitempty"`
	CreationTimestamp    int64             `json:"creation_timestamp,omitempty"`
	LastUpdatedTimestamp int64             `json:"last_updated_timestamp,omitempty"`
	Tags                 []client.KeyValue `json:"tags,omitempty"`
	Aliases              []modelAliasDTO   `json:"aliases,omitempty"`
	LatestVersions       []modelVersionDTO `json:"latest_versions,omitempty"`
}

// modelAliasDTO mirrors MLflow's RegisteredModelAlias entity.
type modelAliasDTO struct {
	Alias   string `json:"alias"`
	Version string `json:"version"`
}

// modelVersionDTO mirrors MLflow's ModelVersion entity.
type modelVersionDTO struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	CreationTimestamp    int64             `json:"creation_timestamp,omitempty"`
	LastUpdatedTimestamp int64             `json:"last_updated_timestamp,omitempty"`
	CurrentStage         string            `json:"current_stage,omitempty"`
	Description          string            `json:"description,omitempty"`
	Source               string            `json:"source,omitempty"`
	RunID                string            `json:"run_id,omitempty"`
	RunLink              string            `json:"run_link,omitempty"`
	Status               string            `json:"status,omitempty"`
	Tags                 []client.KeyValue `json:"tags,omitempty"`
	Aliases              []string          `json:"aliases,omitempty"`
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func eqStrPtr(a, b *string) bool {
	return deref(a) == deref(b)
}
