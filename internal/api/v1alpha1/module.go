package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModuleSpec defines the desired state of Module

type ModuleAuth struct {
	AuthType      string `json:"type,omitempty"`
	SshPrivateKey string `json:"sshPrivateKey,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	AccessToken   string `json:"accessToken,omitempty"`
}

type SecretRef struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type ModuleSpec struct {
	Author       string      `json:"author,omitempty"`
	Description  string      `json:"description,omitempty"`
	SecretRef    *SecretRef  `json:"secretRef,omitempty"`
	Path         string      `json:"path,omitempty"`
	Url          string      `json:"url,omitempty"`
	TemplateType string      `json:"templateType,omitempty"`
	Auth         *ModuleAuth `json:"auth,omitempty"`
}

// ModuleStatus defines the observed state of Module
type ModuleStatus struct {
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Module is the Schema for the modules API
type Module struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModuleSpec   `json:"spec,omitempty"`
	Status ModuleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ModuleList contains a list of Module
type ModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Module `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Module{}, &ModuleList{})
}
