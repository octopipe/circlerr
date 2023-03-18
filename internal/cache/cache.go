package cache

import "github.com/octopipe/circlerr/internal/resource"

type Cache interface {
	Set(key string, resource resource.Resource)
	SetManagedObject(key string, resource resource.ManagedResource)
	List() []string
	ListManagedObjects() []string
	Has(key string) bool
	Get(key string) resource.Resource
	GetManagedObject(key string) resource.ManagedResource
	Delete(key string)
}
