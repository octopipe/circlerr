package domain

import "github.com/octopipe/circlerr/internal/api/v1alpha1"

type Circle struct {
	Name string `json:"name"`
	v1alpha1.CircleSpec
}
