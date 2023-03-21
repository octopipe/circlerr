package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Override struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type CircleModule struct {
	Name      string     `json:"name,omitempty" validate:"required"`
	Revision  string     `json:"revision,omitempty"`
	Overrides []Override `json:"overrides,omitempty"`
	Namespace string     `json:"namespace,omitempty" validate:"required"`
}

type CircleEnvironments struct {
	Key   string `json:"key,omitempty" validate:"key"`
	Value string `json:"value,omitempty" validate:"value"`
}

type CircleMatch struct {
	Headers map[string]string `json:"headers,omitempty"`
}

type CircleSegment struct {
	Key       string `json:"key,omitempty" validate:"key"`
	Value     string `json:"value,omitempty" validate:"value"`
	Condition string `json:"condition,omitempty" validate:"condition"`
}

type CanaryDeployStrategy struct {
	Weight int `json:"weight" validate:"weight"`
}

type CircleRouting struct {
	Strategy string                `json:"strategy,omitempty" validate:"oneof=DEFAULT MATCH CANARY,required"`
	Canary   *CanaryDeployStrategy `json:"canary,omitempty"`
	Match    *CircleMatch          `json:"match,omitempty"`
	Segments []*CircleSegment      `json:"segments,omitempty"`
}

type CircleSpec struct {
	Author      string `json:"author,omitempty" default:"anonymous"`
	Description string `json:"description,omitempty"`
	Namespace   string `json:"namespace,omitempty" validate:"required"`
	// Routing      CircleRouting        `json:"routing,omitempty"`
	Modules      []CircleModule       `json:"modules,omitempty"`
	Environments []CircleEnvironments `json:"environments,omitempty"`
}

type CircleStatusHistory struct {
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	EventTime string `json:"eventTime,omitempty"`
	Action    string `json:"action,omitempty"`
}

type CircleResourceModule struct {
	Name      string `json:"name,omitempty" validate:"required"`
	Revision  string `json:"revision,omitempty"`
	Namespace string `json:"namespace,omitempty" validate:"required"`
}

type CircleResourceStatus struct {
	SyncedAt   string `json:"syncTime,omitempty"`
	SyncStatus string `json:"syncStatus,omitempty"`
	Error      string `json:"error,omitempty"`
}

type CircleStatusResource struct {
	Group     string               `json:"group,omitempty"`
	Kind      string               `json:"kind,omitempty"`
	Name      string               `json:"name,omitempty"`
	Namespace string               `json:"namespace,omitempty"`
	Status    CircleResourceStatus `json:"status,omitempty"`
	Module    CircleResourceModule `json:"module,omitempty"`
}

type CircleStatus struct {
	History    []CircleStatusHistory  `json:"history,omitempty"`
	SyncStatus string                 `json:"syncStatus,omitempty"`
	SyncedAt   string                 `json:"syncTime,omitempty"`
	Resources  []CircleStatusResource `json:"resources,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Circle is the Schema for the circles API
type Circle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CircleSpec   `json:"spec,omitempty"`
	Status CircleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CircleList contains a list of Circle
type CircleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Circle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Circle{}, &CircleList{})
}
