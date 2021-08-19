/*
Copyright 2021 The Kruise Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apiinternal

type PodConstraint struct {
	Spec PodConstraintSpec `json:"spec,omitempty"`
}

// PodConstraintSpec defines the desired state of PodConstraint
type PodConstraintSpec struct {
	// Foo is an example field of PodConstraint. Edit PodConstraint_types.go to remove/update
	SpreadRule SpreadRule `json:"spreadRule"`
}

type SpreadRule struct {
	Requires   []SpreadRuleItem `json:"requires,omitempty"`
	Affinities []SpreadRuleItem `json:"affinities,omitempty"`
}

type SpreadRuleItem struct {
	TopologyKey      string          `json:"topologyKey"`
	PodSpreadType    PodSpreadType   `json:"podSpreadType"`
	MaxCount         *int32          `json:"maxCount,omitempty"`
	MaxSkew          int32           `json:"maxSkew"`
	MinTopologyValue *int32          `json:"minTopologyValue,omitempty"`
	TopologyRatios   []TopologyRatio `json:"topologyRatios,omitempty"`
}

type PodSpreadType string

const (
	PodSpreadTypeDefault = "Default"
	PodSpreadTypeRatio   = "Ratio"
)

type TopologyRatio struct {
	TopologyValue string `json:"topologyValue,omitempty"`
	Ratio         *int32 `json:"ratio,omitempty"`
}
