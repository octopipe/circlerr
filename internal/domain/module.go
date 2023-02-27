package domain

import "github.com/octopipe/circlerr/internal/api/v1alpha1"

const (
	SimpleModuleTemplateType = "SIMPLE"
	HelmModuleTemplateType   = "HELM"
)

type Module struct {
	Name string `json:"name"`
	v1alpha1.ModuleSpec
}
