package cache

import "github.com/octopipe/circlerr/pkg/twice/resource"

type Cache interface {
	Set(key string, resource resource.Resource)
	List(filter func(res resource.Resource) bool) []string
	Has(key string) bool
	Get(key string) resource.Resource
	Delete(key string)
}
