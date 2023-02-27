package cache

import "github.com/octopipe/circlerr/internal/resource"

type Cache interface {
	Set(key string, resource resource.Resource)
	List() []string
	Has(key string) bool
	Get(key string) resource.Resource
	Delete(key string)
}
