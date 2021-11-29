package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type PoolConfig struct {
	PolicyType PoolPolicyType `json:"policyType,omitempty"`
	Pools      []PoolTerm     `json:"pools"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	PatchTemplate runtime.RawExtension `json:"patchTemplate,omitempty"`
}

type PoolPolicyType string

const (
	PoolPolicyPoolingOnly       = "PoolingOnly"
	PoolPolicyPoolingIfPossible = "PoolingIfPossible"
)

type PoolTerm struct {
	Name          string            `json:"name"`
	MatchSelector map[string]string `json:"matchSelector"`
	Percent       int               `json:"percent,omitempty"`
}
