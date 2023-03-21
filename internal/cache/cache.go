package cache

import "github.com/octopipe/circlerr/internal/resource"

type Cache interface {
	Set(key string, resource resource.Resource)
	Scan(filter func(res resource.Resource) bool) map[string]resource.Resource
	Has(key string) bool
	Get(key string) resource.Resource
	Delete(key string)
}
